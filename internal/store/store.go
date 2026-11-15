// Package store persists a complete browser-compatible learning snapshot atomically.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Snapshot map[string]string

type Store struct {
	mu   sync.Mutex
	Path string
}

func Validate(data Snapshot) error {
	for key, value := range data {
		if !strings.HasPrefix(key, "408-") {
			return errors.New("unsupported storage key")
		}
		if !json.Valid([]byte(value)) && key != "408-study-location-v1" && key != "408-theme-v1" {
			return errors.New("invalid snapshot value")
		}
	}
	return nil
}

func (s *Store) Load() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	var result Snapshot
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if err = Validate(result); err != nil {
		return nil, err
	}
	if result == nil {
		result = Snapshot{}
	}
	return result, nil
}

func (s *Store) Save(snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := Validate(snapshot); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".progress-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.Path)
}
