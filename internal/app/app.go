package app

import (
	"fmt"
	"path/filepath"

	"srch/internal/catalog"
	"srch/internal/config"
	"srch/internal/domain"
	"srch/internal/platform"
	"srch/internal/query"
	"srch/internal/state"
)

type Environment struct {
	Config   config.Config
	Paths    config.Paths
	Catalog  *catalog.Catalog
	Platform platform.Services
	History  state.HistoryStore
	Library  state.Library
}

func New() (*Environment, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}
	cfg, configErr := config.Load(paths)
	cat, err := catalog.Load(filepath.Join(paths.ConfigDir, "engines.yaml"))
	if err != nil {
		return nil, err
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = paths.Downloads
	}
	environment := &Environment{
		Config:   cfg,
		Paths:    paths,
		Catalog:  cat,
		Platform: platform.New(),
		History: state.HistoryStore{
			Path:  filepath.Join(paths.DataDir, "history.json"),
			Limit: cfg.HistoryLimit,
		},
		Library: state.Library{
			ProfilePath:   filepath.Join(paths.DataDir, "profiles.json"),
			FavouritePath: filepath.Join(paths.DataDir, "favourites.json"),
		},
	}
	if configErr != nil {
		return environment, configErr
	}
	return environment, nil
}

func (e *Environment) Parse(args []string) (domain.SearchRequest, error) {
	return query.Parse(args, e.Catalog, e.Config)
}

func (e *Environment) URLs(request domain.SearchRequest) ([]query.BuiltURL, error) {
	engineIDs := request.EngineIDs
	if request.SearchSetID != "" {
		set, ok := e.Catalog.SearchSet(request.SearchSetID)
		if !ok {
			return nil, fmt.Errorf("unknown search set %q", request.SearchSetID)
		}
		engineIDs = set.EngineIDs
	}
	var preset *domain.Preset
	if request.PresetID != "" {
		value, ok := e.Catalog.Preset(request.PresetID)
		if !ok {
			return nil, fmt.Errorf("unknown preset %q", request.PresetID)
		}
		preset = &value
		if len(engineIDs) == 0 {
			engineIDs = []string{value.EngineID}
		}
	}
	result := make([]query.BuiltURL, 0, len(engineIDs))
	for _, id := range engineIDs {
		engine, ok := e.Catalog.Engine(id)
		if !ok {
			return nil, fmt.Errorf("engine %q is missing or unavailable", id)
		}
		enginePreset := preset
		if preset != nil && preset.EngineID != engine.ID {
			enginePreset = nil
		}
		if err := validateCapabilities(engine, request.Modifiers); err != nil {
			return nil, err
		}
		rawURL, err := query.Build(engine, enginePreset, request)
		if err != nil {
			return nil, err
		}
		result = append(result, query.BuiltURL{Engine: engine, URL: rawURL})
	}
	return result, nil
}

func validateCapabilities(engine domain.Engine, modifiers domain.Modifiers) error {
	available := make(map[string]struct{}, len(engine.Capabilities))
	for _, capability := range engine.Capabilities {
		available[capability] = struct{}{}
	}
	requested := make([]string, 0, len(modifiers.Values)+2)
	if modifiers.Exact {
		requested = append(requested, "exact")
	}
	if len(modifiers.Exclude) > 0 {
		requested = append(requested, "exclude")
	}
	for key, value := range modifiers.Values {
		if value != "" {
			requested = append(requested, key)
		}
	}
	for _, capability := range requested {
		if _, ok := available[capability]; !ok {
			return fmt.Errorf("engine %s does not support modifier %q", engine.Name, capability)
		}
	}
	return nil
}

func (e *Environment) SaveConfig() error {
	return config.Save(e.Paths, e.Config)
}
