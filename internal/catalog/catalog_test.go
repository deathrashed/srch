package catalog_test

import (
	"path/filepath"
	"testing"

	"srch/internal/catalog"
	"srch/internal/domain"
)

func TestEmbeddedCatalogueIsValid(t *testing.T) {
	cat, err := catalog.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cat.Engines("", true)); got < 70 {
		t.Fatalf("expected a broad initial catalogue, got %d engines", got)
	}
	for _, category := range []string{"web", "images", "ai", "music", "lyrics", "code", "research", "video", "shopping"} {
		if got := len(cat.Engines(category, false)); got == 0 {
			t.Fatalf("category %q has no enabled engines", category)
		}
	}
}

func TestPresetsBelongToTheirEngine(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	for _, preset := range cat.PresetsForEngine("google-images") {
		if preset.EngineID != "google-images" {
			t.Fatalf("unexpected preset owner: %#v", preset)
		}
	}
	if got := len(cat.PresetsForEngine("bing-images")); got != 0 {
		t.Fatalf("bing should not inherit Google presets, got %d", got)
	}
}

func TestResolveTargetRejectsMismatchedPreset(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	_, err = cat.ResolveTarget(domain.SearchTarget{EngineID: "bing-images", PresetID: "google-images-transparent"})
	if err == nil {
		t.Fatal("expected an engine/preset mismatch")
	}
}

func TestLegacyMetalAlbumResolvesCanonicalTarget(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := cat.ResolveTarget(domain.SearchTarget{EngineID: "metal-archives-album"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Engine.ID != "metal-archives" || resolved.Target.ID != "album" {
		t.Fatalf("unexpected target: %#v", resolved)
	}
}
