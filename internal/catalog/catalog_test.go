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

func TestRefinedEngineTargetsAreResolvable(t *testing.T) {
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		engine string
		target string
	}{
		{engine: "bing-images", target: "image"},
		{engine: "wikimedia-commons", target: "audio"},
		{engine: "wikimedia-commons", target: "video"},
		{engine: "musicbrainz", target: "release"},
		{engine: "musicbrainz", target: "recording"},
	}
	for _, check := range checks {
		resolved, err := cat.ResolveTarget(domain.SearchTarget{EngineID: check.engine, TargetID: check.target})
		if err != nil {
			t.Fatalf("resolve %s/%s: %v", check.engine, check.target, err)
		}
		if resolved.Target.ID != check.target {
			t.Fatalf("resolve %s/%s returned target %q", check.engine, check.target, resolved.Target.ID)
		}
	}
}
