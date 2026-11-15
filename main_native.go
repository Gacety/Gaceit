//go:build !js

package main

import (
	"os"
	"path/filepath"
)

func defaultDataDir(bankSlug string) (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if bankSlug != "" {
		return filepath.Join(root, "408研习", "banks", bankSlug), nil
	}
	return filepath.Join(root, "408研习"), nil
}
