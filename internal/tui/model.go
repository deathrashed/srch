package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	lipgloss "charm.land/lipgloss/v2"

	"srch/internal/app"
	"srch/internal/domain"
)

type openResultMsg struct {
	URL string
	Err error
}

type saveSettingsMsg struct{ Err error }

type Model struct {
	env            *app.Environment
	input          textinput.Model
	spinner        spinner.Model
	width          int
	height         int
	categories     []string
	categoryIndex  int
	engineIndex    int
	presetIndex    int
	moreIndex      int
	drawerOpen     bool
	drawerIndex    int
	utility        string
	settings       *huh.Form
	status         string
	lastURL        string
	busy           bool
	dark           bool
	showFullHeader bool
}

var primaryCategories = []string{"web", "images", "ai", "music", "lyrics", "code", "research", "more"}

var secondaryCategories = []string{"video", "social", "shopping", "extensions", "reference", "privacy", "health"}

var utilityItems = []string{"Search Home", "Reader", "API Results", "Downloads", "History", "Favourites & Profiles", "Settings", "Manual & Help", "Doctor"}

func New(environment *app.Environment) Model {
	input := textinput.New()
	input.Prompt = "❯ "
	input.Placeholder = "Search the web, images, AI, music, code, research…"
	input.SetWidth(72)
	input.Focus()
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	categoryIndex := 0
	for index, category := range primaryCategories {
		if category == environment.Config.DefaultCategory {
			categoryIndex = index
			break
		}
	}
	model := Model{
		env:            environment,
		input:          input,
		spinner:        spin,
		categories:     append([]string(nil), primaryCategories...),
		categoryIndex:  categoryIndex,
		dark:           true,
		showFullHeader: true,
	}
	model.engineIndex = model.preferredEngineIndex()
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.RequestBackgroundColor)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(max(20, min(90, msg.Width-8)))
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case openResultMsg:
		m.busy = false
		if msg.Err != nil {
			m.status = "Search failed: " + msg.Err.Error()
		} else {
			m.status = "Opened " + msg.URL
		}
		return m, nil
	case saveSettingsMsg:
		if msg.Err != nil {
			m.status = "Settings were not saved: " + msg.Err.Error()
		} else {
			m.status = "Settings saved"
		}
		m.utility = ""
		m.settings = nil
		m.input.Focus()
		return m, nil
	case tea.MouseClickMsg:
		if m.settings == nil && m.env.Config.Mouse && msg.Button == tea.MouseLeft {
			return m.handleMouseClick(msg.X, msg.Y)
		}
	}

	if m.settings != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
			m.settings = nil
			m.utility = ""
			m.input.Focus()
			return m, nil
		}
		updated, cmd := m.settings.Update(msg)
		m.settings = updated.(*huh.Form)
		if m.settings.State == huh.StateCompleted {
			return m, func() tea.Msg { return saveSettingsMsg{Err: m.env.SaveConfig()} }
		}
		if m.settings.State == huh.StateAborted {
			m.settings = nil
			m.utility = ""
			m.input.Focus()
		}
		return m, cmd
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if m.drawerOpen {
			return m.updateDrawer(key)
		}
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+p", "ctrl+k", "ctrl+g":
			m.drawerOpen = true
			m.input.Blur()
			return m, nil
		case "ctrl+s", "ctrl+,":
			m.utility = "Settings"
			m.settings = m.newSettingsForm()
			m.input.Blur()
			return m, m.settings.Init()
		case "ctrl+h":
			m.showFullHeader = !m.showFullHeader
			return m, nil
		case "ctrl+e":
			engines := m.currentEngines()
			if len(engines) > 0 {
				m.engineIndex = (m.engineIndex + 1) % len(engines)
			}
			return m, nil
		case "ctrl+r":
			presets := m.env.Catalog.Presets(m.currentCategory())
			if len(presets) > 0 {
				m.presetIndex = (m.presetIndex + 1) % (len(presets) + 1)
			}
			return m, nil
		case "ctrl+m":
			if m.categories[m.categoryIndex] == "more" {
				m.moreIndex = (m.moreIndex + 1) % len(secondaryCategories)
				m.engineIndex = m.preferredEngineIndex()
				m.presetIndex = 0
				m.status = "More category: " + title(m.currentCategory())
			}
			return m, nil
		case "left":
			if m.input.Value() != "" {
				break
			}
			m.changeCategory(-1)
			return m, nil
		case "right":
			if m.input.Value() != "" {
				break
			}
			m.changeCategory(1)
			return m, nil
		case "shift+tab", "alt+left":
			m.changeCategory(-1)
			return m, nil
		case "tab", "alt+right":
			m.changeCategory(1)
			return m, nil
		case "enter":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.status = "Enter a search query"
				return m, nil
			}
			request := m.currentRequest()
			urls, err := m.env.URLs(request)
			if err != nil {
				m.status = err.Error()
				return m, nil
			}
			if len(urls) == 0 {
				m.status = "No URL was generated"
				return m, nil
			}
			m.lastURL = urls[0].URL
			m.busy = true
			m.status = "Opening " + urls[0].Engine.Name
			_ = m.env.History.Add(request)
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				err := m.env.Platform.OpenURL(urls[0].URL, m.env.Config.DefaultBrowser)
				return openResultMsg{URL: urls[0].URL, Err: err}
			})
		case "ctrl+y":
			request := m.currentRequest()
			urls, err := m.env.URLs(request)
			if err != nil || len(urls) == 0 {
				m.status = "Nothing to copy"
				return m, nil
			}
			if err := m.env.Platform.Copy(urls[0].URL); err != nil {
				m.status = err.Error()
			} else {
				m.status = "Copied search URL"
			}
			return m, nil
		case "?":
			m.utility = "Manual & Help"
			m.input.Blur()
			return m, nil
		case "esc":
			if m.utility != "" {
				m.utility = ""
				m.input.Focus()
				return m, nil
			}
		}
		if strings.HasPrefix(key.String(), "alt+") && len(key.String()) == 5 {
			digit := int(key.String()[4] - '1')
			if digit >= 0 && digit < len(m.categories) {
				m.categoryIndex = digit
				m.engineIndex = 0
				m.presetIndex = 0
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	if m.env.Config.Mouse {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

func (m Model) render() string {
	dark := m.dark
	if m.env.Config.Theme == "dark" {
		dark = true
	} else if m.env.Config.Theme == "light" {
		dark = false
	}
	styles := newStyles(dark)
	header := m.renderHeader(styles)
	var content string
	if m.drawerOpen {
		content = m.renderDrawer(styles)
	} else if m.settings != nil {
		content = styles.title.Render("Settings") + "\n\n" + m.settings.View() + "\n\n" + m.renderFooter(styles, "esc", "cancel", "enter", "select", "tab", "next field")
	} else if m.utility != "" {
		content = m.renderUtility(styles)
	} else {
		content = m.renderSearchHome(styles)
	}
	body := header + "\n" + content
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styles.page.Width(m.contentWidth()).Render(body))
}

func (m Model) renderSearchHome(styles styles) string {
	tabs := m.renderTabs(styles)
	engineRow := m.renderEngineRow(styles)
	context := m.renderContext(styles)
	input := styles.input.Width(max(20, m.contentWidth()-2)).Render(m.input.View())
	parts := []string{tabs, engineRow}
	if m.env.Config.InputPosition == "top" {
		parts = append(parts, input, context)
	} else {
		parts = append(parts, context, input)
	}
	if status := m.renderStatus(styles); status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, m.renderFooter(styles, "tab/←→", "category", "ctrl+e", "engine", "ctrl+r", "preset", "ctrl+p", "commands", "ctrl+s", "settings"))
	return strings.Join(parts, "\n")
}

func (m Model) renderHeader(styles styles) string {
	full := m.showFullHeader && m.width >= 88 && m.height >= 25 && m.env.Config.HeaderMode != "compact"
	if !full {
		return styles.compactHeader.Width(m.contentWidth()).Render("SRCH  ·  " + m.screenSubtitle())
	}
	banner := strings.Join([]string{
		"███████╗██████╗  ██████╗██╗  ██╗",
		"██╔════╝██╔══██╗██╔════╝██║  ██║",
		"███████╗██████╔╝██║     ███████║",
		"╚════██║██╔══██╗██║     ██╔══██║",
		"███████║██║  ██║╚██████╗██║  ██║",
		"╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝",
	}, "\n")
	return styles.header.Width(m.contentWidth() - 2).Render(banner + "\n\n" + styles.tagline.Render(m.screenSubtitle()))
}

func (m Model) screenSubtitle() string {
	if m.drawerOpen {
		return "COMMAND PALETTE  •  CHOOSE AN ACTION"
	}
	if m.settings != nil {
		return "SETTINGS  •  PREFERENCES"
	}
	if m.utility != "" {
		return strings.ToUpper(m.utility) + "  •  UTILITIES"
	}
	engines := m.currentEngines()
	engine := "NO ENGINE"
	if len(engines) > 0 {
		engine = strings.ToUpper(engines[m.engineIndex%len(engines)].Name)
	}
	return "SEARCH  •  " + strings.ToUpper(m.currentCategory()) + "  •  " + engine
}

func (m Model) renderTabs(styles styles) string {
	items := make([]string, 0, len(m.categories))
	for index, category := range m.categories {
		label := title(category)
		if category == "more" {
			label = "More:" + title(secondaryCategories[m.moreIndex%len(secondaryCategories)])
		}
		if index == m.categoryIndex {
			items = append(items, styles.activeTab.Render(label))
		} else {
			items = append(items, styles.tab.Render(label))
		}
	}
	return strings.Join(items, " ")
}

func (m Model) renderEngineRow(styles styles) string {
	engines := m.currentEngines()
	if len(engines) == 0 {
		return styles.panel.Render("No engines in this category")
	}
	if m.engineIndex >= len(engines) {
		m.engineIndex = 0
	}
	active := engines[m.engineIndex]
	preset := m.currentPreset()
	line := "‹  " + styles.selected.Render(active.Name) + "  ›"
	if preset != nil {
		line += "    Preset: " + styles.accent.Render(preset.Name)
	}
	line += "    " + styles.success.Render("★ Preferred")
	alternates := make([]string, 0, min(4, len(engines)-1))
	for index, engine := range engines {
		if index != m.engineIndex && len(alternates) < 4 {
			alternates = append(alternates, engine.Name)
		}
	}
	if len(alternates) > 0 {
		line += "\n" + styles.muted.Render(strings.Join(alternates, " · "))
	}
	return styles.panel.Width(max(20, m.contentWidth()-2)).Render(line)
}

func (m Model) renderContext(styles styles) string {
	request := m.currentRequest()
	urls, err := m.env.URLs(request)
	preview := "Type a query to preview the generated search URL."
	if err == nil && request.Query != "" && len(urls) > 0 {
		preview = styles.selected.Render(urls[0].Engine.Name) + "\n" + wrap(urls[0].URL, max(24, min(58, m.width/2)))
	}
	engines := m.currentEngines()
	engineName := "None"
	if len(engines) > 0 {
		engineName = engines[m.engineIndex%len(engines)].Name
	}
	scope := "Category  " + styles.selected.Render(title(m.currentCategory())) + "\nEngine    " + styles.selected.Render(engineName)
	if preset := m.currentPreset(); preset != nil {
		scope += "\nPreset    " + styles.accent.Render(preset.Name)
	}
	left := styles.panel.Width(max(24, min(42, m.contentWidth()/2-2))).Render(styles.section.Render("Search Scope") + "\n" + scope)
	right := styles.panel.Width(max(24, min(52, m.contentWidth()/2-2))).Render(styles.section.Render("Request Preview") + "\n" + preview)
	if m.width < 78 {
		return left + "\n" + right
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m Model) renderStatus(styles styles) string {
	status := m.status
	if m.busy {
		status = m.spinner.View() + " " + status
	}
	if status == "" {
		return ""
	}
	return styles.status.Render(status)
}

func (m Model) renderUtility(styles styles) string {
	titleText := styles.title.Render(m.utility)
	var body string
	switch m.utility {
	case "Manual & Help":
		body = manualText
	case "History":
		entries, err := m.env.History.List()
		if err != nil {
			body = err.Error()
		} else if len(entries) == 0 {
			body = "No searches recorded yet."
		} else {
			lines := make([]string, 0, min(20, len(entries)))
			for index, entry := range entries {
				if index == 20 {
					break
				}
				lines = append(lines, entry.CreatedAt.Local().Format("2006-01-02 15:04")+"  "+entry.Query)
			}
			body = strings.Join(lines, "\n")
		}
	case "Doctor":
		tools := []string{"defuddle", "glow", "bat", "jq", "curl", "aria2c", "yt-dlp", "ffmpeg"}
		lines := make([]string, 0, len(tools))
		for _, tool := range tools {
			state := styles.warning.Render("missing")
			if m.env.Platform.Available(tool) {
				state = styles.success.Render("available")
			}
			lines = append(lines, fmt.Sprintf("%-10s %s", tool, state))
		}
		body = strings.Join(lines, "\n")
	case "Reader":
		body = "Extract clutter-free Markdown and render it in the terminal.\n\n" +
			"  srch read <url>\n" +
			"  srch read <url> --viewer glow\n" +
			"  srch read <url> --raw\n\n" +
			"Requires defuddle for extraction; built-in Glamour rendering works without glow."
	case "API Results":
		body = "Structured adapters: MusicBrainz, Internet Archive, Open Library, Crossref, arXiv, and TheAudioDB.\n\n" +
			"  srch api musicbrainz \"Miles Davis\"\n" +
			"  srch api crossref \"terminal interfaces\""
	case "Downloads":
		body = "Direct downloads are atomic, resumable, checksum-aware, and never overwrite by default.\n\n" +
			"  srch download <url> --resume\n" +
			"  srch download <url> --sha256 <digest>\n" +
			"  srch media <url> --audio\n\n" +
			"Media downloads explicitly delegate to yt-dlp."
	case "Favourites & Profiles":
		favourites, favouriteErr := m.env.Library.Favourites()
		profiles, profileErr := m.env.Library.Profiles()
		if favouriteErr != nil {
			body = favouriteErr.Error()
		} else if profileErr != nil {
			body = profileErr.Error()
		} else {
			body = fmt.Sprintf("Favourite engines: %d\nSaved profiles: %d\n\n", len(favourites), len(profiles)) +
				"  srch favourites add <engine>\n" +
				"  srch profiles save <name> @engine\n" +
				"  srch profiles run <name> <query>"
		}
	default:
		body = "Return to Search Home to begin."
	}
	return titleText + "\n\n" + styles.panel.Width(max(30, m.contentWidth()-2)).Render(body) + "\n\n" + m.renderFooter(styles, "esc", "search", "ctrl+p", "commands", "ctrl+s", "settings")
}

func (m Model) renderDrawer(styles styles) string {
	lines := make([]string, 0, len(utilityItems))
	for index, item := range utilityItems {
		if index == m.drawerIndex {
			lines = append(lines, styles.activeTab.Render("› "+item))
		} else {
			lines = append(lines, styles.tab.Render("  "+item))
		}
	}
	return styles.title.Render("Command Palette") + "\n\n" +
		styles.panel.Width(max(32, min(64, m.contentWidth()-2))).Render(strings.Join(lines, "\n")) + "\n\n" +
		m.renderFooter(styles, "↑/↓", "choose", "enter", "open", "esc", "close")
}

func (m Model) renderFooter(styles styles, items ...string) string {
	parts := make([]string, 0, len(items)/2)
	for index := 0; index+1 < len(items); index += 2 {
		parts = append(parts, styles.key.Render(items[index])+" "+styles.muted.Render(items[index+1]))
	}
	if m.contentWidth() < 90 && len(parts) > 4 {
		parts = []string{parts[0], parts[1], parts[len(parts)-2], parts[len(parts)-1]}
	}
	return styles.footer.Width(m.contentWidth()).Render(strings.Join(parts, "   "))
}

func (m Model) updateDrawer(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+p", "ctrl+g", "ctrl+k":
		m.drawerOpen = false
		m.input.Focus()
	case "up", "k":
		if m.drawerIndex > 0 {
			m.drawerIndex--
		}
	case "down", "j":
		if m.drawerIndex < len(utilityItems)-1 {
			m.drawerIndex++
		}
	case "enter":
		return m.activateDrawerChoice()
	}
	return m, nil
}

func (m Model) contentWidth() int {
	return max(40, min(100, m.width-4))
}

func (m Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	styles := newStyles(m.dark)
	contentWidth := m.contentWidth()
	contentLeft := max(0, (m.width-contentWidth)/2)
	headerHeight := lipgloss.Height(m.renderHeader(styles))

	if m.drawerOpen {
		firstItemY := headerHeight + 4
		index := y - firstItemY
		if index >= 0 && index < len(utilityItems) {
			m.drawerIndex = index
			return m.activateDrawerChoice()
		}
		return m, nil
	}

	if m.utility != "" {
		return m, nil
	}

	tabY := headerHeight + 1
	if y == tabY {
		cursor := contentLeft
		for index, category := range m.categories {
			label := title(category)
			if category == "more" {
				label = "More:" + title(secondaryCategories[m.moreIndex%len(secondaryCategories)])
			}
			width := lipgloss.Width(styles.tab.Render(label))
			if x >= cursor && x < cursor+width {
				m.categoryIndex = index
				m.engineIndex = m.preferredEngineIndex()
				m.presetIndex = 0
				m.status = "Category: " + title(m.currentCategory())
				return m, nil
			}
			cursor += width + 1
		}
	}
	if y >= tabY+1 && y <= tabY+4 {
		engines := m.currentEngines()
		if len(engines) > 0 {
			delta := 1
			if x < contentLeft+contentWidth/2 {
				delta = -1
			}
			m.engineIndex = (m.engineIndex + delta + len(engines)) % len(engines)
			m.presetIndex = 0
			m.status = "Engine: " + engines[m.engineIndex].Name
		}
	}

	return m, nil
}

func (m Model) activateDrawerChoice() (tea.Model, tea.Cmd) {
	choice := utilityItems[m.drawerIndex]
	m.drawerOpen = false
	if choice == "Search Home" {
		m.utility = ""
		m.input.Focus()
		return m, nil
	}
	if choice == "Settings" {
		m.utility = choice
		m.settings = m.newSettingsForm()
		return m, m.settings.Init()
	}
	m.utility = choice
	return m, nil
}

func (m Model) currentCategory() string {
	category := m.categories[m.categoryIndex]
	if category == "more" {
		return secondaryCategories[m.moreIndex%len(secondaryCategories)]
	}
	return category
}

func (m Model) currentEngines() []domain.Engine {
	return m.env.Catalog.Engines(m.currentCategory(), false)
}

func (m Model) currentPreset() *domain.Preset {
	presets := m.env.Catalog.Presets(m.currentCategory())
	if m.presetIndex == 0 || len(presets) == 0 {
		preference := m.env.Config.Categories[m.currentCategory()]
		if preference.DefaultPreset != "" {
			if preset, ok := m.env.Catalog.Preset(preference.DefaultPreset); ok {
				return &preset
			}
		}
		return nil
	}
	index := (m.presetIndex - 1) % len(presets)
	return &presets[index]
}

func (m Model) currentRequest() domain.SearchRequest {
	engines := m.currentEngines()
	request := domain.SearchRequest{
		Query:      m.input.Value(),
		CategoryID: m.currentCategory(),
		Action:     domain.ActionOpen,
		Output:     domain.OutputPlain,
		Modifiers: domain.Modifiers{
			Values:   make(map[string]string),
			RawQuery: make(map[string]string),
		},
	}
	if len(engines) > 0 {
		index := m.engineIndex % len(engines)
		request.EngineIDs = []string{engines[index].ID}
	}
	if preset := m.currentPreset(); preset != nil {
		request.PresetID = preset.ID
		request.EngineIDs = []string{preset.EngineID}
	}
	return request
}

func (m *Model) changeCategory(delta int) {
	m.categoryIndex = (m.categoryIndex + delta + len(m.categories)) % len(m.categories)
	m.engineIndex = m.preferredEngineIndex()
	m.presetIndex = 0
}

func (m Model) preferredEngineIndex() int {
	preference := m.env.Config.Categories[m.currentCategory()]
	engines := m.currentEngines()
	for index, engine := range engines {
		if engine.ID == preference.DefaultEngine {
			return index
		}
	}
	return 0
}

func (m *Model) newSettingsForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Default search category").Description("Used when srch starts without a selector.").Options(
				huh.NewOption("Web", "web"), huh.NewOption("Images", "images"), huh.NewOption("AI", "ai"), huh.NewOption("Music", "music"),
				huh.NewOption("Lyrics", "lyrics"), huh.NewOption("Code", "code"), huh.NewOption("Research", "research"),
			).Value(&m.env.Config.DefaultCategory),
			huh.NewSelect[string]().Title("Search input position").Options(
				huh.NewOption("Bottom", "bottom"), huh.NewOption("Top", "top"), huh.NewOption("Automatic", "auto"),
			).Value(&m.env.Config.InputPosition),
			huh.NewInput().Title("Preferred browser").Description("Leave empty to use the operating-system default.").Value(&m.env.Config.DefaultBrowser),
		).Title("Search defaults (1/3)"),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Header").Options(
				huh.NewOption("Automatic", "auto"), huh.NewOption("Full", "full"), huh.NewOption("Compact", "compact"),
			).Value(&m.env.Config.HeaderMode),
			huh.NewSelect[string]().Title("Layout density").Options(
				huh.NewOption("Compact", "compact"), huh.NewOption("Comfortable", "comfortable"), huh.NewOption("Spacious", "spacious"),
			).Value(&m.env.Config.Density),
			huh.NewSelect[string]().Title("Theme").Options(
				huh.NewOption("Automatic", "auto"), huh.NewOption("Dark", "dark"), huh.NewOption("Light", "light"), huh.NewOption("Monochrome", "monochrome"),
			).Value(&m.env.Config.Theme),
		).Title("Appearance (2/3)"),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Information density").Options(
				huh.NewOption("Minimal", "minimal"), huh.NewOption("Standard", "standard"), huh.NewOption("Detailed", "detailed"),
			).Value(&m.env.Config.Information),
			huh.NewSelect[string]().Title("Operation detail").Options(
				huh.NewOption("Quiet", "quiet"), huh.NewOption("Normal", "normal"), huh.NewOption("Verbose", "verbose"),
			).Value(&m.env.Config.OperationDetail),
			huh.NewSelect[string]().Title("Hints").Options(
				huh.NewOption("Contextual", "contextual"), huh.NewOption("Always", "always"), huh.NewOption("Off", "off"),
			).Value(&m.env.Config.Hints),
			huh.NewSelect[string]().Title("Motion").Options(
				huh.NewOption("Normal", "normal"), huh.NewOption("Reduced", "reduced"), huh.NewOption("None", "none"),
			).Value(&m.env.Config.Motion),
			huh.NewConfirm().Title("Store local search history?").Value(&m.env.Config.HistoryEnabled),
			huh.NewConfirm().Title("Enable mouse interactions?").Value(&m.env.Config.Mouse),
		).Title("Behavior (3/3)"),
	).WithWidth(max(48, min(88, m.width-4))).WithShowHelp(true)
}

