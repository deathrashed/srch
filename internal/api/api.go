package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"srch/internal/fetch"
)

type SearchType struct {
	ID          string
	Name        string
	Description string
}

type Adapter struct {
	ID          string
	Name        string
	Description string
	Auth        string
	Confidence  string
	Types       []SearchType
}

type Item struct {
	Title       string
	Subtitle    string
	URL         string
	Description string
}

type Result struct {
	Adapter  Adapter
	Type     SearchType
	URL      string
	Status   string
	Duration time.Duration
	Items    []Item
	Raw      []byte
}

var adapters = []Adapter{
	{
		ID: "musicbrainz", Name: "MusicBrainz", Description: "Artists, releases, recordings, works, and labels from the open music encyclopedia.", Auth: "No API key required", Confidence: "Documented API",
		Types: []SearchType{{ID: "artist", Name: "Artists", Description: "Find performers and groups"}, {ID: "release", Name: "Releases", Description: "Find specific album and single editions"}, {ID: "recording", Name: "Recordings", Description: "Find individual recorded tracks"}, {ID: "work", Name: "Works", Description: "Find compositions and songs"}, {ID: "label", Name: "Labels", Description: "Find record labels"}},
	},
	{
		ID: "internet-archive", Name: "Internet Archive", Description: "Search digitized books, audio, video, software, images, and web collections.", Auth: "No API key required", Confidence: "Documented API",
		Types: []SearchType{{ID: "all", Name: "All media", Description: "Search every media type"}, {ID: "texts", Name: "Texts", Description: "Books and documents"}, {ID: "audio", Name: "Audio", Description: "Music, radio, and spoken word"}, {ID: "movies", Name: "Video", Description: "Films and video"}, {ID: "software", Name: "Software", Description: "Programs and games"}},
	},
	{
		ID: "open-library", Name: "Open Library", Description: "Search books, authors, editions, and publication metadata.", Auth: "No API key required", Confidence: "Documented API",
		Types: []SearchType{{ID: "all", Name: "Books", Description: "Search titles, authors, subjects, and ISBNs"}, {ID: "title", Name: "Title", Description: "Match book titles"}, {ID: "author", Name: "Author", Description: "Match author names"}, {ID: "subject", Name: "Subject", Description: "Match subjects and topics"}},
	},
	{
		ID: "crossref", Name: "Crossref", Description: "Search scholarly works and DOI metadata from publishers and repositories.", Auth: "No API key required", Confidence: "Documented API",
		Types: []SearchType{{ID: "all", Name: "Works", Description: "Search titles, authors, and bibliographic metadata"}, {ID: "title", Name: "Title", Description: "Search work titles"}, {ID: "author", Name: "Author", Description: "Search author names"}, {ID: "bibliographic", Name: "Bibliographic", Description: "Search citations and references"}},
	},
}

func Adapters() []Adapter {
	result := make([]Adapter, len(adapters))
	copy(result, adapters)
	return result
}

func Lookup(id string) (Adapter, bool) {
	for _, adapter := range adapters {
		if adapter.ID == id {
			return adapter, true
		}
	}
	return Adapter{}, false
}

func BuildURL(adapterID, typeID, query string) (string, error) {
	adapter, ok := Lookup(adapterID)
	if !ok {
		return "", fmt.Errorf("unknown API adapter %q", adapterID)
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("enter a query to search %s", adapter.Name)
	}
	if !hasType(adapter, typeID) {
		return "", fmt.Errorf("%s does not support search type %q", adapter.Name, typeID)
	}
	q := url.QueryEscape(strings.TrimSpace(query))
	switch adapterID {
	case "musicbrainz":
		return "https://musicbrainz.org/ws/2/" + typeID + "/?fmt=json&dismax=true&limit=20&query=" + q, nil
	case "internet-archive":
		if typeID != "all" {
			q = url.QueryEscape("mediatype:" + typeID + " AND (" + strings.TrimSpace(query) + ")")
		}
		return "https://archive.org/advancedsearch.php?output=json&rows=20&fl[]=identifier&fl[]=title&fl[]=description&fl[]=mediatype&fl[]=date&q=" + q, nil
	case "open-library":
		parameter := "q"
		if typeID != "all" {
			parameter = typeID
		}
		return "https://openlibrary.org/search.json?limit=20&" + parameter + "=" + q, nil
	case "crossref":
		parameter := "query"
		if typeID != "all" {
			parameter = "query." + typeID
		}
		return "https://api.crossref.org/works?rows=20&select=DOI,title,author,URL,published,type&" + parameter + "=" + q, nil
	default:
		return "", fmt.Errorf("API adapter %q has no request builder", adapterID)
	}
}

