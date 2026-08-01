package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigratesLegacyMetalArchivesDefaults(t *testing.T) {
	root := t.TempDir()
	paths := Paths{ConfigDir: root, DataDir: filepath.Join(root, "data"), CacheDir: filepath.Join(root, "cache"), Downloads: filepath.Join(root, "downloads")}
	legacy := []byte("default_category = 'music'\n[categories.music]\ndefault_engine = 'metal-archives-album'\nalternates = ['metal-archives-band', 'spotify']\n")
	if err := os.WriteFile(filepath.Join(root, "config.toml"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 2 || cfg.Categories["music"].DefaultEngine != "metal-archives" {
		t.Fatalf("migration failed: %#v", cfg.Categories["music"])
	}
	if _, err := os.Stat(filepath.Join(root, "config.toml.v1.bak")); err != nil {
		t.Fatal("migration backup missing")
	}
	cfg2, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Version != 2 {
		t.Fatal("migration was not idempotent")
	}
}
