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
	targets := request.Targets
	if len(targets) == 0 {
		for _, engineID := range request.EngineIDs {
			targets = append(targets, domain.SearchTarget{EngineID: engineID, PresetID: request.PresetID})
		}
	}
	if request.SearchSetID != "" {
		set, ok := e.Catalog.SearchSet(request.SearchSetID)
		if !ok {
			return nil, fmt.Errorf("unknown search set %q", request.SearchSetID)
		}
		targets = append([]domain.SearchTarget(nil), set.Targets...)
		if len(targets) == 0 {
			for _, engineID := range set.EngineIDs {
				targets = append(targets, domain.SearchTarget{EngineID: engineID})
			}
		}
	}
	if len(targets) == 0 && request.PresetID != "" {
		preset, ok := e.Catalog.Preset(request.PresetID)
		if !ok {
			return nil, fmt.Errorf("unknown preset %q", request.PresetID)
		}
		targets = []domain.SearchTarget{{EngineID: preset.EngineID, PresetID: preset.ID}}
	}
	result := make([]query.BuiltURL, 0, len(targets))
	for _, target := range targets {
		resolved, err := e.Catalog.ResolveTarget(target)
		if err != nil {
			return nil, err
		}
		engine := resolved.Engine
		if err := validateCapabilities(engine, request.Modifiers); err != nil {
			return nil, err
		}
		var selected *domain.EngineTarget
		if resolved.Target.ID != "" {
			selected = &resolved.Target
		}
		rawURL, err := query.BuildTarget(engine, selected, resolved.Preset, request)
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
