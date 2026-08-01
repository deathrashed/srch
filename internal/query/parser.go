package query

import (
	"fmt"
	"strings"

	"srch/internal/catalog"
	"srch/internal/config"
	"srch/internal/domain"
)

var recognizedModifiers = map[string]struct{}{
	"site": {}, "lang": {}, "language": {}, "since": {}, "size": {}, "color": {},
	"type": {}, "kind": {}, "stars": {}, "region": {}, "safe": {},
}

func Parse(args []string, cat *catalog.Catalog, cfg config.Config) (domain.SearchRequest, error) {
	request := domain.SearchRequest{
		CategoryID: cfg.DefaultCategory,
		Action:     domain.ActionOpen,
		Output:     domain.OutputPlain,
		Modifiers: domain.Modifiers{
			Values:   make(map[string]string),
			RawQuery: make(map[string]string),
		},
	}
	literal := false
	queryParts := make([]string, 0, len(args))
	for index, arg := range args {
		if literal {
			queryParts = append(queryParts, arg)
			continue
		}
		if arg == "--" {
			literal = true
			continue
		}
		switch {
		case strings.HasPrefix(arg, "@"):
			engine, ok := cat.Engine(strings.TrimPrefix(arg, "@"))
			if !ok {
				return request, fmt.Errorf("unknown engine %q", arg)
			}
			request.EngineIDs = []string{engine.ID}
			request.CategoryID = engine.Category
			continue
		case strings.HasPrefix(arg, "!"):
			engine, ok := cat.EngineByBang(arg)
			if !ok {
				return request, fmt.Errorf("unknown bang %q", arg)
			}
			request.EngineIDs = []string{engine.ID}
			request.CategoryID = engine.Category
			continue
		case strings.HasPrefix(arg, "#"):
			request.CategoryID = strings.TrimPrefix(strings.ToLower(arg), "#")
			continue
		case strings.HasPrefix(arg, "+"):
			preset, ok := cat.Preset(strings.TrimPrefix(arg, "+"))
			if !ok {
				return request, fmt.Errorf("unknown preset %q", arg)
			}
			request.PresetID = preset.ID
			request.EngineIDs = []string{preset.EngineID}
			request.CategoryID = preset.Category
			continue
		}
		if key, value, ok := strings.Cut(arg, ":"); ok {
			key = strings.ToLower(key)
			if _, recognized := recognizedModifiers[key]; recognized {
				if key == "lang" {
					key = "language"
				}
				request.Modifiers.Values[key] = value
				continue
			}
		}
		if index == 0 && len(args) > 1 {
			if engine, ok := cat.Engine(arg); ok {
				request.EngineIDs = []string{engine.ID}
				request.CategoryID = engine.Category
				continue
			}
		}
		queryParts = append(queryParts, arg)
	}
	request.Query = strings.TrimSpace(strings.Join(queryParts, " "))
	if request.PresetID != "" && len(request.EngineIDs) == 0 {
		preset, _ := cat.Preset(request.PresetID)
		request.EngineIDs = []string{preset.EngineID}
	}
	if len(request.EngineIDs) == 0 {
		preference, ok := cfg.Categories[request.CategoryID]
		if !ok || preference.DefaultEngine == "" {
			return request, fmt.Errorf("category %q has no preferred engine", request.CategoryID)
		}
		request.EngineIDs = []string{preference.DefaultEngine}
		if request.PresetID == "" {
			request.PresetID = preference.DefaultPreset
		}
	}
	return request, nil
}