func Search(ctx context.Context, adapterID, typeID, query string) (Result, error) {
	adapter, _ := Lookup(adapterID)
	rawURL, err := BuildURL(adapterID, typeID, query)
	if err != nil {
		return Result{}, err
	}
	response, err := fetch.Get(ctx, rawURL, fetch.Options{Timeout: 30 * time.Second, MaxBytes: 20 << 20, Headers: map[string]string{"Accept": "application/json", "User-Agent": "srch/0.1 (local terminal search client)"}})
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{}, fmt.Errorf("%s returned %s", adapter.Name, response.Status)
	}
	formatted, err := fetch.Format(response, "json")
	if err != nil {
		return Result{}, err
	}
	items, err := decodeItems(adapterID, typeID, response.Body)
	if err != nil {
		return Result{}, fmt.Errorf("decode %s response: %w", adapter.Name, err)
	}
	return Result{Adapter: adapter, Type: lookupType(adapter, typeID), URL: response.URL, Status: response.Status, Duration: response.Duration, Items: items, Raw: formatted}, nil
}

func hasType(adapter Adapter, id string) bool {
	_, ok := findType(adapter, id)
	return ok
}

func lookupType(adapter Adapter, id string) SearchType {
	value, _ := findType(adapter, id)
	return value
}

func findType(adapter Adapter, id string) (SearchType, bool) {
	for _, searchType := range adapter.Types {
		if searchType.ID == id {
			return searchType, true
		}
	}
	return SearchType{}, false
}

func decodeItems(adapterID, typeID string, body []byte) ([]Item, error) {
	switch adapterID {
	case "musicbrainz":
		return decodeMusicBrainz(typeID, body)
	case "internet-archive":
		return decodeInternetArchive(body)
	case "open-library":
		return decodeOpenLibrary(body)
	case "crossref":
		return decodeCrossref(body)
	default:
		return nil, fmt.Errorf("unsupported adapter %q", adapterID)
	}
}

func decodeMusicBrainz(typeID string, body []byte) ([]Item, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	key := map[string]string{"artist": "artists", "release": "releases", "recording": "recordings", "work": "works", "label": "labels"}[typeID]
	var rows []struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Title          string `json:"title"`
		Disambiguation string `json:"disambiguation"`
		Country        string `json:"country"`
		Date           string `json:"date"`
	}
	if err := json.Unmarshal(payload[key], &rows); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		title := row.Name
		if title == "" {
			title = row.Title
		}
		subtitle := strings.TrimSpace(strings.Join([]string{row.Country, row.Date}, " "))
		items = append(items, Item{Title: title, Subtitle: subtitle, Description: row.Disambiguation, URL: "https://musicbrainz.org/" + typeID + "/" + row.ID})
	}
	return items, nil
}

func decodeInternetArchive(body []byte) ([]Item, error) {
	var payload struct {
		Response struct {
			Docs []struct {
				Identifier  string `json:"identifier"`
				Title       any    `json:"title"`
				Description any    `json:"description"`
				MediaType   string `json:"mediatype"`
				Date        string `json:"date"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(payload.Response.Docs))
	for _, row := range payload.Response.Docs {
		items = append(items, Item{Title: stringValue(row.Title), Subtitle: strings.TrimSpace(row.MediaType + " " + row.Date), Description: stringValue(row.Description), URL: "https://archive.org/details/" + row.Identifier})
	}
	return items, nil
}

func decodeOpenLibrary(body []byte) ([]Item, error) {
	var payload struct {
		Docs []struct {
			Key              string   `json:"key"`
			Title            string   `json:"title"`
			AuthorNames      []string `json:"author_name"`
			FirstPublishYear int      `json:"first_publish_year"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(payload.Docs))
	for _, row := range payload.Docs {
		subtitle := strings.Join(row.AuthorNames, ", ")
		if row.FirstPublishYear != 0 {
			subtitle = strings.TrimSpace(fmt.Sprintf("%s · %d", subtitle, row.FirstPublishYear))
		}
		items = append(items, Item{Title: row.Title, Subtitle: subtitle, URL: "https://openlibrary.org" + row.Key})
	}
	return items, nil
}

func decodeCrossref(body []byte) ([]Item, error) {
	var payload struct {
		Message struct {
			Items []struct {
				DOI    string   `json:"DOI"`
				Title  []string `json:"title"`
				URL    string   `json:"URL"`
				Type   string   `json:"type"`
				Author []struct {
					Given  string `json:"given"`
					Family string `json:"family"`
				} `json:"author"`
			} `json:"items"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(payload.Message.Items))
	for _, row := range payload.Message.Items {
		authors := make([]string, 0, len(row.Author))
		for _, author := range row.Author {
			authors = append(authors, strings.TrimSpace(author.Given+" "+author.Family))
		}
		title := row.DOI
		if len(row.Title) > 0 {
			title = row.Title[0]
		}
		items = append(items, Item{Title: title, Subtitle: strings.Join(authors, ", "), Description: row.Type, URL: row.URL})
	}
	return items, nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " · ")
	default:
		return ""
	}
}
