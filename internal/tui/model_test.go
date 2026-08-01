package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"srch/internal/app"
	"srch/internal/catalog"
	"srch/internal/config"
	"srch/internal/platform"
	"srch/internal/state"
)

func testEnvironment(t *testing.T) *app.Environment {
	t.Helper()
	directory := t.TempDir()
	cat, err := catalog.Load("")
	if err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{ConfigDir: filepath.Join(directory, "config"), DataDir: filepath.Join(directory, "data"), CacheDir: filepath.Join(directory, "cache"), Downloads: filepath.Join(directory, "downloads")}
	cfg := config.Defaults()
	cfg.DownloadDir = paths.Downloads
	return &app.Environment{Config: cfg, Paths: paths, Catalog: cat, Platform: platform.Services{}, History: state.HistoryStore{Path: filepath.Join(paths.DataDir, "history.json"), Limit: 20}, Library: state.Library{ProfilePath: filepath.Join(paths.DataDir, "profiles.json"), FavouritePath: filepath.Join(paths.DataDir, "favourites.json")}}
}

func updateModel(t *testing.T, model Model, message tea.Msg) Model {
	t.Helper()
	updated, _ := model.Update(message)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model has type %T", updated)
	}
	return result
}

func TestSearchViewUsesProductNavigation(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 34
	view, _ := model.render()
	for _, label := range []string{"Search", "Reader", "Downloader", "API", "History"} {
		if !strings.Contains(view, label) {
			t.Fatalf("missing %s", label)
		}
	}
	for _, rejected := range []string{"More:Video", "Search Scope", "Request Preview", "★ Preferred"} {
		if strings.Contains(view, rejected) {
			t.Fatalf("unexpected %q", rejected)
		}
	}
}

func TestModeNavigationAndPalette(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 34
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight, Mod: tea.ModAlt}))
	if model.mode != ModeReader {
		t.Fatalf("mode=%v", model.mode)
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if model.overlay != overlayPalette {
		t.Fatal("Ctrl+P did not open palette")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.overlay != overlayNone {
		t.Fatal("escape did not close palette")
	}
}

func TestEngineChangeClearsForeignPreset(t *testing.T) {
	model := New(testEnvironment(t))
	model.categoryIndex = 1
	model.engineIndex = model.preferredEngineIndex()
	model.presetIndex = 1
	if model.currentPreset() == nil {
		t.Fatal("expected Google preset")
	}
	model.changeEngine(1)
	if preset := model.currentPreset(); preset != nil && preset.EngineID != "bing-images" {
		t.Fatalf("foreign preset survived: %#v", preset)
	}
	request := model.currentRequest()
	if request.EngineIDs[0] != model.currentEngine().ID || request.EngineIDs[0] == "google-images" {
		t.Fatalf("displayed engine differs from request: %#v", request)
	}
}

func TestSettingsAreTransactional(t *testing.T) {
	env := testEnvironment(t)
	model := New(env)
	original := env.Config.DefaultCategory
	model.openSettings()
	model.changeSetting(1)
	if env.Config.DefaultCategory != original {
		t.Fatal("draft mutated live config")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if env.Config.DefaultCategory != original {
		t.Fatal("discard changed live config")
	}
}

func TestMouseUsesRenderedHitRegions(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 34
	_, hits := model.render()
	model.hits = hits
	var reader hitRegion
	for _, hit := range hits {
		if hit.action == "mode" && hit.index == int(ModeReader) {
			reader = hit
			break
		}
	}
	if reader.w == 0 {
		t.Fatal("reader hit region missing")
	}
	updated, _ := model.handleMouse(reader.x, reader.y)
	if updated.(Model).mode != ModeReader {
		t.Fatal("mouse did not activate Reader")
	}
}

func TestHeaderCompactsInShortTerminal(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 12
	header := model.renderHeader(newStyles(true))
	if strings.Contains(header, "███████") {
		t.Fatal("short terminal retained large banner")
	}
	if !strings.Contains(header, "SRCH") {
		t.Fatal("compact header missing")
	}
}

func TestGoogleImageFiltersAreVisibleAndComposable(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 40
	model.categoryIndex = 1
	model.engineIndex = model.preferredEngineIndex()
	model.focusIndex = 2
	view, _ := model.render()
	for _, label := range []string{"Size", "Color", "Format", "Recency"} {
		if !strings.Contains(view, label) {
			t.Fatalf("missing filter row %s", label)
		}
	}
	model.filterValues["size"] = "large"
	model.filterValues["color"] = "transparent"
	model.filterValues["format"] = "svg"
	urls, err := model.env.URLs(model.currentRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 {
		t.Fatal("expected one URL")
	}
	for _, part := range []string{"isz%3Al", "ic%3Atrans", "ift%3Asvg"} {
		if !strings.Contains(urls[0].URL, part) {
			t.Fatalf("URL %q missing %q", urls[0].URL, part)
		}
	}
}
