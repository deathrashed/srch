package api

import (
	"strings"
	"testing"
)

func TestBuildURLUsesStructuredEndpoints(t *testing.T) {
	tests := []struct {
		adapter string
		kind    string
		want    []string
	}{
		{adapter: "musicbrainz", kind: "release", want: []string{"musicbrainz.org/ws/2/release", "fmt=json", "query=black+sabbath"}},
		{adapter: "internet-archive", kind: "audio", want: []string{"archive.org/advancedsearch.php", "mediatype%3Aaudio", "output=json"}},
		{adapter: "open-library", kind: "author", want: []string{"openlibrary.org/search.json", "author=black+sabbath"}},
		{adapter: "crossref", kind: "title", want: []string{"api.crossref.org/works", "query.title=black+sabbath"}},
	}
	for _, test := range tests {
		t.Run(test.adapter+"/"+test.kind, func(t *testing.T) {
			rawURL, err := BuildURL(test.adapter, test.kind, "black sabbath")
			if err != nil {
				t.Fatal(err)
			}
			for _, part := range test.want {
				if !strings.Contains(rawURL, part) {
					t.Fatalf("URL %q missing %q", rawURL, part)
				}
			}
		})
	}
}

func TestDecodeOpenLibraryResults(t *testing.T) {
	body := []byte(`{"docs":[{"key":"/works/OL1W","title":"The Book","author_name":["A. Writer"],"first_publish_year":1984}]}`)
	items, err := decodeItems("open-library", "all", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "The Book" || !strings.Contains(items[0].URL, "OL1W") {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestBuildURLRejectsUnsupportedSearchType(t *testing.T) {
	if _, err := BuildURL("crossref", "album", "query"); err == nil {
		t.Fatal("expected unsupported search type error")
	}
}
