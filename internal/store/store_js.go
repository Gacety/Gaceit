//go:build js

package store

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

func (s *Store) storageKey() string {
	return "study408:snapshot:" + s.Path
}

func (s *Store) Load() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var raw string
	var readErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				readErr = fmt.Errorf("localStorage read failed: %v", r)
			}
		}()
		storage := js.Global().Get("localStorage")
		if storage.IsUndefined() || storage.IsNull() {
			return
		}
		v := storage.Call("getItem", s.storageKey())
		if v.Type() == js.TypeString {
			raw = v.String()
		}
	}()
	if readErr != nil {
		return nil, readErr
	}
	if raw == "" {
		return Snapshot{}, nil
	}
	var result Snapshot
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	if err := Validate(result); err != nil {
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
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	var writeErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				writeErr = fmt.Errorf("localStorage write failed: %v", r)
			}
		}()
		storage := js.Global().Get("localStorage")
		if storage.IsUndefined() || storage.IsNull() {
			return
		}
		storage.Call("setItem", s.storageKey(), string(data))
	}()
	return writeErr
}
