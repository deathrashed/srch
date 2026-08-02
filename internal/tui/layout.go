package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	searchapi "srch/internal/api"
)

type styles struct {
	base, header, compact, title, section, panel, input, inputFocused lipgloss.Style
	tab, active, focused, selected, selectedFocused, muted            lipgloss.Style
	status, key, success, warning                                     lipgloss.Style
}

func newStyles(dark bool) styles {
	foreground, muted, border := lipgloss.Color("#24242B"), lipgloss.Color("#666675"), lipgloss.Color("#7D56F4")
	if dark {
		foreground, muted = lipgloss.Color("#F3F0FF"), lipgloss.Color("#8C8996")
	}
	base := lipgloss.NewStyle().Foreground(foreground)
	accent := lipgloss.Color("#7D56F4")
	hot := lipgloss.Color("#FF4FA3")
	return styles{
		base:    base,
		header:  base.Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(1, 4).Align(lipgloss.Center),
		compact: base.Foreground(border).Bold(true).Align(lipgloss.Center), title: base.Foreground(border).Bold(true),
		section: base.Foreground(muted).Bold(true), panel: base.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4A4655")).Padding(0, 1),
		input: base.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4A4655")).Padding(0, 1), inputFocused: base.Border(lipgloss.RoundedBorder()).BorderForeground(hot).Padding(0, 1),
		tab: base.Foreground(muted).Border(lipgloss.NormalBorder(), true, true, false, true).BorderForeground(lipgloss.Color("#4A4655")).Padding(0, 1), active: base.Foreground(lipgloss.Color("#F8F5FF")).Border(lipgloss.NormalBorder(), true, true, false, true).BorderForeground(accent).Bold(true).Padding(0, 1), focused: base.Foreground(hot).Bold(true),
		selected: base.Foreground(accent).Bold(true).Padding(0, 1), selectedFocused: base.Foreground(lipgloss.Color("#17141F")).Background(hot).Bold(true).Padding(0, 1),
		muted: base.Foreground(muted), status: base.Foreground(muted).PaddingTop(1), key: base.Foreground(border).Bold(true),
		success: base.Foreground(lipgloss.Color("#50FA7B")), warning: base.Foreground(lipgloss.Color("#F1FA8C")),
	}
}

func (m Model) contentWidth() int { return max(42, min(104, m.width-4)) }

func (m Model) render() (string, []hitRegion) {
	dark := m.dark
	if m.env.Config.Theme == "dark" {
		dark = true
	} else if m.env.Config.Theme == "light" {
		dark = false
	}
	s := newStyles(dark)
	width := m.contentWidth()
	left := max(0, (m.width-width)/2)
	var hits []hitRegion
	var lines []string
	header := m.renderHeader(s)
	lines = append(lines, header)
	modeY := lipgloss.Height(header)
	modeLine, modeHits := m.renderModeTabs(s, left, modeY)
	lines = append(lines, modeLine)
	hits = append(hits, modeHits...)
	bodyY := modeY + lipgloss.Height(modeLine)
	var body string
	var bodyHits []hitRegion
	switch m.mode {
	case ModeSearch:
		body, bodyHits = m.renderSearch(s, left, bodyY)
	case ModeReader:
		body, bodyHits = m.renderReader(s, left, bodyY)
	case ModeDownloader:
		body, bodyHits = m.renderDownloader(s, left, bodyY)
	case ModeAPI:
		body, bodyHits = m.renderAPI(s, left, bodyY)
	case ModeHistory:
		body, bodyHits = m.renderHistory(s, left, bodyY)
	}
	lines = append(lines, body)
	hits = append(hits, bodyHits...)
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, s.base.Width(width).Render(strings.Join(lines, "\n")))
	if m.overlay == overlayNone {
		return content, hits
	}
	panel, overlayHits := m.renderOverlay(s)
	panelX := max(0, (m.width-lipgloss.Width(panel))/2)
	panelY := max(1, (min(m.height, lipgloss.Height(content))-lipgloss.Height(panel))/2)
	panel, overlayHits = m.renderOverlayAt(s, panelX, panelY)
	composed := lipgloss.NewCompositor(
		lipgloss.NewLayer(content).X(0).Y(0).Z(0),
		lipgloss.NewLayer(panel).X(panelX).Y(panelY).Z(1),
	).Render()
	return composed, overlayHits
}

