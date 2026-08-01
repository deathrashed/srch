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
	return &app.Environment{
		Config:   cfg,
		Paths:    paths,
		Catalog:  cat,
		Platform: platform.Services{},
		History:  state.HistoryStore{Path: filepath.Join(paths.DataDir, "history.json"), Limit: 20},
		Library:  state.Library{ProfilePath: filepath.Join(paths.DataDir, "profiles.json"), FavouritePath: filepath.Join(paths.DataDir, "favourites.json")},
	}
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

func TestKeyboardNavigationAndPalette(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 150, 48

	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	if got := model.currentCategory(); got != "images" {
		t.Fatalf("right arrow category = %q, want images", got)
	}

	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if !model.drawerOpen {
		t.Fatal("Ctrl+P did not open the command palette")
	}

	model.drawerOpen = false
	model.input.SetValue("editable query")
	category := model.currentCategory()
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	if model.currentCategory() != category {
		t.Fatal("left arrow changed category while editing a query")
	}
}

func TestMouseCategoryAndPaletteSelection(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 150, 48
	styles := newStyles(true)
	left := (model.width - model.contentWidth()) / 2
	tabY := lipglossHeight(model.renderHeader(styles)) + 1
	webWidth := len(" Web ")

	updated, _ := model.handleMouseClick(left+webWidth+2, tabY)
	model = updated.(Model)
	if got := model.currentCategory(); got != "images" {
		t.Fatalf("mouse category = %q, want images", got)
	}
	engineBefore := model.engineIndex
	updated, _ = model.handleMouseClick(left+model.contentWidth()-4, tabY+2)
	model = updated.(Model)
	if model.engineIndex == engineBefore {
		t.Fatal("clicking the engine row did not change engines")
	}

	model.drawerOpen = true
	settingsIndex := 6
	firstItemY := lipglossHeight(model.renderHeader(styles)) + 4
	updated, _ = model.handleMouseClick(left+4, firstItemY+settingsIndex)
	model = updated.(Model)
	if model.settings == nil {
		t.Fatal("clicking Settings did not open the settings form")
	}
	if view := model.render(); !strings.Contains(view, "SETTINGS") || !strings.Contains(view, "Search defaults (1/3)") {
		t.Fatal("settings view does not preserve the header and first settings page")
	}
}

func lipglossHeight(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}
