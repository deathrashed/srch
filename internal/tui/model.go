package tui

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"srch/internal/app"
	"srch/internal/config"
	"srch/internal/domain"
	"srch/internal/download"
	"srch/internal/reader"
)

type Mode int

const (
	ModeSearch Mode = iota
	ModeReader
	ModeDownloader
	ModeAPI
	ModeHistory
)

var modeNames = []string{"Search", "Reader", "Downloader", "API", "History"}
var searchCategories = []string{"web", "images", "ai", "music", "lyrics", "code", "research", "video", "social", "shopping", "extensions", "reference", "privacy", "health"}

type overlay int

const (
	overlayNone overlay = iota
	overlayPalette
	overlaySettings
	overlayHelp
	overlayDetails
)

type hitRegion struct {
	x, y, w int
	action  string
	index   int
}

type operationMsg struct {
	kind string
	text string
	err  error
}

type Model struct {
	env     *app.Environment
	input   textinput.Model
	spinner spinner.Model
	width   int
	height  int
	dark    bool
	mode    Mode
	overlay overlay
	status  string
	busy    bool
	content string
	hits    []hitRegion

	categoryIndex int
	engineIndex   int
	targetIndex   int
	presetIndex   int
	focusIndex    int

	paletteIndex   int
	settings       settingsState
	historyIndex   int
	downloadKind   int
	apiEngineIndex int
}

type settingsState struct {
	draft    config.Config
	index    int
	editing  bool
	previous Mode
}

func New(environment *app.Environment) Model {
	input := textinput.New()
	input.Prompt = "❯ "
	input.Placeholder = "Search…"
	input.SetWidth(72)
	input.Focus()
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	m := Model{env: environment, input: input, spinner: spin, dark: true}
	for index, category := range searchCategories {
		if category == environment.Config.DefaultCategory {
			m.categoryIndex = index
			break
		}
	}
	m.engineIndex = m.preferredEngineIndex()
	m.focusIndex = m.searchControlCount() - 1
	return m
}

func (m Model) Init() tea.Cmd { return tea.Batch(textinput.Blink, tea.RequestBackgroundColor) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case tea.BackgroundColorMsg:
		m.dark = value.IsDark()
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = value.Width, value.Height
		m.input.SetWidth(max(20, min(86, m.contentWidth()-4)))
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(value)
		return m, cmd
	case operationMsg:
		m.busy = false
		if value.err != nil {
			m.status = value.kind + " failed: " + value.err.Error()
		} else {
			m.status, m.content = value.kind+" complete", value.text
		}
		return m, nil
	case tea.MouseClickMsg:
		if m.env.Config.Mouse && value.Button == tea.MouseLeft {
			return m.handleMouse(value.X, value.Y)
		}
	}

	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		if m.overlay != overlayNone {
			return m.updateOverlay(key)
		}
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+p":
			m.openPalette()
			return m, nil
		case "ctrl+,":
			m.openSettings()
			return m, nil
		case "?":
			m.overlay = overlayHelp
			m.input.Blur()
			return m, nil
		case "alt+left":
			m.changeMode(-1)
			return m, nil
		case "alt+right":
			m.changeMode(1)
			return m, nil
		case "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5":
			m.setMode(int(key.String()[5] - '1'))
			return m, nil
		}
	}

	switch m.mode {
	case ModeSearch:
		return m.updateSearch(msg)
	case ModeReader:
		return m.updateReader(msg)
	case ModeDownloader:
		return m.updateDownloader(msg)
	case ModeAPI:
		return m.updateAPI(msg)
	case ModeHistory:
		return m.updateHistory(msg)
	}
	return m, nil
}