func (m Model) renderOverlay(s styles) (string, []hitRegion) { return m.renderOverlayAt(s, 0, 0) }

func (m Model) renderOverlayAt(s styles, left, y int) (string, []hitRegion) {
	switch m.overlay {
	case overlayPicker:
		return m.renderPicker(s, left, y)
	case overlaySettings:
		return m.renderSettings(s, left, y)
	case overlayHelp:
		return m.renderHelp(s), nil
	case overlayDetails:
		return m.renderDoctor(s), nil
	default:
		return "", nil
	}
}

func (m Model) renderHeader(s styles) string {
	subtitles := []string{"SEARCH  •  FIND  •  OPEN", "READER  •  FETCH  •  READ", "DOWNLOADER  •  SAVE  •  MONITOR", "API  •  QUERY  •  INSPECT", "HISTORY  •  REVISIT  •  REUSE"}
	subtitle := subtitles[m.mode]
	if m.width < 88 || m.height < 25 || m.env.Config.HeaderMode == "compact" {
		return s.compact.Width(m.contentWidth()).Render("SRCH  ·  " + subtitle)
	}
	banner := strings.Join([]string{"███████╗██████╗  ██████╗██╗  ██╗", "██╔════╝██╔══██╗██╔════╝██║  ██║", "███████╗██████╔╝██║     ███████║", "╚════██║██╔══██╗██║     ██╔══██║", "███████║██║  ██║╚██████╗██║  ██║", "╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝"}, "\n")
	return s.header.Width(m.contentWidth() - 2).Render(banner + "\n\n" + s.focused.Render(subtitle))
}

func (m Model) renderModeTabs(s styles, left, y int) (string, []hitRegion) {
	parts := make([]string, len(modeNames))
	hits := make([]hitRegion, 0, len(parts))
	plainWidth := 0
	for i, name := range modeNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if Mode(i) == m.mode {
			parts[i] = s.active.Render(label)
		} else {
			parts[i] = s.tab.Render(label)
		}
		plainWidth += lipgloss.Width(parts[i])
	}
	start := left + max(0, (m.contentWidth()-plainWidth)/2)
	cursor := start
	for i, part := range parts {
		hits = append(hits, hitRegion{x: cursor, y: y + 1, w: lipgloss.Width(part), action: "mode", index: i})
		cursor += lipgloss.Width(part)
	}
	return lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)), hits
}

