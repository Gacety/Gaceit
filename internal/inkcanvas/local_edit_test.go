package inkcanvas

import (
	"study408/internal/ink"
	"testing"
)

func TestLocalEditRepairMatchesFullScene(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		ed, tt := transitionEditor(t, transparent)
		tt.Frame()
		before := ed.strokes[0]
		after := before
		after.Translation = [2]float32{35, 20}
		edit := strokeEdit{changed: []strokeChange{{0, before, after}}}
		for _, reverse := range []bool{false, true, false} {
			ed.applyEdit(edit, reverse)
			if ed.dryInvalid || len(ed.dirtyRegions) == 0 {
				t.Fatal("local edit invalidated full surface")
			}
			tt.Frame()
			assertTransitionPixels(t, ed, tt, ed.Snapshot(), 320, 240)
		}
		ed.erasePath = []ink.Point{{X: 25, Y: 33, Radius: 8}, {X: 70, Y: 50, Radius: 8}}
		ed.commitPixelErase()
		if !ed.eraseChanged || ed.dryInvalid {
			t.Fatal("pixel eraser did not use local repair")
		}
		ed.erasePath = nil
		tt.Frame()
		assertTransitionPixels(t, ed, tt, ed.Snapshot(), 320, 240)
	}
}

func TestTilePhaseChangeRetainsWorldGeometry(t *testing.T) {
	ed := tileFixture(t, false, 1)
	defer ed.ReleaseCaches()
	ed.tiles.syncFrame(ed, ink.Camera{Zoom: 1, PixelRatio: 1}, false)
	first := &ed.tiles.sources[0]
	ed.tiles.syncFrame(ed, ink.Camera{Zoom: 1, PixelRatio: 1, Offset: [2]float32{.25, .5}}, false)
	if first != &ed.tiles.sources[0] {
		t.Fatal("phase-only change rebuilt world geometry")
	}
}

func TestToolCancelDoesNotInvalidateCompletedInk(t *testing.T) {
	ed, tt := transitionEditor(t, true)
	tt.Frame()
	ed.Cancel()
	if ed.dryInvalid {
		t.Fatal("tool cancellation invalidated completed ink")
	}
	tt.Frame()
	assertTransitionPixels(t, ed, tt, ed.Snapshot(), 320, 240)
}