func (m Model) View() tea.View {
	content, hits := m.render()
	m.hits = hits
	view := tea.NewView(content)
	view.AltScreen = true
	if m.env.Config.Mouse {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch key.String() {
		case "tab":
			m.moveFocus(1)
			return m, nil
		case "shift+tab":
			m.moveFocus(-1)
			return m, nil
		case "ctrl+e":
			m.changeEngine(1)
			return m, nil
		case "ctrl+r":
			m.changePreset(1)
			return m, nil
		case "/":
			m.focusIndex = m.searchControlCount() - 1
			m.input.Focus()
			return m, nil
		case "left":
			if m.input.Focused() && m.input.Value() != "" {
				break
			}
			m.changeFocused(-1)
			return m, nil
		case "right":
			if m.input.Focused() && m.input.Value() != "" {
				break
			}
			m.changeFocused(1)
			return m, nil
		case "enter":
			return m.executeSearch(false)
		case "ctrl+y":
			return m.executeSearch(true)
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateReader(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok && key.String() == "enter" {
		rawURL := strings.TrimSpace(m.input.Value())
		if rawURL == "" {
			m.status = "Enter a URL to read"
			return m, nil
		}
		m.busy, m.status = true, "Fetching readable content"
		width := max(40, m.contentWidth()-6)
		return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			markdown, err := reader.Extract(ctx, rawURL)
			if err != nil {
				return operationMsg{kind: "Reader", err: err}
			}
			rendered, err := reader.Render(markdown, width, map[bool]string{true: "dark", false: "light"}[m.dark])
			return operationMsg{kind: "Reader", text: rendered, err: err}
		})
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateDownloader(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch key.String() {
		case "left", "right":
			m.downloadKind = 1 - m.downloadKind
			return m, nil
		case "enter":
			rawURL := strings.TrimSpace(m.input.Value())
			if rawURL == "" {
				m.status = "Enter a URL to download"
				return m, nil
			}
			if m.downloadKind == 1 {
				if m.env.Platform.Runner == nil || !m.env.Platform.Available("yt-dlp") {
					m.status = "Media downloads require yt-dlp"
					return m, nil
				}
				m.busy, m.status = true, "Running yt-dlp"
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					err := m.env.Platform.Runner.Run("yt-dlp", "-P", m.env.Config.DownloadDir, rawURL)
					return operationMsg{kind: "Media download", err: err}
				})
			}
			destination := filepath.Join(m.env.Config.DownloadDir, "download")
			m.busy, m.status = true, "Downloading"
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()
				result, err := download.Direct(ctx, rawURL, download.Options{Destination: destination, Resume: true})
				return operationMsg{kind: "Download", text: result.Path, err: err}
			})
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateAPI(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch key.String() {
		case "left", "right":
			engines := m.apiEngines()
			if len(engines) > 0 {
				m.apiEngineIndex = (m.apiEngineIndex + 1) % len(engines)
			}
			return m, nil
		case "enter":
			request := m.apiRequest()
			urls, err := m.env.URLs(request)
			if err != nil || len(urls) == 0 {
				if err != nil {
					m.status = err.Error()
				}
				return m, nil
			}
			m.content = "Adapter request\n" + urls[0].URL + "\n\nUse Ctrl+Y to copy or Enter in Search to open."
			m.status = "API request compiled"
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	entries, _ := m.env.History.List()
	switch key.String() {
	case "up", "k":
		if m.historyIndex > 0 {
			m.historyIndex--
		}
	case "down", "j":
		if m.historyIndex+1 < len(entries) {
			m.historyIndex++
		}
	case "enter":
		if len(entries) > 0 {
			m.input.SetValue(entries[m.historyIndex].Query)
			m.mode = ModeSearch
			m.input.Focus()
			m.status = "History query restored"
		}
	case "d", "delete":
		if len(entries) > 0 {
			if err := m.env.History.Remove(entries[m.historyIndex].ID); err != nil {
				m.status = err.Error()
			} else {
				m.status = "History entry deleted"
				if m.historyIndex > 0 {
					m.historyIndex--
				}
			}
		}
	}
	return m, nil
}

func (m *Model) openPalette() { m.overlay = overlayPalette; m.paletteIndex = 0; m.input.Blur() }
func (m *Model) openSettings() {
	m.settings = settingsState{draft: cloneConfig(m.env.Config), previous: m.mode}
	m.overlay = overlaySettings
	m.input.Blur()
}

func (m Model) updateOverlay(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayPalette:
		items := paletteItems()
		switch key.String() {
		case "esc", "ctrl+p":
			m.closeOverlay()
		case "up", "k":
			if m.paletteIndex > 0 {
				m.paletteIndex--
			}
		case "down", "j":
			if m.paletteIndex+1 < len(items) {
				m.paletteIndex++
			}
		case "enter":
			return m.activatePalette(m.paletteIndex)
		}
	case overlaySettings:
		switch key.String() {
		case "esc":
			m.overlay = overlayNone
			m.settings = settingsState{}
			m.input.Focus()
		case "up", "k":
			if m.settings.index > 0 {
				m.settings.index--
			}
		case "down", "j", "tab":
			if m.settings.index < 7 {
				m.settings.index++
			}
		case "left":
			m.changeSetting(-1)
		case "right", "enter":
			m.changeSetting(1)
		case "ctrl+s":
			previous := m.env.Config
			m.env.Config = cloneConfig(m.settings.draft)
			if err := m.env.SaveConfig(); err != nil {
				m.env.Config = previous
				m.status = "Settings were not saved: " + err.Error()
			} else {
				m.status = "Settings saved"
			}
			m.overlay = overlayNone
			m.input.Focus()
		}
	default:
		if key.String() == "esc" {
			m.closeOverlay()
		}
	}
	return m, nil
}

func (m *Model) closeOverlay() { m.overlay = overlayNone; m.input.Focus() }

func (m Model) activatePalette(index int) (tea.Model, tea.Cmd) {
	if index >= 0 && index < 5 {
		m.setMode(index)
		m.closeOverlay()
		return m, nil
	}
	switch index {
	case 5:
		m.openSettings()
	case 6:
		m.overlay = overlayHelp
	case 7:
		m.overlay = overlayDetails
	}
	return m, nil
}

func paletteItems() []string {
	return []string{"Search", "Reader", "Downloader", "API", "History", "Settings", "Manual & Help", "Doctor"}
}

func (m *Model) setMode(index int) {
	if index < 0 || index >= len(modeNames) {
		return
	}
	m.mode = Mode(index)
	m.overlay = overlayNone
	m.status, m.content = "", ""
	m.input.SetValue("")
	m.input.Focus()
	switch m.mode {
	case ModeReader:
		m.input.Placeholder = "Enter an article URL…"
	case ModeDownloader:
		m.input.Placeholder = "Enter a download URL…"
	case ModeAPI:
		m.input.Placeholder = "Search an API-backed source…"
	default:
		m.input.Placeholder = "Search…"
	}
}
func (m *Model) changeMode(delta int) {
	m.setMode((int(m.mode) + delta + len(modeNames)) % len(modeNames))
}

func (m Model) executeSearch(copyOnly bool) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.input.Value()) == "" {
		m.status = "Enter a search query"
		return m, nil
	}
	request := m.currentRequest()
	urls, err := m.env.URLs(request)
	if err != nil || len(urls) == 0 {
		if err != nil {
			m.status = err.Error()
		}
		return m, nil
	}
	if copyOnly {
		if err := m.env.Platform.Copy(urls[0].URL); err != nil {
			m.status = err.Error()
		} else {
			m.status = "Copied search URL"
		}
		return m, nil
	}
	if m.env.Config.HistoryEnabled {
		_ = m.env.History.Add(request)
	}
	m.busy, m.status = true, "Opening "+urls[0].Engine.Name
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		return operationMsg{kind: "Search", text: urls[0].URL, err: m.env.Platform.OpenURL(urls[0].URL, m.env.Config.DefaultBrowser)}
	})
}

