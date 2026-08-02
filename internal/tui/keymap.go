package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type keyBinding struct {
	keys  []string
	label string
	help  string
}

var (
	keyQuit       = keyBinding{keys: []string{"ctrl+c"}, label: "ctrl+c", help: "quit"}
	keyPalette    = keyBinding{keys: []string{"ctrl+p"}, label: "ctrl+p", help: "commands"}
	keySettings   = keyBinding{keys: []string{"ctrl+,"}, label: "ctrl+,", help: "settings"}
	keyHelp       = keyBinding{keys: []string{"?"}, label: "?", help: "help"}
	keyMode       = keyBinding{keys: []string{"alt+left", "alt+right"}, label: "alt+←/→", help: "workspace"}
	keyNext       = keyBinding{keys: []string{"tab"}, label: "tab", help: "next field"}
	keyPrevious   = keyBinding{keys: []string{"shift+tab"}, label: "shift+tab", help: "previous field"}
	keyChange     = keyBinding{keys: []string{"left", "right"}, label: "←/→", help: "change"}
	keyFocusQuery = keyBinding{keys: []string{"/"}, label: "/", help: "query"}
	keyRun        = keyBinding{keys: []string{"enter"}, label: "enter", help: "run"}
	keyCopy       = keyBinding{keys: []string{"ctrl+y"}, label: "ctrl+y", help: "copy URL"}
	keyClose      = keyBinding{keys: []string{"esc"}, label: "esc", help: "close"}
	keyMove       = keyBinding{keys: []string{"up", "down", "k", "j"}, label: "↑/↓", help: "move"}
	keySave       = keyBinding{keys: []string{"ctrl+s"}, label: "ctrl+s", help: "save"}
)

func matchesKey(key tea.KeyPressMsg, binding keyBinding) bool {
	value := key.String()
	for _, candidate := range binding.keys {
		if value == candidate {
			return true
		}
	}
	return false
}

func footerBindings(s styles, bindings []keyBinding) string {
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		parts = append(parts, s.key.Render(binding.label)+" "+s.muted.Render(binding.help))
	}
	return strings.Join(parts, "   ")
}

func globalHelpBindings() []keyBinding {
	return []keyBinding{{label: "1…5", help: "workspace"}, keyMode, keyPalette, keySettings, keyHelp, keyClose, keyQuit}
}

func searchFooterBindings() []keyBinding {
	return []keyBinding{keyNext, keyChange, keyFocusQuery, {keys: []string{"enter"}, label: "enter", help: "choose / open"}, keyCopy, keyPalette, keySettings}
}

func apiFooterBindings(m Model) []keyBinding {
	bindings := []keyBinding{keyNext, keyChange, keyFocusQuery, {keys: []string{"enter"}, label: "enter", help: "choose / run"}, keyPalette, keySettings}
	if m.api.phase == "success" {
		bindings = append(bindings, keyMove)
	}
	return bindings
}

func modeHelpBindings(mode Mode) []keyBinding {
	switch mode {
	case ModeSearch:
		return []keyBinding{keyNext, keyPrevious, keyChange, keyFocusQuery, {label: "enter", help: "choose / open"}, keyCopy}
	case ModeReader:
		return []keyBinding{{label: "enter", help: "fetch"}, {label: "↑/↓ pgup/pgdn g/G", help: "scroll"}, {label: "/ or i", help: "edit URL"}, keyClose}
	case ModeDownloader:
		return []keyBinding{keyChange, {label: "enter", help: "download"}}
	case ModeAPI:
		return []keyBinding{keyNext, keyPrevious, keyChange, keyFocusQuery, {label: "enter", help: "choose / run"}, keyMove}
	case ModeHistory:
		return []keyBinding{keyMove, {label: "enter", help: "restore"}, {label: "d", help: "delete"}}
	default:
		return nil
	}
}
