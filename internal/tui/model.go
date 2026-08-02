package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	searchapi "srch/internal/api"
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
var apiActionNames = []string{"Run", "Open", "Copy URL", "Export JSON", "Send to Reader", "Raw JSON"}

type overlay int

const (
	overlayNone overlay = iota
	overlaySettings
	overlayHelp
	overlayDetails
	overlayPicker
)

type hitRegion struct {
	x, y, w int
	action  string
	index   int
}

type operationMsg struct {
	kind            string
	text            string
	err             error
	preserveContent bool
}

type apiResultMsg struct {
	result searchapi.Result
	err    error
	id     uint64
}

type apiState struct {
	sourceIndex int
	typeIndex   int
	focusIndex  int
	actionIndex int
	selected    int
	phase       string
	showRaw     bool
	result      searchapi.Result
	requestID   uint64
}

type Model struct {
	env      *app.Environment
	input    textinput.Model
	spinner  spinner.Model
	width    int
	height   int
	dark     bool
	mode     Mode
	overlay  overlay
	status   string
	busy     bool
	content  string
	viewport viewport.Model

	categoryIndex int
	engineIndex   int
	targetIndex   int
	presetIndex   int
	focusIndex    int

	settings     settingsState
	historyIndex int
	downloadKind int
	api          apiState
	filterValues map[string]string
	fieldInputs  map[string]textinput.Model
	picker       pickerState
	returnFocus  int
}

type settingsState struct {
	draft config.Config
	index int
}