func (m Model) currentCategory() string {
	return searchCategories[m.categoryIndex%len(searchCategories)]
}
func (m Model) currentEngines() []domain.Engine {
	return m.env.Catalog.Engines(m.currentCategory(), false)
}
func (m Model) currentEngine() *domain.Engine {
	engines := m.currentEngines()
	if len(engines) == 0 {
		return nil
	}
	value := engines[m.engineIndex%len(engines)]
	return &value
}
func (m Model) currentPresets() []domain.Preset {
	engine := m.currentEngine()
	if engine == nil {
		return nil
	}
	return m.env.Catalog.PresetsForEngine(engine.ID)
}
func (m Model) currentPreset() *domain.Preset {
	presets := m.currentPresets()
	if len(presets) == 0 {
		return nil
	}
	if m.presetIndex == 0 {
		pref := m.env.Config.Categories[m.currentCategory()].DefaultPreset
		if value, ok := m.env.Catalog.Preset(pref); ok && value.EngineID == m.currentEngine().ID {
			return &value
		}
		return nil
	}
	value := presets[(m.presetIndex-1)%len(presets)]
	return &value
}
func (m Model) currentTarget() *domain.EngineTarget {
	engine := m.currentEngine()
	if engine == nil || len(engine.Targets) == 0 {
		return nil
	}
	if preset := m.currentPreset(); preset != nil && preset.TargetID != "" {
		for index := range engine.Targets {
			if engine.Targets[index].ID == preset.TargetID {
				return &engine.Targets[index]
			}
		}
	}
	value := engine.Targets[m.targetIndex%len(engine.Targets)]
	return &value
}
func (m Model) currentRequest() domain.SearchRequest {
	request := domain.SearchRequest{Query: m.input.Value(), CategoryID: m.currentCategory(), Action: domain.ActionOpen, Output: domain.OutputPlain, Modifiers: domain.Modifiers{Values: map[string]string{}, RawQuery: map[string]string{}}}
	if engine := m.currentEngine(); engine != nil {
		target := domain.SearchTarget{EngineID: engine.ID}
		request.EngineIDs = []string{engine.ID}
		if selected := m.currentTarget(); selected != nil {
			target.TargetID = selected.ID
		}
		if preset := m.currentPreset(); preset != nil {
			target.PresetID = preset.ID
			request.PresetID = preset.ID
		}
		request.Targets = []domain.SearchTarget{target}
	}
	return request
}

