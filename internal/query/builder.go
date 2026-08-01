package query

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"srch/internal/domain"
)

type BuiltURL struct {
	Engine domain.Engine `json:"engine"`
	URL    string        `json:"url"`
}

func Build(engine domain.Engine, preset *domain.Preset, request domain.SearchRequest) (string, error) {
	var target *domain.EngineTarget
	if preset != nil && preset.TargetID != "" {
		for index := range engine.Targets {
			if engine.Targets[index].ID == preset.TargetID {
				target = &engine.Targets[index]
				break
			}
		}
	}
	return BuildTarget(engine, target, preset, request)
}

func BuildTarget(engine domain.Engine, target *domain.EngineTarget, preset *domain.Preset, request domain.SearchRequest) (string, error) {
	u, err := url.Parse(engine.URL.Base)
	if err != nil {
		return "", fmt.Errorf("parse base URL for %s: %w", engine.Name, err)
	}
	queryText := strings.TrimSpace(request.Query)
	params := cloneMap(engine.URL.Params)
	if target != nil {
		for key, value := range target.Params {
			params[key] = value
		}
	}
	values := make(map[string]string)
	if preset != nil {
		queryText = strings.TrimSpace(strings.Join([]string{queryText, preset.QuerySuffix}, " "))
		for key, value := range preset.Params {
			params[key] = value
		}
		for key, value := range preset.Modifiers {
			values[key] = value
		}
	}
	for key, value := range request.Modifiers.Values {
		if key == "type" && value == "svg" {
			key = "format"
		}
		values[key] = value
	}
	queryText = applyTextModifiers(queryText, request.Modifiers)
	applyBindings(params, engine.Bindings, values)
	for key, value := range request.Modifiers.RawQuery {
		params[key] = value
	}

	switch engine.URL.Placement {
	case domain.PlacementQuery:
		q := u.Query()
		for key, value := range params {
			q.Set(key, value)
		}
		key := engine.URL.QueryParam
		if key == "" {
			key = "q"
		}
		q.Set(key, queryText)
		u.RawQuery = q.Encode()
	case domain.PlacementPath:
		segment := queryText
		if engine.URL.Template != "" {
			segment = strings.ReplaceAll(engine.URL.Template, "{query}", queryText)
		}
		u.Path = path.Join(u.Path, segment)
		q := u.Query()
		for key, value := range params {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	case domain.PlacementFragment:
		fragment := engine.URL.Template
		if fragment == "" {
			fragment = "{query}"
		}
		u.Fragment = strings.ReplaceAll(fragment, "{query}", url.QueryEscape(queryText))
	case domain.PlacementFixed:
		if queryText != "" && strings.Contains(engine.URL.Template, "{query}") {
			u.Path = strings.ReplaceAll(engine.URL.Template, "{query}", url.PathEscape(queryText))
		}
	default:
		return "", fmt.Errorf("engine %s has unsupported placement %q", engine.Name, engine.URL.Placement)
	}
	return u.String(), nil
}

func applyTextModifiers(queryText string, modifiers domain.Modifiers) string {
	parts := []string{queryText}
	if modifiers.Exact && queryText != "" {
		parts[0] = `"` + queryText + `"`
	}
	for _, excluded := range modifiers.Exclude {
		if excluded != "" {
			parts = append(parts, "-"+excluded)
		}
	}
	for _, key := range []string{"site", "language", "stars"} {
		if value := modifiers.Values[key]; value != "" {
			prefix := key
			if key == "language" {
				prefix = "language"
			}
			parts = append(parts, prefix+":"+value)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func applyBindings(params map[string]string, bindings map[string]domain.ModifierBinding, values map[string]string) {
	grouped := make(map[string][]string)
	for groupID, value := range values {
		binding, ok := bindings[groupID]
		if !ok || value == "" {
			continue
		}
		encoded := binding.Values[value]
		if encoded == "" && binding.Template != "" {
			encoded = fmt.Sprintf(binding.Template, value)
		}
		if encoded == "" {
			continue
		}
		param := binding.Param
		if param == "" {
			continue
		}
		grouped[param] = append(grouped[param], encoded)
	}
	for param, encoded := range grouped {
		values := encoded
		if existing := params[param]; existing != "" {
			values = append(strings.Split(existing, ","), values...)
		}
		params[param] = dedupeJoin(values)
	}
}

func cloneMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+4)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func dedupeJoin(values []string) string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return strings.Join(result, ",")
}