func New(environment *app.Environment) Model {
	input := textinput.New()
	input.Prompt = "❯ "
	input.Placeholder = "Search…"
	input.SetWidth(72)
	input.Focus()
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	view := viewport.New(viewport.WithWidth(72), viewport.WithHeight(12))
	view.SoftWrap = true
	view.MouseWheelEnabled = true
	m := Model{env: environment, input: input, spinner: spin, viewport: view, dark: true, filterValues: make(map[string]string), fieldInputs: make(map[string]textinput.Model)}
	for index, category := range searchCategories {
		if category == environment.Config.DefaultCategory {
			m.categoryIndex = index
			break
		}
	}
	m.engineIndex = m.preferredEngineIndex()
	m.resetFieldInputs()
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
		m.resizeViewport()
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(value)
		return m, cmd
	case operationMsg:
		m.busy = false
		if value.err != nil {
			m.status = value.kind + " failed: " + value.err.Error()
			if value.text != "" {
				m.content = value.text
				m.viewport.SetContent(value.text)
			}
		} else {
			m.status = value.kind + " complete"
			if !value.preserveContent || value.text != "" {
				m.content = value.text
				m.viewport.SetContent(value.text)
				m.viewport.GotoTop()
				if m.mode == ModeReader && value.text != "" {
					m.input.Blur()
				}
			}
		}
		return m, nil
	case apiResultMsg:
		if value.id != m.api.requestID {
			return m, nil
		}
		m.busy = false
		if value.err != nil {
			m.api.phase = "error"
			m.status = "API search failed: " + value.err.Error()
			m.content = ""
			m.viewport.SetContent("")
			return m, nil
		}
		m.api.phase = "success"
		m.api.result = value.result
		m.api.selected = 0
		m.status = fmt.Sprintf("%s returned %d results", value.result.Adapter.Name, len(value.result.Items))
		m.updateAPIContent()
		return m, nil
	case tea.MouseClickMsg:
		if m.env.Config.Mouse && value.Button == tea.MouseLeft {
			return m.handleMouse(value.X, value.Y)
		}
	case tea.MouseWheelMsg:
		if (m.mode == ModeReader || m.mode == ModeAPI) && m.content != "" && m.overlay == overlayNone {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(value)
			return m, cmd
		}
	}
	if m.overlay != overlayNone {
		return m.updateOverlay(msg)
	}

	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch {
		case matchesKey(key, keyQuit):
			return m, tea.Quit
		case matchesKey(key, keyPalette):
			m.openPalette()
			return m, nil
		case matchesKey(key, keySettings):
			m.openSettings()
			return m, nil
		case matchesKey(key, keyHelp):
			m.returnFocus = m.currentModeFocus()
			m.overlay = overlayHelp
			m.input.Blur()
			return m, nil
		case key.String() == "alt+left":
			m.changeMode(-1)
			return m, nil
		case key.String() == "alt+right":
			m.changeMode(1)
			return m, nil
		case strings.HasPrefix(key.String(), "ctrl+") && len(key.String()) == 6 && key.String()[5] >= '1' && key.String()[5] <= '5':
			m.setMode(int(key.String()[5] - '1'))
			return m, nil
		case !m.anyTextInputFocused() && len(key.String()) == 1 && key.String()[0] >= '1' && key.String()[0] <= '5':
			m.setMode(int(key.String()[0] - '1'))
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
	content, _ := m.render()
	view := tea.NewView(content)
	view.AltScreen = true
	if m.env.Config.Mouse {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	control := m.focusedSearchControl()
	if ok {
		switch {
		case matchesKey(key, keyNext):
			m.moveFocus(1)
			return m, nil
		case matchesKey(key, keyPrevious):
			m.moveFocus(-1)
			return m, nil
		case key.String() == "ctrl+e":
			m.changeEngine(1)
			return m, nil
		case key.String() == "ctrl+r":
			m.changePreset(1)
			return m, nil
		case matchesKey(key, keyFocusQuery):
			if m.input.Focused() {
				break
			}
			m.focusIndex = m.searchControlCount() - 1
			m.focusSearchControl()
			return m, textinput.Blink
		case key.String() == "left":
			if m.focusedTextInputHasValue() {
				break
			}
			m.changeFocused(-1)
			return m, nil
		case key.String() == "right":
			if m.focusedTextInputHasValue() {
				break
			}
			m.changeFocused(1)
			return m, nil
		case matchesKey(key, keyRun):
			controls := m.searchControls()
			if m.focusIndex >= 0 && m.focusIndex < len(controls)-1 {
				if strings.HasPrefix(controls[m.focusIndex], "field:") {
					m.moveFocus(1)
					return m, nil
				}
				m.openSearchPicker(controls[m.focusIndex])
				return m, textinput.Blink
			}
			return m.executeSearch(false)
		case matchesKey(key, keyCopy):
			return m.executeSearch(true)
		}
	}
	if strings.HasPrefix(control, "field:") {
		fieldID := strings.TrimPrefix(control, "field:")
		field := m.fieldInputs[fieldID]
		var cmd tea.Cmd
		field, cmd = field.Update(msg)
		m.fieldInputs[fieldID] = field
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateReader(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok && m.content != "" && key.String() == "esc" {
		m.content = ""
		m.viewport.SetContent("")
		m.input.Focus()
		return m, nil
	}
	if ok && m.content != "" && !m.input.Focused() {
		switch key.String() {
		case "/", "i":
			m.input.Focus()
			return m, textinput.Blink
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
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
				return m, tea.Batch(m.spinner.Tick, m.mediaDownloadCmd(rawURL))
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

func (m Model) mediaDownloadCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		output, err := m.env.Platform.Runner.Output("yt-dlp", "--newline", "--progress-template", "download:%(progress._percent_str)s %(progress._speed_str)s ETA %(progress._eta_str)s", "-P", m.env.Config.DownloadDir, rawURL)
		return operationMsg{kind: "Media download", text: tailOutput(string(output), 12), err: err}
	}
}

func (m Model) updateAPI(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch {
		case matchesKey(key, keyNext):
			m.moveAPIFocus(1)
			return m, nil
		case matchesKey(key, keyPrevious):
			m.moveAPIFocus(-1)
			return m, nil
		case matchesKey(key, keyFocusQuery):
			if !m.input.Focused() {
				m.api.focusIndex = 2
				m.input.Focus()
				return m, textinput.Blink
			}
		case key.String() == "left":
			if m.input.Focused() && m.input.Value() != "" {
				break
			}
			m.changeAPIFocused(-1)
			return m, nil
		case key.String() == "right":
			if m.input.Focused() && m.input.Value() != "" {
				break
			}
			m.changeAPIFocused(1)
			return m, nil
		case key.String() == "up" || key.String() == "k":
			if m.api.focusIndex == 4 && m.api.selected > 0 {
				m.api.selected--
				m.updateAPIContent()
				return m, nil
			}
		case key.String() == "down" || key.String() == "j":
			if m.api.focusIndex == 4 && m.api.selected+1 < len(m.api.result.Items) {
				m.api.selected++
				m.updateAPIContent()
				return m, nil
			}
		case matchesKey(key, keyRun):
			switch m.api.focusIndex {
			case 0:
				m.openAPIPicker(pickerAPISources)
				return m, textinput.Blink
			case 1:
				m.openAPIPicker(pickerAPITypes)
				return m, textinput.Blink
			case 2:
				return m.runAPISearch()
			case 3:
				return m.activateAPIAction()
			case 4:
				return m.openAPIResult()
			}
		}
	}
	previousQuery := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != previousQuery {
		m.resetAPIResult()
	}
	return m, cmd
}

func (m Model) runAPISearch() (tea.Model, tea.Cmd) {
	adapter := m.currentAPIAdapter()
	searchType := m.currentAPIType()
	query := strings.TrimSpace(m.input.Value())
	if adapter.ID == "" || searchType.ID == "" {
		m.api.phase = "error"
		m.status = "Choose an API source and search type"
		return m, nil
	}
	if query == "" {
		m.api.phase = "empty"
		m.status = "Enter a query to run the API search"
		return m, nil
	}
	m.api.requestID++
	requestID := m.api.requestID
	m.api.result = searchapi.Result{}
	m.api.showRaw = false
	m.content = ""
	m.viewport.SetContent("")
	m.busy = true
	m.api.phase = "loading"
	m.status = "Searching " + adapter.Name
	m.input.Blur()
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		result, err := searchapi.Search(ctx, adapter.ID, searchType.ID, query)
		return apiResultMsg{result: result, err: err, id: requestID}
	})
}

func (m Model) activateAPIAction() (tea.Model, tea.Cmd) {
	switch m.api.actionIndex {
	case 0:
		return m.runAPISearch()
	case 1:
		return m.openAPIResult()
	case 2:
		rawURL := m.currentAPIResultURL()
		if rawURL == "" {
			m.status = "Run a search before copying a result URL"
			return m, nil
		}
		if err := m.env.Platform.Copy(rawURL); err != nil {
			m.status = err.Error()
		} else {
			m.status = "Copied result URL"
		}
	case 3:
		if len(m.api.result.Raw) == 0 {
			m.status = "Run a search before exporting JSON"
			return m, nil
		}
		destination, err := m.exportAPIResult()
		if err != nil {
			m.status = "Export failed: " + err.Error()
		} else {
			m.status = "Exported JSON to " + destination
		}
	case 4:
		rawURL := m.currentAPIResultURL()
		if rawURL == "" {
			m.status = "Run a search before sending a result to Reader"
			return m, nil
		}
		m.setMode(int(ModeReader))
		m.input.SetValue(rawURL)
		m.status = "API result ready in Reader"
	case 5:
		if len(m.api.result.Raw) == 0 {
			m.status = "Run a search before viewing raw JSON"
			return m, nil
		}
		m.api.showRaw = !m.api.showRaw
		m.updateAPIContent()
	}
	return m, nil
}

func (m Model) openAPIResult() (tea.Model, tea.Cmd) {
	rawURL := m.currentAPIResultURL()
	if rawURL == "" {
		m.status = "Run a search and select a result to open"
		return m, nil
	}
	m.busy, m.status = true, "Opening result"
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		return operationMsg{kind: "API result", err: m.env.Platform.OpenURL(rawURL, m.env.Config.DefaultBrowser), preserveContent: true}
	})
}

