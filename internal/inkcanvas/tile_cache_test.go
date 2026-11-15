package inkcanvas

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

func tileFixture(t *testing.T, dark bool, zoom float64) *Editor {
	t.Helper()
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(dark, 1)
	var strokes []ink.Stroke
	for i := 0; i < 32; i++ {
		x, y := float32(i%8)*85-290, float32(i/8)*95-100
		s := ink.Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: x, Y: y, Radius: 3}, {X: x + 80, Y: y + 65, Radius: 2}, {X: x + 160, Y: y, Radius: 3}}}
		if i%4 == 0 {
			s.Tool = ink.Highlighter
			s.Color = [4]float32{1, .8, .1, .45}
		}
		if i%7 == 0 {
			s.Erasures = [][]ink.Point{{{X: x + 35, Y: y + 20, Radius: 8}, {X: x + 45, Y: y + 45, Radius: 8}}}
		}
		strokes = append(strokes, s)
	}
	ed.Load(Document{Version: 1, Zoom: zoom, Strokes: strokes})
	return ed
}
func TestTiledPanMatchesFullRedrawAtSeams(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, zoom := range []float64{.5, 1, 1.375, 2} {
			ed := tileFixture(t, dark, zoom)
			tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
			for _, off := range [][2]float64{{-128, -70}, {-300, -170}, {140, 90}, {-700, 60}, {-128, -70}} {
				ed.offsetX, ed.offsetY = off[0], off[1]
				tt.Frame()
				ref, err := New()
				if err != nil {
					t.Fatal(err)
				}
				ref.Appearance(dark, 1)
				ref.Load(ed.Snapshot())
				full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 340, 260, 1)
				if !bytes.Equal(tt.Image().Pix, full.Pix) {
					t.Fatalf("tile seam/ink mismatch dark=%v zoom=%v offset=%v", dark, zoom, off)
				}
			}
			if ed.tiles.hits == 0 || ed.tiles.misses == 0 {
				t.Fatal("tiles not reused")
			}
		}
	}
}
func TestTiledRetinaPanMatchesFullRedraw(t *testing.T) {
	ed := tileFixture(t, true, 1.375)
	ed.Appearance(true, 2)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
	ed.offsetX = -128
	ed.offsetY = -70
	tt.Frame()
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(true, 2)
	ref.Load(ed.Snapshot())
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 340, 260, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		count, delta := 0, 0
		for i, v := range tt.Image().Pix {
			d := int(v) - int(full.Pix[i])
			if d < 0 {
				d = -d
			}
			if d > 0 {
				count++
			}
			delta = max(delta, d)
		}
		// At 2×, local raster coordinates and final downsampling can round an
		// antialiased channel by one 8-bit unit. Keep the same strict area bound
		// as real Metal; larger differences indicate missing or duplicated ink.
		if delta > 1 || count > len(full.Pix)/1000 {
			t.Fatal("Retina tile seam/ink mismatch", count, delta)
		}
	}
}

func TestTileInvalidationKeepsUnaffectedBlocks(t *testing.T) {
	ed := tileFixture(t, false, 1)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
	ed.offsetX = -150
	tt.Frame()
	ed.tiles.sync(ed, 1)
	original := map[tileKey]*ui.Canvas{}
	for k, v := range ed.tiles.entries {
		original[k] = v.canvas
	}
	var center ui.Point
	for k := range original {
		b := ed.tiles.worldBounds(k)
		center = ui.Point{X: (b[0] + b[2]) / 2, Y: (b[1] + b[3]) / 2}
		break
	}
	s := ink.Stroke{Points: []ink.Point{{X: center.X, Y: center.Y, Radius: 2}, {X: center.X + 10, Y: center.Y + 10, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}}
	ed.strokes = append(ed.strokes, s)
	ed.tiles.sync(ed, 1)
	affected, kept := 0, 0
	for k, v := range original {
		_, ok := ed.tiles.entries[k]
		if ink.BoundsOverlap(ed.tiles.worldBounds(k), ink.StrokeBounds(s)) {
			if ok {
				t.Fatal("edited tile retained")
			}
			affected++
		} else {
			if !ok || ed.tiles.entries[k].canvas != v {
				t.Fatal("unrelated tile invalidated")
			}
			kept++
		}
	}
	if affected == 0 || kept == 0 {
		t.Fatal("fixture did not exercise both regions")
	}
	ed.ResetSurfaceCaches()
	if len(ed.tiles.entries) != 0 {
		t.Fatal("surface reset retained tiles")
	}
}
func TestTileCacheBudgetAndEvictedReturn(t *testing.T) {
	ed := tileFixture(t, true, 1)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 120, 100)
	for i := 1; i < 70; i++ {
		ed.offsetX = float64(-i * 256)
		tt.Frame()
		if len(ed.tiles.entries) > inkTileLimit {
			t.Fatal("tile budget exceeded")
		}
	}
	if ed.tiles.evictions == 0 {
		t.Fatal("no eviction exercised")
	}
	ed.offsetX = -128
	ed.offsetY = -70
	tt.Frame()
	ref := tileFixture(t, true, 1)
	ref.offsetX = ed.offsetX
	ref.offsetY = ed.offsetY
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 120, 100, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("return to evicted region lost ink")
	}
}

func TestTileDeletionAndUndoInvalidateOnlyRemovedStroke(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		x := float32(i*1024 + 80)
		ed.strokes = append(ed.strokes, ink.Stroke{Points: []ink.Point{{X: x, Y: 80, Radius: 2}, {X: x + 30, Y: 120, Radius: 2}}})
	}
	ed.tiles.sync(ed, 1)
	for i := 0; i < 4; i++ {
		ed.tiles.entries[tileKey{int64(i * 4), 0}] = inkTile{canvas: ui.NewCanvas(inkTilePixels, inkTilePixels)}
	}
	removed := ed.strokes[0]
	ed.strokes = ed.strokes[1:]
	ed.tiles.sync(ed, 1)
	if len(ed.tiles.entries) != 3 {
		t.Fatal("deleting first stroke invalidated unrelated tiles")
	}
	ed.strokes = append([]ink.Stroke{removed}, ed.strokes...)
	ed.tiles.sync(ed, 1)
	if len(ed.tiles.entries) != 3 {
		t.Fatal("restoring first stroke invalidated unrelated tiles")
	}
}
