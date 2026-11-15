//go:build js

package main

func defaultDataDir(bankSlug string) (string, error) {
	if bankSlug != "" {
		return "web/" + bankSlug, nil
	}
	return "web/default", nil
}
