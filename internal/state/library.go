package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"srch/internal/domain"
)

type Profile struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Request domain.SearchRequest `json:"request"`
}

type Library struct {
	ProfilePath   string
	FavouritePath string
}

func (l Library) Profiles() ([]Profile, error) {
	var profiles []Profile
	if err := readJSONFile(l.ProfilePath, &profiles); err != nil {
		return nil, err
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func (l Library) SaveProfile(profile Profile) error {
	profiles, err := l.Profiles()
	if err != nil {
		return err
	}
	found := false
	for index := range profiles {
		if profiles[index].ID == profile.ID {
			profiles[index] = profile
			found = true
			break
		}
	}
	if !found {
		profiles = append(profiles, profile)
	}
	return writeJSONFile(l.ProfilePath, profiles)
}

func (l Library) DeleteProfile(id string) error {
	profiles, err := l.Profiles()
	if err != nil {
		return err
	}
	filtered := profiles[:0]
	for _, profile := range profiles {
		if profile.ID != id {
			filtered = append(filtered, profile)
		}
	}
	return writeJSONFile(l.ProfilePath, filtered)
}

func (l Library) Profile(id string) (Profile, bool, error) {
	profiles, err := l.Profiles()
	if err != nil {
		return Profile{}, false, err
	}
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true, nil
		}
	}
	return Profile{}, false, nil
}

func (l Library) Favourites() ([]string, error) {
	var values []string
	if err := readJSONFile(l.FavouritePath, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (l Library) AddFavourite(id string) error {
	values, err := l.Favourites()
	if err != nil {
		return err
	}
	for _, value := range values {
		if value == id {
			return nil
		}
	}
	return writeJSONFile(l.FavouritePath, append(values, id))
}

func (l Library) RemoveFavourite(id string) error {
	values, err := l.Favourites()
	if err != nil {
		return err
	}
	filtered := values[:0]
	for _, value := range values {
		if value != id {
			filtered = append(filtered, value)
		}
	}
	return writeJSONFile(l.FavouritePath, filtered)
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}
	return nil
}
