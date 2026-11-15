// Package store persists a complete browser-compatible learning snapshot atomically.
package store

import (
	"encoding/json"
	"errors"
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
