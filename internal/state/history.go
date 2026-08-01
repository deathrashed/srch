package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"srch/internal/domain"
)

type HistoryStore struct {
	Path  string
	Limit int
}

func (s HistoryStore) List() ([]domain.HistoryEntry, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read history: %w", err)
	}
	var entries []domain.HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse history: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	return entries, nil
}

func (s HistoryStore) Add(request domain.SearchRequest) error {
	if request.Private || request.Query == "" {
		return nil
	}
	entries, err := s.List()
	if err != nil {
		return err
	}
	if len(entries) > 0 && entries[0].Query == request.Query && equalStrings(entries[0].EngineIDs, request.EngineIDs) {
		return nil
	}
	entry := domain.HistoryEntry{
		ID:        strconv.FormatInt(time.Now().UnixNano(), 36),
		Query:     request.Query,
		EngineIDs: append([]string(nil), request.EngineIDs...),
		PresetID:  request.PresetID,
		CreatedAt: time.Now().UTC(),
	}
	entries = append([]domain.HistoryEntry{entry}, entries...)
	limit := s.Limit
	if limit <= 0 {
		limit = 1000
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return s.write(entries)
}

func (s HistoryStore) Clear() error {
	return s.write(nil)
}

func (s HistoryStore) Remove(id string) error {
	entries, err := s.List()
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, entry := range entries {
		if entry.ID != id {
			filtered = append(filtered, entry)
		}
	}
	return s.write(filtered)
}

func (s HistoryStore) write(entries []domain.HistoryEntry) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode history: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "history-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary history: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write history: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync history: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close history: %w", err)
	}
	if err := os.Rename(name, s.Path); err != nil {
		return fmt.Errorf("replace history: %w", err)
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