func (m Model) renderSearch(s styles, left, y int) (string, []hitRegion) {
	rows := []string{
		"",
		sectionLine(s, "SOURCE", m.contentWidth()),
	}
	var hits []hitRegion
	y += 2
	controlIndex := 0
	categoryNames := make([]string, len(searchCategories))
	for index, categoryID := range searchCategories {
		categoryNames[index] = categoryName(categoryID)
	}
	category, categoryHits := selectorRow("Category", categoryNames, m.categoryIndex, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "category")
	rows = append(rows, category)
	hits = append(hits, categoryHits...)
	y++
	controlIndex++
	engines := m.currentEngines()
	engineNames := make([]string, len(engines))
	for i, e := range engines {
		engineNames[i] = e.Name
	}
	engine, engineHits := selectorRow("Engine", engineNames, m.engineIndex, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "engine")
	rows = append(rows, engine)
	hits = append(hits, engineHits...)
	y++
	controlIndex++
	if active := m.currentEngine(); active != nil && len(active.Targets) > 1 {
		names := make([]string, len(active.Targets))
		for i, t := range active.Targets {
			names[i] = t.Name
		}
		target, targetHits := selectorRow("Target", names, m.targetIndex, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "target")
		rows = append(rows, target)
		hits = append(hits, targetHits...)
		y++
		controlIndex++
	}
	target := m.currentTarget()
	presets := m.currentPresets()
	hasRefinements := (target != nil && len(target.OptionGroups) > 0) || len(presets) > 0
	if hasRefinements {
		rows = append(rows, "", sectionLine(s, "REFINE", m.contentWidth()))
		y += 2
	}
	if target != nil {
		for _, group := range target.OptionGroups {
			names := []string{"Any"}
			selected := 0
			for index, option := range group.Options {
				names = append(names, option.Name)
				if m.filterValues[group.ID] == option.ID {
					selected = index + 1
				}
			}
			row, filterHits := selectorRow(group.Name, names, selected, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "filter:"+group.ID)
			rows = append(rows, row)
			hits = append(hits, filterHits...)
			y++
			controlIndex++
		}
	}
	if len(presets) > 0 {
		names := []string{"None"}
		for _, p := range presets {
			names = append(names, p.Name)
		}
		preset, presetHits := selectorRow("Preset", names, m.presetIndex, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "preset")
		rows = append(rows, preset)
		hits = append(hits, presetHits...)
		y++
		controlIndex++
	}
	if target != nil {
		for _, field := range target.Fields {
			input := m.fieldInputs[field.ID]
			marker := "  "
			labelStyle := s.muted
			valueStyle := s.selected
			if m.focusIndex == controlIndex {
				marker = s.focused.Render("› ")
				labelStyle = s.focused
				valueStyle = s.selectedFocused
			}
			valueWidth := max(18, min(48, m.contentWidth()-30))
			row := marker + labelStyle.Width(11).Render(field.Name) + "  " + valueStyle.Width(valueWidth).Render(input.View())
			rows = append(rows, row)
			hits = append(hits, hitRegion{x: left + 15, y: y, w: valueWidth, action: "field:" + field.ID})
			y++
			controlIndex++
		}
	}
	rows = append(rows, "", sectionLine(s, "QUERY", m.contentWidth()))
	inputStyle := s.input
	if m.focusIndex == controlIndex {
		inputStyle = s.inputFocused
	}
	input := inputStyle.Width(max(20, m.contentWidth()-2)).Render(m.input.View())
	rows = append(rows, input)
	hits = append(hits, hitRegion{x: left, y: y + 2, w: m.contentWidth(), action: "query"})
	if m.status != "" {
		status := m.status
		if m.busy {
			status = m.spinner.View() + " " + status
		}
		rows = append(rows, s.status.Render(status))
	}
	rows = append(rows, "", s.muted.Render(strings.Repeat("─", m.contentWidth())), footerBindings(s, searchFooterBindings()))
	return strings.Join(rows, "\n"), hits
}

func selectorRow(label string, items []string, selected int, focused bool, s styles, left, y, width int, action string) (string, []hitRegion) {
	if len(items) == 0 {
		return s.muted.Render(label + "  unavailable"), nil
	}
	selected = min(max(selected, 0), len(items)-1)
	marker := "  "
	labelStyle := s.muted
	valueStyle := s.selected
	if focused {
		marker = s.focused.Render("› ")
		labelStyle = s.focused
		valueStyle = s.selectedFocused
	}
	prefix := marker + labelStyle.Width(11).Render(label)
	valueWidth := max(18, min(30, width-34))
	value := valueStyle.Width(valueWidth).Render(items[selected])
	position := s.muted.Render(fmt.Sprintf("%d of %d", selected+1, len(items)))
	hint := ""
	if focused {
		hint = "  " + s.key.Render("←/→") + " " + s.muted.Render("change")
	}
	hits := []hitRegion{}
	if action != "" {
		next := (selected + 1) % len(items)
		hits = append(hits, hitRegion{x: left + 15, y: y, w: valueWidth, action: action, index: next})
	}
	return prefix + "  " + value + "  " + position + hint, hits
}

func sectionLine(s styles, label string, width int) string {
	prefix := "─ " + label + " "
	return s.section.Render(prefix + strings.Repeat("─", max(0, width-lipgloss.Width(prefix))))
}

