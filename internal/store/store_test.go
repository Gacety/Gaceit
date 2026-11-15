package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTripAndFailedWritePreservesRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "progress.json")
	s := &Store{Path: path}
	empty, err := s.Load()
	if err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	data := Snapshot{"408-study-glass-v1": `{"version":1,"records":{"original-id":{"notes":"保留笔记","attempts":[],"wrong":false}}}`, "408-theme-v1": "dark", "408-study-location-v1": "original-id"}
	if err := s.Save(data); err != nil {
		t.Fatal(err)
	}
	loaded, err := (&Store{Path: path}).Load()
	if err != nil || !reflect.DeepEqual(loaded, data) {
		t.Fatal(loaded, err)
	}
	if err := s.Save(Snapshot{"foreign-key": "bad"}); err == nil {
		t.Fatal("invalid write accepted")
	}
	loaded, _ = s.Load()
	if !reflect.DeepEqual(loaded, data) {
		t.Fatal("failed write modified records")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions", info.Mode())
	}
}

func TestCorruptionIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	os.WriteFile(path, []byte("broken"), 0600)
	if _, err := (&Store{Path: path}).Load(); err == nil {
		t.Fatal("corruption silently accepted")
	}
}
