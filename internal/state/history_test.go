package state_test

import (
	"path/filepath"
	"testing"

	"srch/internal/domain"
	"srch/internal/state"
)

func TestHistoryDeduplicatesAndHonorsPrivate(t *testing.T) {
	store := state.HistoryStore{Path: filepath.Join(t.TempDir(), "history.json"), Limit: 10}
	request := domain.SearchRequest{Query: "bubble tea", EngineIDs: []string{"google"}}
	if err := store.Add(request); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(request); err != nil {
		t.Fatal(err)
	}
	request.Private = true
	request.Query = "secret"
	if err := store.Add(request); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Query != "bubble tea" {
		t.Fatalf("unexpected history: %#v", entries)
	}
}