func categoryName(id string) string {
	switch id {
	case "ai":
		return "AI"
	case "code":
		return "Code"
	case "images":
		return "Images"
	case "lyrics":
		return "Lyrics"
	case "music":
		return "Music"
	case "research":
		return "Research"
	case "social":
		return "Social"
	case "shopping":
		return "Shopping"
	case "video":
		return "Video"
	case "web":
		return "Web"
	case "extensions":
		return "Extensions"
	case "reference":
		return "Reference"
	case "privacy":
		return "Privacy"
	case "health":
		return "Health"
	default:
		if id == "" {
			return ""
		}
		return strings.ToUpper(id[:1]) + id[1:]
	}
}
func (m Model) renderReader(s styles, left, y int) (string, []hitRegion) {
	parts := []string{s.title.Render("Reader"), s.muted.Render("Fetch a page, extract the article, and read it without leaving the terminal."), s.input.Render(m.input.View())}
	inputY := y + lipgloss.Height(parts[0]) + 1 + lipgloss.Height(parts[1]) + 1
	hits := []hitRegion{{x: left, y: inputY, w: m.contentWidth(), action: "reader-input"}}
	if m.content != "" {
		progress := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)
		parts = append(parts, s.panel.Width(m.contentWidth()-4).Render(m.viewport.View()), s.muted.Render("↑/↓ scroll  pgup/pgdn page  g/G ends  / edit URL")+"  "+s.key.Render(progress))
	}
	if m.status != "" {
		status := m.status
		if m.busy {
			status = m.spinner.View() + " " + status
		}
		parts = append(parts, s.status.Render(status))
	}
	parts = append(parts, footerBindings(s, []keyBinding{{label: "enter", help: "fetch"}, {label: "↑/↓ pgup/pgdn", help: "scroll"}, keyMode, keyPalette, keySettings}))
	return strings.Join(parts, "\n\n"), hits
}
func (m Model) renderDownloader(s styles, left, y int) (string, []hitRegion) {
	kinds := []string{"Direct", "Media (yt-dlp)"}
	parts := []string{s.title.Render("Downloader"), "Mode: " + s.selected.Render(kinds[m.downloadKind]) + "   ←/→ changes mode\nDownloads save to " + m.env.Config.DownloadDir, s.input.Render(m.input.View())}
	modeY := y + lipgloss.Height(parts[0]) + 1
	inputY := modeY + lipgloss.Height(parts[1]) + 1
	hits := []hitRegion{{x: left, y: modeY, w: m.contentWidth(), action: "download-kind"}, {x: left, y: inputY, w: m.contentWidth(), action: "download-input"}}
	if m.content != "" {
		parts = append(parts, s.panel.Render(m.content))
	}
	if m.status != "" {
		status := m.status
		if m.busy {
			status = m.spinner.View() + " " + status
		}
		parts = append(parts, s.status.Render(status))
	}
	button := s.active.Render("Download")
	buttonX := left + max(0, (m.contentWidth()-lipgloss.Width(button))/2)
	buttonY := y
	for _, part := range parts {
		buttonY += lipgloss.Height(part) + 1
	}
	parts = append(parts, lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, button), footerBindings(s, []keyBinding{keyRun, keyChange, keyMode, keyPalette, keySettings}))
	hits = append(hits, hitRegion{x: buttonX, y: buttonY + 1, w: lipgloss.Width(button), action: "download-run"})
	return strings.Join(parts, "\n\n"), hits
}
func (m Model) renderAPI(s styles, left, y int) (string, []hitRegion) {
	adapter := m.currentAPIAdapter()
	searchType := m.currentAPIType()
	rows := []string{}
	hits := []hitRegion{}
	cursorY := y
	appendRow := func(row string) {
		rows = append(rows, row)
		cursorY += max(1, lipgloss.Height(row))
	}
	appendRow(s.title.Render("Structured API Search"))
	if m.height >= 28 {
		appendRow(s.muted.Width(m.contentWidth()).Render("Choose a documented data source, narrow what it searches, then run a live query."))
	}
	appendRow(sectionLine(s, "1 SOURCE", m.contentWidth()))
	adapters := searchapi.Adapters()
	sourceNames := make([]string, len(adapters))
	for i, candidate := range adapters {
		sourceNames[i] = candidate.Name
	}
	source, sourceHits := selectorRow("Source", sourceNames, m.api.sourceIndex, m.api.focusIndex == 0, s, left, cursorY, m.contentWidth(), "api-source")
	appendRow(source)
	hits = append(hits, sourceHits...)
	typeNames := make([]string, len(adapter.Types))
	for i, candidate := range adapter.Types {
		typeNames[i] = candidate.Name
	}
	typeRow, typeHits := selectorRow("Search type", typeNames, m.api.typeIndex, m.api.focusIndex == 1, s, left, cursorY, m.contentWidth(), "api-type")
	appendRow(typeRow)
	hits = append(hits, typeHits...)
	availability := adapter.Confidence + " · " + adapter.Auth
	if m.height >= 28 {
		appendRow(s.muted.Width(m.contentWidth()).Render(adapter.Description))
		appendRow(s.muted.Width(m.contentWidth()).Render(searchType.Description + " · " + availability))
	} else {
		appendRow(s.muted.Width(m.contentWidth()).Render(adapter.Name + " · " + searchType.Name + " · " + availability))
	}
	appendRow(sectionLine(s, "2 QUERY", m.contentWidth()))
	inputStyle := s.input
	if m.api.focusIndex == 2 {
		inputStyle = s.inputFocused
	}
	hits = append(hits, hitRegion{x: left, y: cursorY, w: m.contentWidth(), action: "api-query"})
	appendRow(inputStyle.Width(max(20, m.contentWidth()-2)).Render(m.input.View()))
	appendRow(sectionLine(s, "3 ACTIONS", m.contentWidth()))
	actions := make([]string, len(apiActionNames))
	actionWidth := 0
	for i, name := range apiActionNames {
		style := s.tab
		if i == m.api.actionIndex {
			style = s.active
		}
		actions[i] = style.Render(name)
		actionWidth += lipgloss.Width(actions[i])
	}
	chunks := [][]string{actions}
	if actionWidth > m.contentWidth() {
		middle := (len(actions) + 1) / 2
		chunks = [][]string{actions[:middle], actions[middle:]}
	}
	actionIndex := 0
	for _, chunk := range chunks {
		line := lipgloss.JoinHorizontal(lipgloss.Center, chunk...)
		start := left + max(0, (m.contentWidth()-lipgloss.Width(line))/2)
		cursor := start
		for _, action := range chunk {
			hits = append(hits, hitRegion{x: cursor, y: cursorY + 1, w: lipgloss.Width(action), action: "api-action", index: actionIndex})
			cursor += lipgloss.Width(action)
			actionIndex++
		}
		appendRow(lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, line))
	}
	if m.status != "" {
		status := m.status
		if m.busy {
			status = m.spinner.View() + " " + status
		}
		appendRow(s.status.Render(status))
	}
	if m.api.phase == "loading" {
		appendRow(s.muted.Width(m.contentWidth()).Render("The request is running in the background. The interface remains available."))
	}
	if m.api.phase == "success" {
		appendRow(sectionLine(s, "RESULTS", m.contentWidth()))
		if len(m.api.result.Items) == 0 {
			appendRow(s.muted.Render("No matching results. Try a broader query or another search type."))
		} else {
			visibleLimit := 8
			if m.height < 34 {
				visibleLimit = 3
			}
			start := 0
			if m.api.selected >= visibleLimit {
				start = m.api.selected - visibleLimit + 1
			}
			end := min(start+visibleLimit, len(m.api.result.Items))
			for i := start; i < end; i++ {
				item := m.api.result.Items[i]
				prefix := "  "
				style := s.base
				if i == m.api.selected {
					prefix = "› "
					style = s.focused
				}
				line := style.Render(prefix + item.Title)
				if item.Subtitle != "" {
					line += "  " + s.muted.Render(item.Subtitle)
				}
				hits = append(hits, hitRegion{x: left, y: cursorY, w: min(m.contentWidth(), lipgloss.Width(line)), action: "api-result", index: i})
				appendRow(line)
			}
			appendRow(s.panel.Width(m.contentWidth() - 4).Render(m.viewport.View()))
		}
	}
	appendRow(s.muted.Render(strings.Repeat("─", m.contentWidth())))
	appendRow(footerBindings(s, apiFooterBindings(m)))
	return strings.Join(rows, "\n"), hits
}
func taskView(s styles, title, description, input, content, status string, busy bool, spin string) string {
	parts := []string{s.title.Render(title), s.muted.Render(description), s.input.Render(input)}
	if content != "" {
		parts = append(parts, s.panel.Render(content))
	}
	if status != "" {
		if busy {
			status = spin + " " + status
		}
		parts = append(parts, s.status.Render(status))
	}
	parts = append(parts, footerBindings(s, []keyBinding{keyRun, keyChange, keyMode, keyPalette, keySettings}))
	return strings.Join(parts, "\n\n")
}