func Run(environment *app.Environment) error {
	program := tea.NewProgram(New(environment))
	_, err := program.Run()
	return err
}

type styles struct {
	page          lipgloss.Style
	header        lipgloss.Style
	compactHeader lipgloss.Style
	tagline       lipgloss.Style
	title         lipgloss.Style
	panel         lipgloss.Style
	input         lipgloss.Style
	tab           lipgloss.Style
	activeTab     lipgloss.Style
	section       lipgloss.Style
	selected      lipgloss.Style
	accent        lipgloss.Style
	muted         lipgloss.Style
	status        lipgloss.Style
	footer        lipgloss.Style
	key           lipgloss.Style
	success       lipgloss.Style
	warning       lipgloss.Style
}

func newStyles(dark bool) styles {
	background := lipgloss.Color("#F7F7FA")
	foreground := lipgloss.Color("#24242B")
	muted := lipgloss.Color("#666675")
	border := lipgloss.Color("#9B82FF")
	if dark {
		background = lipgloss.Color("#17171B")
		foreground = lipgloss.Color("#F3F0FF")
		muted = lipgloss.Color("#8C8996")
		border = lipgloss.Color("#7D56F4")
	}
	base := lipgloss.NewStyle().Foreground(foreground)
	return styles{
		page:          base.Padding(1, 0),
		header:        base.Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(1, 4).Align(lipgloss.Center),
		compactHeader: base.Foreground(border).Bold(true),
		tagline:       base.Foreground(lipgloss.Color("#FF4FA3")).Bold(true),
		title:         base.Foreground(border).Bold(true).Underline(true),
		panel:         base.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4A4655")).Padding(0, 1),
		input:         base.Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1),
		tab:           base.Foreground(muted).Padding(0, 1),
		activeTab:     base.Foreground(background).Background(border).Bold(true).Padding(0, 1),
		section:       base.Foreground(lipgloss.Color("#8BE9FD")).Bold(true),
		selected:      base.Foreground(lipgloss.Color("#FF4FA3")).Bold(true),
		accent:        base.Foreground(border),
		muted:         base.Foreground(muted),
		status:        base.Foreground(muted).PaddingTop(1),
		footer:        base.PaddingTop(1).Align(lipgloss.Center),
		key:           base.Foreground(border).Bold(true),
		success:       base.Foreground(lipgloss.Color("#50FA7B")),
		warning:       base.Foreground(lipgloss.Color("#F1FA8C")),
	}
}

func title(value string) string {
	if value == "ai" {
		return "AI"
	}
	if value == "api" {
		return "API"
	}
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func wrap(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	var parts []string
	for len(value) > width {
		parts = append(parts, value[:width])
		value = value[width:]
	}
	return strings.Join(append(parts, value), "\n")
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

const manualText = `SRCH QUICK START

1. Choose Web, Images, AI, Music, Lyrics, Code, Research, or More.
2. Tab or Left/Right changes category; Ctrl+E cycles engines.
3. Type a query and press Enter.

SMART CLI

  srch @google "terminal ui"
  srch #images +transparent "band logo"
  srch #music kind:album "transilvanian hunger"
  srch --explain @github language:go "bubble tea"

UTILITIES

  Ctrl+P  command palette
  Ctrl+S  settings
  Ctrl+R  cycle presets
  Ctrl+Y  copy generated URL
  Ctrl+H  toggle header
  Ctrl+M  cycle categories under More
  ?       contextual help

Mouse users can click category tabs, the engine row, and command-palette items.

Use srch doctor to inspect optional integrations and platform support.`