func (m *Model) updateAPIContent() {
	if m.api.showRaw {
		m.content = string(m.api.result.Raw)
	} else if len(m.api.result.Items) == 0 {
		m.content = "No matching results. Try a broader query or another search type."
	} else {
		item := m.api.result.Items[min(m.api.selected, len(m.api.result.Items)-1)]
		m.content = item.Title
		if item.Subtitle != "" {
			m.content += "\n" + item.Subtitle
		}
		if item.Description != "" {
			m.content += "\n\n" + item.Description
		}
		if item.URL != "" {
			m.content += "\n\n" + item.URL
		}
	}
	m.viewport.SetContent(m.content)
	m.viewport.GotoTop()
}

func (m Model) currentAPIResultURL() string {
	if len(m.api.result.Items) > 0 {
		return m.api.result.Items[min(m.api.selected, len(m.api.result.Items)-1)].URL
	}
	return ""
}

func (m Model) exportAPIResult() (string, error) {
	if err := os.MkdirAll(m.env.Paths.DataDir, 0o755); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(m.env.Paths.DataDir, ".api-results-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return "", err
	}
	data := append(append([]byte(nil), m.api.result.Raw...), '\n')
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	destination := filepath.Join(m.env.Paths.DataDir, "api-results-"+time.Now().Format("20060102-150405.000000000")+".json")
	if err := os.Rename(temporaryName, destination); err != nil {
		return "", err
	}
	return destination, nil
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

func (m *Model) openPalette() {
	m.returnFocus = m.currentModeFocus()
	m.picker = newPicker(pickerCommands, "Command Palette", "Type a command…", commandPickerItems())
	m.overlay = overlayPicker
	m.input.Blur()
}

func (m *Model) openSourcePicker(kind pickerKind) {
	m.returnFocus = m.focusIndex
	if kind == pickerCategories {
		m.picker = newPicker(kind, "Choose a category", "Search categories…", categoryPickerItems())
	} else {
		m.picker = newPicker(kind, "Choose a search source", "Search engines…", enginePickerItems(m.currentEngines()))
	}
	m.overlay = overlayPicker
	m.input.Blur()
}

func (m *Model) openSearchPicker(control string) {
	switch control {
	case "category":
		m.openSourcePicker(pickerCategories)
	case "engine":
		m.openSourcePicker(pickerEngines)
	case "target":
		engine := m.currentEngine()
		if engine == nil {
			return
		}
		m.returnFocus = m.focusIndex
		m.picker = newPicker(pickerTargets, "Choose a search type", "Search targets…", targetPickerItems(engine.Targets))
		m.overlay = overlayPicker
		m.input.Blur()
	case "preset":
		m.returnFocus = m.focusIndex
		m.picker = newPicker(pickerPresets, "Choose a preset", "Search presets…", presetPickerItems(m.currentPresets()))
		m.overlay = overlayPicker
		m.input.Blur()
	default:
		if strings.HasPrefix(control, "filter:") {
			groupID := strings.TrimPrefix(control, "filter:")
			if target := m.currentTarget(); target != nil {
				for _, group := range target.OptionGroups {
					if group.ID == groupID {
						m.returnFocus = m.focusIndex
						m.picker = newPicker(pickerOptions, "Choose "+strings.ToLower(group.Name), "Search options…", optionPickerItems(group))
						m.overlay = overlayPicker
						m.input.Blur()
						return
					}
				}
			}
		}
	}
}

func (m *Model) openAPIPicker(kind pickerKind) {
	m.returnFocus = m.api.focusIndex
	if kind == pickerAPISources {
		m.picker = newPicker(kind, "Choose an API source", "Search documented APIs…", apiSourcePickerItems(searchapi.Adapters()))
	} else {
		m.picker = newPicker(kind, "Choose a search type", "Search source fields…", apiTypePickerItems(m.currentAPIAdapter().Types))
	}
	m.overlay = overlayPicker
	m.input.Blur()
}
func (m *Model) openSettings() {
	m.returnFocus = m.currentModeFocus()
	m.settings = settingsState{draft: cloneConfig(m.env.Config)}
	m.overlay = overlaySettings
	m.input.Blur()
}

func (m Model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		if m.overlay == overlayPicker {
			var cmd tea.Cmd
			m.picker.input, cmd = m.picker.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	switch m.overlay {
	case overlayPicker:
		items := m.picker.filtered()
		switch key.String() {
		case "esc", "ctrl+p":
			m.closeOverlay()
		case "up", "k":
			if m.picker.selected > 0 {
				m.picker.selected--
			}
		case "down", "j":
			if m.picker.selected+1 < len(items) {
				m.picker.selected++
			}
		case "enter":
			if len(items) == 0 {
				return m, nil
			}
			selected := items[min(m.picker.selected, len(items)-1)]
			switch m.picker.kind {
			case pickerCommands:
				return m.activatePalette(selected.index)
			case pickerCategories:
				m.categoryIndex = selected.index
				m.engineIndex = m.preferredEngineIndex()
				m.targetIndex, m.presetIndex = 0, 0
				m.filterValues = make(map[string]string)
				m.resetFieldInputs()
			case pickerEngines:
				m.engineIndex = selected.index
				m.targetIndex, m.presetIndex = 0, 0
				m.filterValues = make(map[string]string)
				m.resetFieldInputs()
			case pickerTargets:
				m.targetIndex = selected.index
				m.presetIndex = 0
				m.filterValues = make(map[string]string)
				m.resetFieldInputs()
			case pickerPresets:
				m.presetIndex = selected.index
				m.filterValues = make(map[string]string)
				m.resetFieldInputs()
			case pickerOptions:
				if selected.value == "" {
					delete(m.filterValues, selected.action)
				} else {
					m.filterValues[selected.action] = selected.value
				}
			case pickerAPISources:
				m.api.sourceIndex = selected.index
				m.api.typeIndex = 0
				m.resetAPIResult()
			case pickerAPITypes:
				m.api.typeIndex = selected.index
				m.resetAPIResult()
			}
			m.closeOverlay()
			return m, nil
		default:
			var cmd tea.Cmd
			m.picker.input, cmd = m.picker.input.Update(msg)
			m.picker.selected = 0
			return m, cmd
		}
	case overlaySettings:
		switch key.String() {
		case "esc":
			m.settings = settingsState{}
			m.closeOverlay()
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
			m.closeOverlay()
		}
	default:
		if key.String() == "esc" {
			m.closeOverlay()
		}
	}
	return m, nil
}

func (m *Model) closeOverlay() {
	m.overlay = overlayNone
	if m.mode == ModeAPI {
		m.api.focusIndex = m.returnFocus
		if m.api.focusIndex == 2 {
			m.input.Focus()
		}
		return
	}
	m.focusIndex = m.returnFocus
	if m.mode == ModeSearch {
		m.focusSearchControl()
	} else {
		m.input.Focus()
	}
}

func (m Model) activatePalette(index int) (tea.Model, tea.Cmd) {
	if index >= 0 && index < 5 {
		m.setMode(index)
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
	m.viewport.SetContent("")
	m.input.SetValue("")
	m.input.Focus()
	for id, field := range m.fieldInputs {
		field.Blur()
		m.fieldInputs[id] = field
	}
	switch m.mode {
	case ModeSearch:
		m.input.Placeholder = "Search…"
		m.focusIndex = m.searchControlCount() - 1
		m.focusSearchControl()
	case ModeReader:
		m.input.Placeholder = "Enter an article URL…"
	case ModeDownloader:
		m.input.Placeholder = "Enter a download URL…"
	case ModeAPI:
		m.input.Placeholder = "Search an API-backed source…"
		m.api.focusIndex = 2
	case ModeHistory:
		m.input.Placeholder = "Search…"
		m.input.Blur()
	default:
		m.input.Placeholder = "Search…"
	}
}

func (m Model) currentModeFocus() int {
	if m.mode == ModeAPI {
		return m.api.focusIndex
	}
	return m.focusIndex
}

func (m *Model) resizeViewport() {
	m.viewport.SetWidth(max(32, m.contentWidth()-4))
	headerHeight := lipgloss.Height(m.renderHeader(newStyles(m.dark)))
	m.viewport.SetHeight(max(6, m.height-headerHeight-15))
}

func tailOutput(value string, lines int) string {
	parts := strings.Split(strings.TrimSpace(value), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
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
	request := domain.SearchRequest{Query: m.input.Value(), CategoryID: m.currentCategory(), Action: domain.ActionOpen, Output: domain.OutputPlain, Modifiers: domain.Modifiers{Values: cloneStrings(m.filterValues), RawQuery: map[string]string{}}}
	for fieldID, input := range m.fieldInputs {
		if value := strings.TrimSpace(input.Value()); value != "" {
			request.Modifiers.Values[fieldID] = value
		}
	}
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

func (m Model) currentAPIAdapter() searchapi.Adapter {
	adapters := searchapi.Adapters()
	if len(adapters) == 0 {
		return searchapi.Adapter{}
	}
	return adapters[m.api.sourceIndex%len(adapters)]
}

func (m Model) currentAPIType() searchapi.SearchType {
	adapter := m.currentAPIAdapter()
	if len(adapter.Types) == 0 {
		return searchapi.SearchType{}
	}
	return adapter.Types[m.api.typeIndex%len(adapter.Types)]
}

func (m *Model) moveAPIFocus(delta int) {
	count := 4
	if len(m.api.result.Items) > 0 {
		count = 5
	}
	m.api.focusIndex = (m.api.focusIndex + delta + count) % count
	if m.api.focusIndex == 2 {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m *Model) changeAPIFocused(delta int) {
	switch m.api.focusIndex {
	case 0:
		adapters := searchapi.Adapters()
		if len(adapters) > 0 {
			m.api.sourceIndex = (m.api.sourceIndex + delta + len(adapters)) % len(adapters)
			m.api.typeIndex = 0
			m.resetAPIResult()
		}
	case 1:
		types := m.currentAPIAdapter().Types
		if len(types) > 0 {
			m.api.typeIndex = (m.api.typeIndex + delta + len(types)) % len(types)
			m.resetAPIResult()
		}
	case 3:
		m.api.actionIndex = (m.api.actionIndex + delta + len(apiActionNames)) % len(apiActionNames)
	}
}

func (m *Model) resetAPIResult() {
	m.api.requestID++
	m.api.phase = "idle"
	m.api.result = searchapi.Result{}
	m.api.selected = 0
	m.api.showRaw = false
	m.busy = false
	m.content, m.status = "", ""
	m.viewport.SetContent("")
}

func (m *Model) moveFocus(delta int) {
	count := m.searchControlCount()
	m.focusIndex = (m.focusIndex + delta + count) % count
	m.focusSearchControl()
}
func (m Model) searchControlCount() int {
	return len(m.searchControls())
}
func (m *Model) changeFocused(delta int) {
	controls := m.searchControls()
	if m.focusIndex < 0 || m.focusIndex >= len(controls) {
		return
	}
	switch control := controls[m.focusIndex]; control {
	case "category":
		m.changeCategory(delta)
	case "engine":
		m.changeEngine(delta)
	case "target":
		if engine := m.currentEngine(); engine != nil && len(engine.Targets) > 1 {
			m.targetIndex = (m.targetIndex + delta + len(engine.Targets)) % len(engine.Targets)
			m.filterValues = make(map[string]string)
			m.resetFieldInputs()
		}
	case "preset":
		m.changePreset(delta)
	default:
		if strings.HasPrefix(control, "filter:") {
			m.changeFilter(strings.TrimPrefix(control, "filter:"), delta)
		}
	}
}
func (m *Model) changeCategory(delta int) {
	m.categoryIndex = (m.categoryIndex + delta + len(searchCategories)) % len(searchCategories)
	m.engineIndex = m.preferredEngineIndex()
	m.targetIndex = 0
	m.presetIndex = 0
	m.filterValues = make(map[string]string)
	m.resetFieldInputs()
}
func (m *Model) changeEngine(delta int) {
	engines := m.currentEngines()
	if len(engines) > 0 {
		m.engineIndex = (m.engineIndex + delta + len(engines)) % len(engines)
		m.targetIndex = 0
		m.presetIndex = 0
		m.filterValues = make(map[string]string)
		m.resetFieldInputs()
	}
}
func (m *Model) changePreset(delta int) {
	presets := m.currentPresets()
	if len(presets) > 0 {
		m.presetIndex = (m.presetIndex + delta + len(presets) + 1) % (len(presets) + 1)
		m.resetFieldInputs()
	}
}

func (m Model) searchControls() []string {
	controls := []string{"category", "engine"}
	if engine := m.currentEngine(); engine != nil && len(engine.Targets) > 1 {
		controls = append(controls, "target")
	}
	if target := m.currentTarget(); target != nil {
		for _, group := range target.OptionGroups {
			controls = append(controls, "filter:"+group.ID)
		}
		for _, field := range target.Fields {
			controls = append(controls, "field:"+field.ID)
		}
	}
	if len(m.currentPresets()) > 0 {
		controls = append(controls, "preset")
	}
	return append(controls, "query")
}

func (m *Model) changeFilter(groupID string, delta int) {
	target := m.currentTarget()
	if target == nil {
		return
	}
	for _, group := range target.OptionGroups {
		if group.ID != groupID || len(group.Options) == 0 {
			continue
		}
		values := []string{""}
		for _, option := range group.Options {
			values = append(values, option.ID)
		}
		m.filterValues[groupID] = cycle(values, m.filterValues[groupID], delta)
		return
	}
}

func cloneStrings(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		if value != "" {
			result[key] = value
		}
	}
	return result
}

func (m *Model) resetFieldInputs() {
	m.fieldInputs = make(map[string]textinput.Model)
	if target := m.currentTarget(); target != nil {
		for _, definition := range target.Fields {
			field := textinput.New()
			field.Prompt = ""
			field.Placeholder = definition.Placeholder
			field.CharLimit = 120
			field.SetWidth(max(18, min(48, m.contentWidth()-18)))
			m.fieldInputs[definition.ID] = field
		}
	}
}

func (m Model) focusedSearchControl() string {
	controls := m.searchControls()
	if m.focusIndex < 0 || m.focusIndex >= len(controls) {
		return ""
	}
	return controls[m.focusIndex]
}

func (m *Model) focusSearchControl() {
	m.input.Blur()
	for id, field := range m.fieldInputs {
		field.Blur()
		m.fieldInputs[id] = field
	}
	control := m.focusedSearchControl()
	if control == "query" {
		m.input.Focus()
	} else if strings.HasPrefix(control, "field:") {
		id := strings.TrimPrefix(control, "field:")
		field := m.fieldInputs[id]
		field.Focus()
		m.fieldInputs[id] = field
	}
}

func (m Model) anyTextInputFocused() bool {
	if m.input.Focused() {
		return true
	}
	for _, field := range m.fieldInputs {
		if field.Focused() {
			return true
		}
	}
	return false
}

func (m Model) focusedTextInputHasValue() bool {
	if m.input.Focused() {
		return m.input.Value() != ""
	}
	for _, field := range m.fieldInputs {
		if field.Focused() {
			return field.Value() != ""
		}
	}
	return false
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
	_, hits := m.render()
	for _, hit := range hits {
		if y == hit.y && x >= hit.x && x < hit.x+hit.w {
			switch hit.action {
			case "mode":
				m.setMode(hit.index)
			case "category":
				m.focusIndex = 0
				m.openSourcePicker(pickerCategories)
			case "engine":
				m.focusIndex = 1
				m.openSourcePicker(pickerEngines)
			case "target", "preset":
				m.focusControl(hit.action)
				m.openSearchPicker(hit.action)
			case "query":
				m.focusIndex = m.searchControlCount() - 1
				m.input.Focus()
			case "reader-input":
				m.input.Focus()
			case "download-kind":
				m.downloadKind = 1 - m.downloadKind
			case "download-input":
				m.input.Focus()
			case "download-run":
				return m.updateDownloader(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
			case "history":
				m.historyIndex = hit.index
			case "api-source":
				m.api.focusIndex = 0
				m.openAPIPicker(pickerAPISources)
			case "api-type":
				m.api.focusIndex = 1
				m.openAPIPicker(pickerAPITypes)
			case "api-query":
				m.api.focusIndex = 2
				m.input.Focus()
			case "api-action":
				m.api.focusIndex = 3
				m.api.actionIndex = hit.index
				return m.activateAPIAction()
			case "api-result":
				if hit.index >= 0 && hit.index < len(m.api.result.Items) {
					m.api.focusIndex = 4
					m.api.selected = hit.index
					m.updateAPIContent()
				}
			case "picker":
				items := m.picker.filtered()
				if hit.index >= 0 && hit.index < len(items) {
					m.picker.selected = hit.index
					return m.updateOverlay(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
				}
			case "settings":
				m.settings.index = hit.index
				m.changeSetting(1)
			}
			if strings.HasPrefix(hit.action, "filter:") {
				m.focusControl(hit.action)
				m.openSearchPicker(hit.action)
			} else if strings.HasPrefix(hit.action, "field:") {
				m.focusControl(hit.action)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) focusControl(name string) {
	for index, control := range m.searchControls() {
		if control == name {
			m.focusIndex = index
			m.focusSearchControl()
			return
		}
	}
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
