package query_test

import (
	"path/filepath"
	"strings"
	"testing"

	"srch/internal/catalog"
	"srch/internal/config"
	"srch/internal/domain"
	"srch/internal/query"
)

func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestParseSmartSelectors(t *testing.T) {
	cat := loadCatalog(t)
	cfg := config.Defaults()
	tests := []struct {
		name     string
		args     []string
		engine   string
		preset   string
		category string
		query    string
	}{
		{name: "default", args: []string{"terminal", "ui"}, engine: "google", category: "web", query: "terminal ui"},
		{name: "explicit engine", args: []string{"@github", "bubble", "tea"}, engine: "github", category: "code", query: "bubble tea"},
		{name: "bang", args: []string{"!d", "private", "search"}, engine: "duckduckgo", category: "web", query: "private search"},
		{name: "category", args: []string{"#images", "cover", "art"}, engine: "google-images", preset: "google-images-large", category: "images", query: "cover art"},
		{name: "preset", args: []string{"+transparent", "band", "logo"}, engine: "google-images", preset: "google-images-transparent", category: "images", query: "band logo"},
		{name: "first token alias", args: []string{"spotify", "darkthrone"}, engine: "spotify", category: "music", query: "darkthrone"},
		{name: "literal boundary", args: []string{"@google", "--", "site:github.com"}, engine: "google", category: "web", query: "site:github.com"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := query.Parse(test.args, cat, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if request.EngineIDs[0] != test.engine || request.PresetID != test.preset || request.CategoryID != test.category || request.Query != test.query {
				t.Fatalf("unexpected request: %#v", request)
			}
		})
	}
}

func TestBuildURLs(t *testing.T) {
	cat := loadCatalog(t)
	tests := []struct {
		engine string
		preset string
		query  string
		want   []string
	}{
		{engine: "google", query: "fish shell", want: []string{"https://www.google.com/search?", "q=fish+shell"}},
		{engine: "spotify", query: "Darkthrone", want: []string{"https://open.spotify.com/search/Darkthrone"}},
		{engine: "google-images", preset: "google-images-transparent", query: "band logo", want: []string{"q=band+logo", "tbm=isch", "tbs=ic%3Atrans"}},
		{engine: "metal-archives-album", query: "Transilvanian Hunger", want: []string{"searchString=Transilvanian+Hunger", "type=album"}},
		{engine: "devdocs", query: "fetch", want: []string{"https://devdocs.io/#q=fetch"}},
	}
	for _, test := range tests {
		t.Run(test.engine+test.preset, func(t *testing.T) {
			engine, ok := cat.Engine(test.engine)
			if !ok {
				t.Fatal("engine not found")
			}
			var preset *domain.Preset
			if test.preset != "" {
				value, ok := cat.Preset(test.preset)
				if !ok {
					t.Fatal("preset not found")
				}
				preset = &value
			}
			got, err := query.Build(engine, preset, domain.SearchRequest{Query: test.query, Modifiers: domain.Modifiers{Values: map[string]string{}}})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(got, want) {
					t.Fatalf("URL %q does not contain %q", got, want)
				}
			}
		})
	}
}

func TestImageModifiersCompose(t *testing.T) {
	cat := loadCatalog(t)
	engine, _ := cat.Engine("google-images")
	request := domain.SearchRequest{
		Query: "terminal icon",
		Modifiers: domain.Modifiers{Values: map[string]string{
			"size": "large", "color": "transparent", "type": "svg", "since": "y1",
		}},
	}
	got, err := query.Build(engine, nil, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"ic%3Atrans", "ift%3Asvg", "isz%3Al", "qdr%3Ay1"} {
		if !strings.Contains(got, value) {
			t.Fatalf("URL %q does not include %q", got, value)
		}
	}
}

func TestSelectorsAreOrderIndependent(t *testing.T) {
	cat := loadCatalog(t)
	cfg := config.Defaults()
	left, err := query.Parse([]string{"@metal-archives", "+metal-album", "Black Sabbath"}, cat, cfg)
	if err != nil {
		t.Fatal(err)
	}
	right, err := query.Parse([]string{"+metal-album", "@metal-archives", "Black Sabbath"}, cat, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if left.EngineIDs[0] != right.EngineIDs[0] || left.PresetID != right.PresetID || left.CategoryID != right.CategoryID {
		t.Fatalf("selector order changed resolution: %#v != %#v", left, right)
	}
}

func TestBingDoesNotReceiveGoogleImageBindings(t *testing.T) {
	cat := loadCatalog(t)
	engine, _ := cat.Engine("bing-images")
	got, err := query.Build(engine, nil, domain.SearchRequest{Query: "logo", Modifiers: domain.Modifiers{Values: map[string]string{"size": "large"}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "tbs=") {
		t.Fatalf("provider-specific parameter leaked into Bing: %s", got)
	}
}