func (m Model) apiEngines() []domain.Engine {
	all := m.env.Catalog.Engines("", false)
	result := make([]domain.Engine, 0)
	for _, engine := range all {
		if engine.Verification.Status == domain.StatusAPI {
			result = append(result, engine)
		}
	}
	return result
}

func (m Model) apiRequest() domain.SearchRequest {
	engines := m.apiEngines()
	request := domain.SearchRequest{Query: m.input.Value(), Action: domain.ActionPrint, Output: domain.OutputJSON, Modifiers: domain.Modifiers{Values: map[string]string{}, RawQuery: map[string]string{}}}
	if len(engines) > 0 {
		engine := engines[m.apiEngineIndex%len(engines)]
		request.CategoryID = engine.Category
		request.EngineIDs = []string{engine.ID}
		request.Targets = []domain.SearchTarget{{EngineID: engine.ID}}
	}
	return request
}

func (m *Model) moveFocus(delta int) {
	count := m.searchControlCount()
	m.focusIndex = (m.focusIndex + delta + count) % count
	if m.focusIndex == count-1 {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}
func (m Model) searchControlCount() int {
	count := 3
	if engine := m.currentEngine(); engine != nil && len(engine.Targets) > 1 {
		count++
	}
	return count
}
func (m *Model) changeFocused(delta int) {
	switch m.focusIndex {
	case 0:
		m.changeCategory(delta)
	case 1:
		m.changeEngine(delta)
	case 2:
		if engine := m.currentEngine(); engine != nil && len(engine.Targets) > 1 {
			m.targetIndex = (m.targetIndex + delta + len(engine.Targets)) % len(engine.Targets)
		} else {
			m.changePreset(delta)
		}
	case 3:
		m.changePreset(delta)
	}
}
func (m *Model) changeCategory(delta int) {
	m.categoryIndex = (m.categoryIndex + delta + len(searchCategories)) % len(searchCategories)
	m.engineIndex = m.preferredEngineIndex()
	m.targetIndex = 0
	m.presetIndex = 0
}
func (m *Model) changeEngine(delta int) {
	engines := m.currentEngines()
	if len(engines) > 0 {
		m.engineIndex = (m.engineIndex + delta + len(engines)) % len(engines)
		m.targetIndex = 0
		m.presetIndex = 0
	}
}
func (m *Model) changePreset(delta int) {
	presets := m.currentPresets()
	if len(presets) > 0 {
		m.presetIndex = (m.presetIndex + delta + len(presets) + 1) % (len(presets) + 1)
	}
}
func (m Model) preferredEngineIndex() int {
	engines := m.currentEngines()
	pref := m.env.Config.Categories[m.currentCategory()].DefaultEngine
	for index, engine := range engines {
		if engine.ID == pref {
			return index
		}
	}
	return 0
}

func (m *Model) changeSetting(delta int) {
	switch m.settings.index {
	case 0:
		m.settings.draft.DefaultCategory = cycle([]string{"web", "images", "ai", "music", "lyrics", "code", "research"}, m.settings.draft.DefaultCategory, delta)
	case 1:
		m.settings.draft.InputPosition = cycle([]string{"bottom", "top", "auto"}, m.settings.draft.InputPosition, delta)
	case 2:
		m.settings.draft.HeaderMode = cycle([]string{"auto", "full", "compact"}, m.settings.draft.HeaderMode, delta)
	case 3:
		m.settings.draft.Density = cycle([]string{"compact", "comfortable", "spacious"}, m.settings.draft.Density, delta)
	case 4:
		m.settings.draft.Theme = cycle([]string{"auto", "dark", "light", "monochrome"}, m.settings.draft.Theme, delta)
	case 5:
		m.settings.draft.Motion = cycle([]string{"normal", "reduced", "none"}, m.settings.draft.Motion, delta)
	case 6:
		m.settings.draft.HistoryEnabled = !m.settings.draft.HistoryEnabled
	case 7:
		m.settings.draft.Mouse = !m.settings.draft.Mouse
	}
}
func cycle(values []string, current string, delta int) string {
	index := 0
	for i, v := range values {
		if v == current {
			index = i
			break
		}
	}
	return values[(index+delta+len(values))%len(values)]
}
func cloneConfig(source config.Config) config.Config {
	result := source
	result.Categories = make(map[string]config.CategoryPreference, len(source.Categories))
	for key, value := range source.Categories {
		value.Alternates = append([]string(nil), value.Alternates...)
		result.Categories[key] = value
	}
	return result
}

func (m Model) handleMouse(x, y int) (tea.Model, tea.Cmd) {
	for _, hit := range m.hits {
		if y == hit.y && x >= hit.x && x < hit.x+hit.w {
			switch hit.action {
			case "mode":
				m.setMode(hit.index)
			case "category":
				m.categoryIndex = hit.index
				m.engineIndex = m.preferredEngineIndex()
				m.presetIndex = 0
			case "engine":
				m.engineIndex = hit.index
				m.presetIndex = 0
			case "palette":
				return m.activatePalette(hit.index)
			case "settings":
				m.settings.index = hit.index
				m.changeSetting(1)
			}
			return m, nil
		}
	}
	return m, nil
}

func Run(environment *app.Environment) error {
	_, err := tea.NewProgram(New(environment)).Run()
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
