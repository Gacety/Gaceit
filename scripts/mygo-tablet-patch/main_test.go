package main

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTabletPatchUsesEventPosition(t *testing.T) {
	original := "package darwin\n" + strings.ReplaceAll(oldTabletPoint, "\n ", "\n\t") + "\n// unrelated\n"
	patched, err := patchTabletSource(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(patched, `sel("mouseLocation")`) || !strings.Contains(patched, "x, y := s.location(ev)") {
		t.Fatal("patch did not preserve event coordinates")
	}
	if !strings.HasSuffix(patched, "\n// unrelated\n") {
		t.Fatal("patch changed unrelated source")
	}
	if _, err := patchTabletSource(patched); err == nil {
		t.Fatal("changed upstream silently accepted")
	}
}

func TestIdlePatchIdempotenceAndDrift(t *testing.T) {
	for _, patch := range idlePatches {
		original := "// untouched prefix\n" + patch.old + "\n// untouched suffix"
		changed, err := applySourcePatch(original, patch)
		if err != nil {
			t.Fatal(patch.path, err)
		}
		twice, err := applySourcePatch(changed, patch)
		if err != nil || twice != changed {
			t.Fatal("idle patch not idempotent", patch.path, err)
		}
		if _, err := applySourcePatch("// changed upstream", patch); err == nil {
			t.Fatal("patch silently accepted drift", patch.path)
		}
	}
}

func TestNativePatchesRemainIdempotentAfterGofmt(t *testing.T) {
	sources := map[string]string{}
	for _, p := range idlePatches {
		if _, ok := sources[p.path]; !ok {
			b, err := os.ReadFile(filepath.Join("../..", p.path))
			if err != nil {
				t.Fatal(err)
			}
			b, err = format.Source(b)
			if err != nil {
				t.Fatal(err)
			}
			sources[p.path] = string(b)
		}
		before := sources[p.path]
		after, err := applySourcePatch(before, p)
		if err != nil || after != before {
			t.Fatalf("patch changes gofmt'd installed source %s: %v", p.path, err)
		}
	}
}
