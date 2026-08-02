package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	searchapi "srch/internal/api"
	"srch/internal/app"
	"srch/internal/catalog"
	"srch/internal/config"
	"srch/internal/domain"
	"srch/internal/platform"
	"srch/internal/state"
)

type captureRunner struct {
	runCalled bool
	name      string
	args      []string
	output    []byte
	err       error
}

func (r *captureRunner) Run(string, ...string) error { r.runCalled = true; return nil }
func (r *captureRunner) Output(name string, args ...string) ([]byte, error) {
	r.name, r.args = name, append([]string(nil), args...)
	return r.output, r.err
}
func (r *captureRunner) LookPath(string) (string, error) { return "", errors.New("not found") }

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
	if model.overlay != overlayPicker || model.picker.kind != pickerCommands {
		t.Fatal("Ctrl+P did not open palette")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.overlay != overlayNone {
		t.Fatal("escape did not close palette")
	}
}

func TestAPIOverlayRestoresAPIFocusAndModeCommandsResetDestinationFocus(t *testing.T) {
	model := New(testEnvironment(t))
	model.setMode(int(ModeAPI))
	model.api.focusIndex = 1
	model.openPalette()
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if model.api.focusIndex != 1 {
		t.Fatalf("API focus restored to %d", model.api.focusIndex)
	}
	model.openPalette()
	for _, r := range "search" {
		model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.mode != ModeSearch || model.focusIndex != model.searchControlCount()-1 || !model.input.Focused() {
		t.Fatalf("Search mode focus not reset: mode=%v focus=%d focused=%v", model.mode, model.focusIndex, model.input.Focused())
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
	expectedY := lipgloss.Height(model.renderHeader(newStyles(true))) + 1
	if reader.y != expectedY {
		t.Fatalf("reader hit region y=%d, want rendered mode row y=%d", reader.y, expectedY)
	}
	updated, _ := model.handleMouse(reader.x, reader.y)
	if updated.(Model).mode != ModeReader {
		t.Fatal("mouse did not activate Reader")
	}
}

func TestMouseOpensSourcePickerWithoutStoredViewState(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 34
	_, hits := model.render()
	var category hitRegion
	for _, hit := range hits {
		if hit.action == "category" {
			category = hit
			break
		}
	}
	if category.w == 0 {
		t.Fatal("category hit region missing")
	}
	updated, _ := model.handleMouse(category.x, category.y)
	result := updated.(Model)
	if result.overlay != overlayPicker || result.picker.kind != pickerCategories {
		t.Fatal("category click did not open the category picker")
	}
}

func TestReaderViewportScrollsAndReturnsToInput(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 90, 24
	model.setMode(int(ModeReader))
	model.resizeViewport()
	model.content = strings.Repeat("line\n", 80)
	model.viewport.SetContent(model.content)
	model.input.Blur()
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	if model.viewport.YOffset() == 0 {
		t.Fatal("reader viewport did not scroll")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: '/'}))
	if !model.input.Focused() {
		t.Fatal("reader did not return focus to URL input")
	}
}

