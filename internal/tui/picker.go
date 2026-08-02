package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	searchapi "srch/internal/api"
	"srch/internal/domain"
)

type pickerKind int

const (
	pickerCommands pickerKind = iota
	pickerCategories
	pickerEngines
	pickerTargets
	pickerPresets
	pickerOptions
	pickerAPISources
	pickerAPITypes
)

type pickerItem struct {
	name        string
	description string
	index       int
	score       int
	value       string
	action      string
}

type pickerState struct {
	kind     pickerKind
	title    string
	input    textinput.Model
	items    []pickerItem
	selected int
}

func newPicker(kind pickerKind, title, placeholder string, items []pickerItem) pickerState {
	input := textinput.New()
	input.Prompt = "⌕ "
	input.Placeholder = placeholder
	input.CharLimit = 80
	input.SetWidth(52)
	input.Focus()
	return pickerState{kind: kind, title: title, input: input, items: items}
}

func (p pickerState) filtered() []pickerItem {
	query := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if query == "" {
		return append([]pickerItem(nil), p.items...)
	}
	result := make([]pickerItem, 0, len(p.items))
	for _, item := range p.items {
		haystack := strings.ToLower(item.name + " " + item.description)
		if score, ok := fuzzySubsequence(haystack, query); ok {
			item.score = score
			result = append(result, item)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].score < result[j].score })
	return result
}

func fuzzySubsequence(value, query string) (int, bool) {
	position, score := 0, 0
	for _, wanted := range query {
		found := strings.IndexRune(value[position:], wanted)
		if found < 0 {
			return 0, false
		}
		score += found
		position += found + 1
	}
	return score, true
}

func commandPickerItems() []pickerItem {
	items := paletteItems()
	result := make([]pickerItem, len(items))
	for i, name := range items {
		result[i] = pickerItem{name: name, index: i}
	}
	return result
}

func categoryPickerItems() []pickerItem {
	result := make([]pickerItem, len(searchCategories))
	for i, id := range searchCategories {
		result[i] = pickerItem{name: categoryName(id), description: id, index: i}
	}
	return result
}

func enginePickerItems(engines []domain.Engine) []pickerItem {
	result := make([]pickerItem, len(engines))
	for i, engine := range engines {
		description := categoryName(engine.Category)
		if len(engine.Aliases) > 0 {
			description += " · " + strings.Join(engine.Aliases, ", ")
		}
		result[i] = pickerItem{name: engine.Name, description: description, index: i, value: engine.ID}
	}
	return result
}

func targetPickerItems(targets []domain.EngineTarget) []pickerItem {
	result := make([]pickerItem, len(targets))
	for i, target := range targets {
		result[i] = pickerItem{name: target.Name, description: target.ID, index: i, value: target.ID}
	}
	return result
}

func presetPickerItems(presets []domain.Preset) []pickerItem {
	result := []pickerItem{{name: "None", description: "No saved refinement preset", index: 0}}
	for i, preset := range presets {
		result = append(result, pickerItem{name: preset.Name, description: preset.ID, index: i + 1, value: preset.ID})
	}
	return result
}

func optionPickerItems(group domain.OptionGroup) []pickerItem {
	result := []pickerItem{{name: "Any", description: "Do not restrict " + strings.ToLower(group.Name), index: 0, action: group.ID}}
	for i, option := range group.Options {
		result = append(result, pickerItem{name: option.Name, description: confidenceName(group.Confidence), index: i + 1, value: option.ID, action: group.ID})
	}
	return result
}

func apiSourcePickerItems(adapters []searchapi.Adapter) []pickerItem {
	result := make([]pickerItem, len(adapters))
	for i, adapter := range adapters {
		result[i] = pickerItem{name: adapter.Name, description: adapter.Description, index: i, value: adapter.ID}
	}
	return result
}

func apiTypePickerItems(types []searchapi.SearchType) []pickerItem {
	result := make([]pickerItem, len(types))
	for i, searchType := range types {
		result[i] = pickerItem{name: searchType.Name, description: searchType.Description, index: i, value: searchType.ID}
	}
	return result
}

func confidenceName(confidence domain.CapabilityConfidence) string {
	switch confidence {
	case domain.ConfidenceDocumented:
		return "Documented"
	case domain.ConfidenceObserved:
		return "Browser-observed"
	case domain.ConfidenceDestinationOnly:
		return "Opens destination"
	default:
		return "Available"
	}
}