func (m Model) renderHistory(s styles, left, y int) (string, []hitRegion) {
	entries, err := m.env.History.List()
	if err != nil {
		return s.warning.Render(err.Error()), nil
	}
	lines := []string{s.title.Render("History")}
	hits := []hitRegion{}
	rowY := y + lipgloss.Height(lines[0]) + 1
	if len(entries) == 0 {
		lines = append(lines, "", s.muted.Render("No searches recorded yet."))
	} else {
		for i, entry := range entries {
			prefix := "  "
			style := s.base
			if i == m.historyIndex {
				prefix = "› "
				style = s.focused
			}
			lines = append(lines, style.Render(prefix+entry.CreatedAt.Local().Format("2006-01-02 15:04")+"  "+entry.Query))
			hits = append(hits, hitRegion{x: left, y: rowY, w: m.contentWidth(), action: "history", index: i})
			rowY++
		}
	}
	lines = append(lines, "", footer(s, "↑/↓", "select", "enter", "rerun", "d", "delete", "ctrl+p", "commands"))
	return strings.Join(lines, "\n"), hits
}

func (m Model) renderPicker(s styles, left, y int) (string, []hitRegion) {
	items := m.picker.filtered()
	lines := []string{s.title.Render(m.picker.title), s.inputFocused.Width(56).Render(m.picker.input.View()), ""}
	hits := []hitRegion{}
	start := 0
	if m.picker.selected >= 10 {
		start = m.picker.selected - 9
	}
	end := min(start+10, len(items))
	for i := start; i < end; i++ {
		item := items[i]
		style := s.tab
		prefix := "  "
		if i == m.picker.selected {
			style = s.focused
			prefix = "› "
		}
		line := style.UnsetBorderStyle().UnsetPadding().Render(prefix + item.name)
		if item.description != "" && item.description != strings.ToLower(item.name) {
			line += "  " + s.muted.Render(item.description)
		}
		lines = append(lines, line)
		hits = append(hits, hitRegion{x: left + 2, y: y + 4 + i - start, w: lipgloss.Width(line), action: "picker", index: i})
	}
	if len(items) == 0 {
		lines = append(lines, s.muted.Render("No matching items"))
	} else if len(items) > 10 {
		lines = append(lines, s.muted.Render(fmt.Sprintf("Showing %d–%d of %d", start+1, end, len(items))))
	}
	lines = append(lines, "", footer(s, "type", "filter", "↑/↓", "choose", "enter", "select", "esc", "close"))
	return s.panel.Width(max(48, min(70, m.contentWidth()-4))).Render(strings.Join(lines, "\n")), hits
}

