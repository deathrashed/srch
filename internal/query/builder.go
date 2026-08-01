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
	u, err := url.Parse(engine.URL.Base)
	if err != nil {
		return "", fmt.Errorf("parse base URL for %s: %w", engine.Name, err)
	}
	queryText := strings.TrimSpace(request.Query)
	params := cloneMap(engine.URL.Params)
	if preset != nil {
		queryText = strings.TrimSpace(strings.Join([]string{queryText, preset.QuerySuffix}, " "))
		for key, value := range preset.Params {
			params[key] = value
		}
	}
	queryText = applyTextModifiers(queryText, request.Modifiers)
	applyValueModifiers(params, request.Modifiers.Values)
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

func applyValueModifiers(params map[string]string, values map[string]string) {
	if values == nil {
		return
	}
	tbs := make([]string, 0, 4)
	if existing := params["tbs"]; existing != "" {
		tbs = append(tbs, strings.Split(existing, ",")...)
	}
	switch values["size"] {
	case "large", "l":
		tbs = append(tbs, "isz:l")
	case "medium", "m":
		tbs = append(tbs, "isz:m")
	case "icon", "small", "s":
		tbs = append(tbs, "isz:i")
	}
	if values["color"] == "transparent" {
		tbs = append(tbs, "ic:trans")
	}
	if values["type"] == "svg" {
		tbs = append(tbs, "ift:svg")
	}
	if since := values["since"]; since != "" {
		tbs = append(tbs, "qdr:"+since)
	}
	if len(tbs) > 0 {
		params["tbs"] = dedupeJoin(tbs)
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
