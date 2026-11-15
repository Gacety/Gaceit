//go:build js

package main

func defaultDataDir(bankSlug string) (string, error) {
	if bankSlug != "" {
		return "web/" + bankSlug, nil
	}
	return "web/clean-v1", nil
}