func (m Model) renderSettings(s styles, left, y int) (string, []hitRegion) {
	rows := [][2]string{{"Default category", categoryName(m.settings.draft.DefaultCategory)}, {"Search input", categoryName(m.settings.draft.InputPosition)}, {"Header", categoryName(m.settings.draft.HeaderMode)}, {"Density", categoryName(m.settings.draft.Density)}, {"Theme", categoryName(m.settings.draft.Theme)}, {"Motion", categoryName(m.settings.draft.Motion)}, {"Store history", yesNo(m.settings.draft.HistoryEnabled)}, {"Mouse", yesNo(m.settings.draft.Mouse)}}
	lines := []string{s.title.Render("Settings"), s.muted.Render("Draft values · Ctrl+S applies · Esc discards"), ""}
	hits := []hitRegion{}
	for i, row := range rows {
		marker := "  "
		name := s.base.Render(fmt.Sprintf("%-20s", row[0]))
		value := s.selected.Render(row[1])
		if i == m.settings.index {
			marker = "› "
			name = s.focused.Render(fmt.Sprintf("%-20s", row[0]))
		}
		line := marker + name + "  " + value
		lines = append(lines, line)
		hits = append(hits, hitRegion{x: left + 2, y: y + 4 + i, w: lipgloss.Width(line), action: "settings", index: i})
	}
	lines = append(lines, "", footer(s, "↑/↓", "setting", "←/→", "change", "ctrl+s", "apply", "esc", "discard"))
	return s.panel.Width(max(44, min(76, m.contentWidth()-2))).Render(strings.Join(lines, "\n")), hits
}
func yesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func (m Model) renderHelp(s styles) string {
	sections := []string{
		s.title.Render("SRCH Manual & Help"),
		s.muted.Render("Navigate every workspace with one shared key map. Visible commands change with the active workspace."),
		sectionLine(s, "GLOBAL", 64),
		footerBindings(s, globalHelpBindings()),
		sectionLine(s, strings.ToUpper(modeNames[m.mode]), 64),
		footerBindings(s, modeHelpBindings(m.mode)),
	}
	switch m.mode {
	case ModeSearch:
		sections = append(sections, "Tab / Shift+Tab moves through source, target, refinements, presets, and query.\nEnter on any selector opens fuzzy selection. Arrow keys cycle values.\n/ focuses the query. Enter opens the search. Ctrl+Y copies its URL.")
	case ModeReader:
		sections = append(sections, "Enter fetches the URL. Once loaded, use arrows, PgUp/PgDn, g/G, or the mouse wheel.\nPress / or i to edit the URL; Esc clears the article and returns to input.")
	case ModeDownloader:
		sections = append(sections, "Left/Right selects Direct or Media. Enter starts a contained background job.\nyt-dlp output is captured and summarized here; it never takes over the terminal.")
	case ModeAPI:
		sections = append(sections, "Choose Source → Search type → Query → Action. Run executes the documented API in the background and normalizes results.\nOpen, Copy URL, Export JSON, Send to Reader, and Raw JSON act on the selected result. API keys are not required for the bundled sources.")
	case ModeHistory:
		sections = append(sections, "Up/Down selects a prior search. Enter restores it. D deletes the selected entry.")
	}
	return s.panel.Width(max(54, min(74, m.contentWidth()-4))).Render(strings.Join(sections, "\n\n"))
}
func (m Model) renderDoctor(s styles) string {
	tools := []string{"defuddle", "glow", "bat", "curl", "aria2c", "yt-dlp", "ffmpeg"}
	lines := []string{s.title.Render("Doctor")}
	for _, tool := range tools {
		state := s.warning.Render("not detected")
		if m.env.Platform.Runner != nil && m.env.Platform.Available(tool) {
			state = s.success.Render("available")
		}
		lines = append(lines, fmt.Sprintf("%-10s %s", tool, state))
	}
	return s.panel.Render(strings.Join(lines, "\n"))
}
func footer(s styles, items ...string) string {
	parts := []string{}
	for i := 0; i+1 < len(items); i += 2 {
		parts = append(parts, s.key.Render(items[i])+" "+s.muted.Render(items[i+1]))
	}
	return strings.Join(parts, "   ")
}