func TestFuzzyEnginePickerSelectsAndRestoresFocus(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 100, 30
	model.categoryIndex = 1
	model.engineIndex = 0
	model.focusIndex = 1
	model.openSourcePicker(pickerEngines)
	for _, r := range "wikimedia" {
		model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	items := model.picker.filtered()
	if len(items) == 0 || !strings.Contains(strings.ToLower(items[0].name), "wikimedia") {
		t.Fatalf("fuzzy picker did not find Wikimedia: %#v", items)
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.overlay != overlayNone || model.focusIndex != 1 || model.currentEngine().ID != "wikimedia-commons" {
		t.Fatalf("picker selection not applied/restored: overlay=%v focus=%d engine=%s", model.overlay, model.focusIndex, model.currentEngine().ID)
	}
}

func TestTargetAndRefinementRowsUseReusablePickers(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 40
	model.categoryIndex = categoryIndex("music")
	model.engineIndex = engineIndex(model.currentEngines(), "musicbrainz")
	model.focusIndex = 2
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.overlay != overlayPicker || model.picker.kind != pickerTargets {
		t.Fatal("target row did not open the reusable picker")
	}
	for _, r := range "recording" {
		model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if target := model.currentTarget(); target == nil || target.ID != "recording" {
		t.Fatalf("target picker selected %#v", target)
	}

	model.categoryIndex = categoryIndex("images")
	model.engineIndex = engineIndex(model.currentEngines(), "google-images")
	model.targetIndex, model.presetIndex = 0, 0
	model.filterValues = make(map[string]string)
	model.focusIndex = 2
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.picker.kind != pickerOptions {
		t.Fatal("refinement row did not open the reusable picker")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.filterValues["size"] != "large" {
		t.Fatalf("refinement picker selected %q", model.filterValues["size"])
	}
}

func TestCatalogFieldsAreEditableAndCompileIntoRequest(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 40
	model.categoryIndex = categoryIndex("web")
	model.engineIndex = engineIndex(model.currentEngines(), "google")
	model.resetFieldInputs()
	model.input.SetValue("bubble tea")
	model.focusControl("field:site")
	for _, r := range "example.com" {
		model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	request := model.currentRequest()
	if request.Modifiers.Values["site"] != "example.com" {
		t.Fatalf("site field not included in request: %#v", request.Modifiers.Values)
	}
	urls, err := model.env.URLs(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 || !strings.Contains(urls[0].URL, "site%3Aexample.com") {
		t.Fatalf("field did not compile into URL: %#v", urls)
	}
}

func TestSlashFocusesQueryAndCanBeTyped(t *testing.T) {
	model := New(testEnvironment(t))
	model.focusIndex = 0
	model.input.Blur()
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	if !model.input.Focused() {
		t.Fatal("slash did not focus query")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	if model.input.Value() != "/" {
		t.Fatalf("slash was not inserted after query focus: %q", model.input.Value())
	}
}

func TestAllCategoryLabelsAreDisplayCase(t *testing.T) {
	for _, id := range searchCategories {
		name := categoryName(id)
		if name == id && id != "ai" {
			t.Fatalf("category %q was not converted to display case", id)
		}
	}
}

func TestOverlayIsCompositedOverWorkspace(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 34
	model.openPalette()
	view, _ := model.render()
	for _, expected := range []string{"Command Palette", "SOURCE", "Search"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("composited overlay missing %q", expected)
		}
	}
}

func TestMediaDownloadCapturesYTDLPOutput(t *testing.T) {
	env := testEnvironment(t)
	runner := &captureRunner{output: []byte("download: 50% 2MiB/s ETA 00:10\ndownload:100% 4MiB/s ETA 00:00\n")}
	env.Platform.Runner = runner
	model := New(env)
	message := model.mediaDownloadCmd("https://example.com/watch?v=1")().(operationMsg)
	if runner.runCalled {
		t.Fatal("media download inherited terminal stdio through Run")
	}
	if runner.name != "yt-dlp" || !strings.Contains(strings.Join(runner.args, " "), "--progress-template") {
		t.Fatalf("unexpected yt-dlp invocation: %s %v", runner.name, runner.args)
	}
	if !strings.Contains(message.text, "100%") || message.err != nil {
		t.Fatalf("captured output missing: %#v", message)
	}
}

func TestAPIGuidedWorkspaceAndStructuredResults(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 44
	model.setMode(int(ModeAPI))
	view, hits := model.render()
	for _, expected := range []string{"Structured API Search", "1 SOURCE", "Search type", "2 QUERY", "3 ACTIONS", "No API key required", "Export JSON"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("API workspace missing %q", expected)
		}
	}
	var sourceHit hitRegion
	for _, hit := range hits {
		if hit.action == "api-source" {
			sourceHit = hit
			break
		}
	}
	if sourceHit.w == 0 {
		t.Fatal("API source has no mouse target")
	}
	updated, _ := model.handleMouse(sourceHit.x, sourceHit.y)
	model = updated.(Model)
	if model.overlay != overlayPicker || model.picker.kind != pickerAPISources {
		t.Fatal("API source click did not open picker")
	}
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model.api.focusIndex = 2
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.api.phase != "empty" || !strings.Contains(model.status, "Enter a query") {
		t.Fatalf("blank API query state = %q, %q", model.api.phase, model.status)
	}

	result := searchapi.Result{
		Adapter: searchapi.Adapters()[0],
		Type:    searchapi.Adapters()[0].Types[0],
		URL:     "https://example.test/request",
		Status:  "200 OK",
		Items:   []searchapi.Item{{Title: "Black Sabbath", Subtitle: "GB", URL: "https://example.test/result"}},
		Raw:     []byte(`{"artists":[{"name":"Black Sabbath"}]}`),
	}
	model = updateModel(t, model, apiResultMsg{result: result})
	view, _ = model.render()
	if !strings.Contains(view, "Black Sabbath") || model.api.phase != "success" {
		t.Fatalf("API result was not rendered: phase=%q", model.api.phase)
	}
	model.api.focusIndex = 3
	model.api.actionIndex = 5
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if !model.api.showRaw || !strings.Contains(model.content, "artists") {
		t.Fatal("Raw JSON action did not update result detail")
	}
	model.api.actionIndex = 3
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	exports, err := filepath.Glob(filepath.Join(model.env.Paths.DataDir, "api-results-*.json"))
	if err != nil || len(exports) != 1 {
		t.Fatalf("Export JSON path failed: %v, %v", err, exports)
	}
	exported, err := os.ReadFile(exports[0])
	if err != nil || !strings.Contains(string(exported), "artists") {
		t.Fatalf("Export JSON action failed: %v, %q", err, exported)
	}
	model.api.actionIndex = 4
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if model.mode != ModeReader || model.input.Value() != "https://example.test/result" {
		t.Fatalf("Send to Reader action failed: mode=%v input=%q", model.mode, model.input.Value())
	}
}

func TestNumberKeyJumpsModesOnlyOutsideTextInput(t *testing.T) {
	model := New(testEnvironment(t))
	model.input.Blur()
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: '4', Text: "4"}))
	if model.mode != ModeAPI {
		t.Fatalf("number key selected mode %v", model.mode)
	}
	model.input.Focus()
	model = updateModel(t, model, tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}))
	if model.mode != ModeAPI || !strings.Contains(model.input.Value(), "2") {
		t.Fatal("focused input did not retain numeric typing")
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

func TestSearchWorkspaceHasClearVisualHierarchy(t *testing.T) {
	model := New(testEnvironment(t))
	model.width, model.height = 110, 40
	view, _ := model.render()
	for _, label := range []string{"SOURCE", "QUERY", "of", "1 Search", "2 Reader"} {
		if !strings.Contains(view, label) {
			t.Fatalf("search workspace missing %q", label)
		}
	}
	if strings.Contains(view, "Category  web") {
		t.Fatal("category identifiers should be rendered as display labels")
	}
	if strings.Contains(view, "Baidu") {
		t.Fatal("inactive engine alternatives should not compete with the current selection")
	}
}

func categoryIndex(id string) int {
	for index, category := range searchCategories {
		if category == id {
			return index
		}
	}
	return 0
}

func engineIndex(engines []domain.Engine, id string) int {
	for index, engine := range engines {
		if engine.ID == id {
			return index
		}
	}
	return 0
}
