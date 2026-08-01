package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
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
		tab: base.Foreground(muted).Padding(0, 1), active: base.Foreground(lipgloss.Color("#F8F5FF")).Background(accent).Bold(true).Padding(0, 1), focused: base.Foreground(hot).Bold(true),
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
	bodyY := modeY + 1
	var body string
	var bodyHits []hitRegion
	switch m.overlay {
	case overlayPalette:
		body, bodyHits = m.renderPalette(s, left, bodyY)
	case overlaySettings:
		body, bodyHits = m.renderSettings(s, left, bodyY)
	case overlayHelp:
		body = m.renderHelp(s)
	case overlayDetails:
		body = m.renderDoctor(s)
	default:
		switch m.mode {
		case ModeSearch:
			body, bodyHits = m.renderSearch(s, left, bodyY)
		case ModeReader:
			body = m.renderReader(s)
		case ModeDownloader:
			body = m.renderDownloader(s)
		case ModeAPI:
			body = m.renderAPI(s)
		case ModeHistory:
			body = m.renderHistory(s)
		}
	}
	lines = append(lines, body)
	hits = append(hits, bodyHits...)
	content := strings.Join(lines, "\n")
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, s.base.Width(width).Render(content)), hits
}

func (m Model) renderHeader(s styles) string {
	subtitles := []string{"SEARCH  •  FIND  •  OPEN", "READER  •  FETCH  •  READ", "DOWNLOADER  •  SAVE  •  MONITOR", "API  •  QUERY  •  INSPECT", "HISTORY  •  REVISIT  •  REUSE"}
	subtitle := subtitles[m.mode]
	if m.overlay == overlaySettings {
		subtitle = "SETTINGS  •  PREFERENCES"
	} else if m.overlay == overlayPalette {
		subtitle = "COMMANDS  •  NAVIGATE  •  ACT"
	}
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
		if i > 0 {
			plainWidth++
		}
	}
	start := left + max(0, (m.contentWidth()-plainWidth)/2)
	cursor := start
	for i, part := range parts {
		hits = append(hits, hitRegion{x: cursor, y: y, w: lipgloss.Width(part), action: "mode", index: i})
		cursor += lipgloss.Width(part) + 1
	}
	return lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, strings.Join(parts, " ")), hits
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
		target, _ := selectorRow("Target", names, m.targetIndex, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "")
		rows = append(rows, target)
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
			row, _ := selectorRow(group.Name, names, selected, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "")
			rows = append(rows, row)
			y++
			controlIndex++
		}
	}
	if len(presets) > 0 {
		names := []string{"None"}
		for _, p := range presets {
			names = append(names, p.Name)
		}
		preset, _ := selectorRow("Preset", names, m.presetIndex, m.focusIndex == controlIndex, s, left, y, m.contentWidth(), "")
		rows = append(rows, preset)
		y++
		controlIndex++
	}
	if target != nil {
		for _, field := range target.Fields {
			rows = append(rows, s.muted.Render(field.Name+": ")+field.Placeholder)
			y++
		}
	}
	rows = append(rows, "", sectionLine(s, "QUERY", m.contentWidth()))
	inputStyle := s.input
	if m.focusIndex == controlIndex {
		inputStyle = s.inputFocused
	}
	input := inputStyle.Width(max(20, m.contentWidth()-2)).Render(m.input.View())
	rows = append(rows, input)
	if m.status != "" {
		status := m.status
		if m.busy {
			status = m.spinner.View() + " " + status
		}
		rows = append(rows, s.status.Render(status))
	}
	rows = append(rows, "", s.muted.Render(strings.Repeat("─", m.contentWidth())), footer(s, "tab", "next field", "←/→", "change", "/", "query", "enter", "open", "ctrl+p", "commands", "ctrl+,", "settings"))
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
	default:
		return id
	}
}
func (m Model) renderReader(s styles) string {
	return taskView(s, "Reader", "Enter a URL. Built-in extraction uses Defuddle and renders Markdown with Glamour.", m.input.View(), m.content, m.status, m.busy, m.spinner.View())
}
func (m Model) renderDownloader(s styles) string {
	kinds := []string{"Direct", "Media (yt-dlp)"}
	return taskView(s, "Downloader", "Mode: "+s.selected.Render(kinds[m.downloadKind])+"   ←/→ changes mode\nDownloads save to "+m.env.Config.DownloadDir, m.input.View(), m.content, m.status, m.busy, m.spinner.View())
}
func (m Model) renderAPI(s styles) string {
	engine := "No API adapters"
	if engines := m.apiEngines(); len(engines) > 0 {
		engine = engines[m.apiEngineIndex%len(engines)].Name
	}
	return taskView(s, "API", "Engine: "+s.selected.Render(engine)+"   ←/→ changes adapter\nCompile an API-backed request and inspect its URL or returned content.", m.input.View(), m.content, m.status, m.busy, m.spinner.View())
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
	parts = append(parts, footer(s, "enter", "run", "alt+←/→", "mode", "ctrl+p", "commands", "ctrl+,", "settings"))
	return strings.Join(parts, "\n\n")
}

func (m Model) renderHistory(s styles) string {
	entries, err := m.env.History.List()
	if err != nil {
		return s.warning.Render(err.Error())
	}
	lines := []string{s.title.Render("History")}
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
		}
	}
	lines = append(lines, "", footer(s, "↑/↓", "select", "enter", "rerun", "d", "delete", "ctrl+p", "commands"))
	return strings.Join(lines, "\n")
}

func (m Model) renderPalette(s styles, left, y int) (string, []hitRegion) {
	items := paletteItems()
	lines := []string{s.title.Render("Command Palette")}
	hits := []hitRegion{}
	for i, item := range items {
		style := s.tab
		prefix := "  "
		if i == m.paletteIndex {
			style = s.focused
			prefix = "› "
		}
		line := style.Render(prefix + item)
		lines = append(lines, line)
		hits = append(hits, hitRegion{x: left + 2, y: y + 2 + i, w: lipgloss.Width(line), action: "palette", index: i})
	}
	lines = append(lines, "", footer(s, "↑/↓", "choose", "enter", "run", "esc", "close"))
	return s.panel.Width(max(40, min(70, m.contentWidth()-2))).Render(strings.Join(lines, "\n")), hits
}

func (m Model) renderSettings(s styles, left, y int) (string, []hitRegion) {
	rows := [][2]string{{"Default category", m.settings.draft.DefaultCategory}, {"Search input", m.settings.draft.InputPosition}, {"Header", m.settings.draft.HeaderMode}, {"Density", m.settings.draft.Density}, {"Theme", m.settings.draft.Theme}, {"Motion", m.settings.draft.Motion}, {"Store history", yesNo(m.settings.draft.HistoryEnabled)}, {"Mouse", yesNo(m.settings.draft.Mouse)}}
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
	return s.panel.Render(s.title.Render("Manual & Help") + "\n\n" + "Alt+Left/Right or Ctrl+1…5 switches product modes.\nTab moves through Search controls; arrows change the focused value.\nCtrl+P opens commands. Ctrl+, opens transactional Settings.\nEnter runs the current mode. Esc closes the top overlay.")
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
