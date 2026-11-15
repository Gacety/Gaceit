package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

func TestTilePrefetchCompletesWithinExistingBudget(t *testing.T) {
	ed := tileFixture(t, false, 1)
	defer ed.ReleaseCaches()
	ed.SetTransparentInk(true)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
	for i := 0; i < 80; i++ {
		tt.Frame()
		if len(ed.tiles.entries) > inkTileLimit {
			t.Fatal("tile budget exceeded")
		}
	}
	if ed.tiles.prefetched == 0 {
		t.Fatal("adjacent tiles not warmed")
	}
	assertTransitionPixels(t, ed, tt, ed.Snapshot(), 340, 260)
}
func TestSparseAppendCopiesOnlyOccupiedPixels(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer ed.ReleaseCaches()
	ed.SetTransparentInk(true)
	ed.Load(Document{Version: 1, Zoom: 1})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 800, 600)
	tt.Frame()
	ed.strokes = append(ed.strokes, ink.Stroke{Points: []ink.Point{{X: 50, Y: 50, Radius: 2}, {X: 70, Y: 70, Radius: 2}}, Smooth: true, Color: [4]float32{0, 0, 0, 1}})
	ed.editRevision++
	tt.Frame()
	if ed.frameTrace.copiedPixels == 0 || ed.frameTrace.copiedPixels >= 800*600 {
		t.Fatal("sparse append copied full viewport", ed.frameTrace.copiedPixels)
	}
	assertTransitionPixels(t, ed, tt, ed.Snapshot(), 800, 600)
}
