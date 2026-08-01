package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

type CategoryPreference struct {
	DefaultEngine string   `toml:"default_engine"`
	DefaultPreset string   `toml:"default_preset,omitempty"`
	Alternates    []string `toml:"alternates,omitempty"`
}

type Config struct {
	Version         int                           `toml:"version"`
	DefaultCategory string                        `toml:"default_category"`
	DefaultBrowser  string                        `toml:"default_browser,omitempty"`
	InputPosition   string                        `toml:"input_position"`
	HeaderMode      string                        `toml:"header_mode"`
	Density         string                        `toml:"density"`
	Information     string                        `toml:"information"`
	Hints           string                        `toml:"hints"`
	OperationDetail string                        `toml:"operation_detail"`
	Theme           string                        `toml:"theme"`
	Motion          string                        `toml:"motion"`
	Mouse           bool                          `toml:"mouse"`
	HistoryEnabled  bool                          `toml:"history_enabled"`
	HistoryLimit    int                           `toml:"history_limit"`
	DownloadDir     string                        `toml:"download_dir,omitempty"`
	Categories      map[string]CategoryPreference `toml:"categories"`
}

type Paths struct {
	ConfigDir string
	DataDir   string
	CacheDir  string
	Downloads string
}

func Defaults() Config {
	return Config{
		Version:         2,
		DefaultCategory: "web",
		InputPosition:   "bottom",
		HeaderMode:      "auto",
		Density:         "comfortable",
		Information:     "standard",
		Hints:           "contextual",
		OperationDetail: "normal",
		Theme:           "auto",
		Motion:          "normal",
		Mouse:           true,
		HistoryEnabled:  true,
		HistoryLimit:    1000,
		Categories: map[string]CategoryPreference{
			"web":      {DefaultEngine: "google", Alternates: []string{"brave", "duckduckgo", "kagi"}},
			"images":   {DefaultEngine: "google-images", DefaultPreset: "google-images-large", Alternates: []string{"bing-images", "openverse", "wikimedia-commons"}},
			"ai":       {DefaultEngine: "perplexity", Alternates: []string{"chatgpt", "claude", "brave-ai"}},
			"music":    {DefaultEngine: "musicbrainz", Alternates: []string{"spotify", "bandcamp", "metal-archives"}},
			"lyrics":   {DefaultEngine: "genius", Alternates: []string{"google-lyrics", "azlyrics"}},
			"code":     {DefaultEngine: "github", Alternates: []string{"grep-app", "sourcegraph", "stackoverflow"}},
			"research": {DefaultEngine: "google-scholar", Alternates: []string{"crossref", "arxiv", "pubmed"}},
		},
	}
}

func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	configDir := os.Getenv("XDG_CONFIG_HOME")
	dataDir := os.Getenv("XDG_DATA_HOME")
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if runtime.GOOS == "windows" {
		if configDir == "" {
			configDir, err = os.UserConfigDir()
			if err != nil {
				return Paths{}, fmt.Errorf("resolve config directory: %w", err)
			}
		}
		if dataDir == "" {
			dataDir = configDir
		}
		if cacheDir == "" {
			cacheDir, err = os.UserCacheDir()
			if err != nil {
				return Paths{}, fmt.Errorf("resolve cache directory: %w", err)
			}
		}
	} else {
		if configDir == "" {
			configDir = filepath.Join(home, ".config")
		}
		if dataDir == "" {
			dataDir = filepath.Join(home, ".local", "share")
		}
		if cacheDir == "" {
			cacheDir = filepath.Join(home, ".cache")
		}
	}
	return Paths{
		ConfigDir: filepath.Join(configDir, "srch"),
		DataDir:   filepath.Join(dataDir, "srch"),
		CacheDir:  filepath.Join(cacheDir, "srch"),
		Downloads: filepath.Join(home, "Downloads", "srch"),
	}, nil
}

func Load(paths Paths) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(filepath.Join(paths.ConfigDir, "config.toml"))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	var stored struct {
		Version int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &stored); err != nil {
		return Defaults(), fmt.Errorf("parse config version: %w", err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Defaults(), fmt.Errorf("parse config: %w", err)
	}
	if stored.Version < 2 {
		migrateV2(&cfg)
		backup := filepath.Join(paths.ConfigDir, "config.toml.v1.bak")
		if _, statErr := os.Stat(backup); errors.Is(statErr, os.ErrNotExist) {
			if writeErr := os.WriteFile(backup, data, 0o600); writeErr != nil {
				return Defaults(), fmt.Errorf("back up version 1 config: %w", writeErr)
			}
		}
		if err := Save(paths, cfg); err != nil {
			return Defaults(), fmt.Errorf("save migrated config: %w", err)
		}
	}
	if cfg.HistoryLimit <= 0 {
		cfg.HistoryLimit = 1000
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = paths.Downloads
	}
	return cfg, nil
}

func migrateV2(cfg *Config) {
	cfg.Version = 2
	for category, preference := range cfg.Categories {
		if preference.DefaultEngine == "metal-archives-band" || preference.DefaultEngine == "metal-archives-album" {
			preference.DefaultEngine = "metal-archives"
		}
		for index, engineID := range preference.Alternates {
			if engineID == "metal-archives-band" || engineID == "metal-archives-album" {
				preference.Alternates[index] = "metal-archives"
			}
		}
		preference.Alternates = unique(preference.Alternates)
		cfg.Categories[category] = preference
	}
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func Save(paths Paths, cfg Config) error {
	if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(paths.ConfigDir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(paths.ConfigDir, "config.toml")); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
