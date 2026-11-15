package inkcanvas

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

func TestTransparentInkEraseRevealsUnderlyingUI(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	ed.SetTransparentInk(true)
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: []ink.Stroke{
		{Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: 20, Y: 70, Radius: 5}, {X: 220, Y: 70, Radius: 5}}},
		{Tool: ink.Highlighter, Color: [4]float32{1, .8, .1, .45}, Points: []ink.Point{{X: 20, Y: 140, Radius: 7}, {X: 220, Y: 140, Radius: 7}}},
	}})
	view := func(c *ui.Context) { ui.Box(c).Fill().Background(ui.Hex("#3A78BC")).Children(func() { ed.Canvas(c) }) }
	tt := ui.NewTester(view, 260, 220)
	if got := tt.Image().RGBAAt(10, 10); got.R != 0x3A || got.G != 0x78 || got.B != 0xBC {
		t.Fatal("ink layer painted paper over native UI", got)
	}
	ed.SetTool("eraser")
	ed.SetEraserRegion(true)
	ed.SetWidth(30)
	ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 120, Y: 70})
	tt.Frame()
	if got := tt.Image().RGBAAt(120, 70); got.R != 0x3A || got.G != 0x78 || got.B != 0xBC {
		t.Fatal("eraser preview failed to reveal UI", got)
	}
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, Pointer: ui.PointerPen, X: 120, Y: 70})
	tt.Frame()
	got := append([]byte(nil), tt.Image().Pix...)
	ed.ResetSurfaceCaches()
	tt.Frame()
	if !bytes.Equal(got, tt.Image().Pix) {
		t.Fatal("transparent erasure differs from full redraw")
	}
	// Whole-stroke deletion uses dirty tile patches and must clear alpha too.
	ed.SetEraserRegion(false)
	ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 80, Y: 140})
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, Pointer: ui.PointerPen, X: 80, Y: 140})
	tt.Frame()
	got = append([]byte(nil), tt.Image().Pix...)
	ed.ResetSurfaceCaches()
	tt.Frame()
	if !bytes.Equal(got, tt.Image().Pix) {
		t.Fatal("transparent tile deletion retained ghost ink")
	}
}

func TestTransparentLassoPreviewPreservesOrderAndAlpha(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	ed.SetTransparentInk(true)
	strokes := []ink.Stroke{
		{Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: 20, Y: 50, Radius: 4}, {X: 210, Y: 100, Radius: 4}}},
		{Color: [4]float32{1, 0, 0, .5}, Points: []ink.Point{{X: 20, Y: 80, Radius: 6}, {X: 210, Y: 80, Radius: 6}}},
		{Tool: ink.Highlighter, Color: [4]float32{0, 1, 0, .5}, Points: []ink.Point{{X: 20, Y: 100, Radius: 5}, {X: 210, Y: 50, Radius: 5}}},
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	view := func(c *ui.Context) { ui.Box(c).Fill().Background(ui.Hex("#3A78BC")).Children(func() { ed.Canvas(c) }) }
	tt := ui.NewTester(view, 260, 220)
	ed.SetTool("lasso")
	ed.selection.ids = []int{1}
	ed.updateSelectionBounds()
	ed.selection.moving = true
	ed.selection.before = append([]ink.Stroke(nil), ed.strokes...)
	ed.selection.delta = ui.Point{X: 18, Y: 16}
	ed.paletteMode = 0
	tt.Frame()
	moved := append([]ink.Stroke(nil), strokes...)
	moved[1] = translatedStroke(moved[1], ed.selection.delta)
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.SetTransparentInk(true)
	ref.Load(Document{Version: 1, Zoom: 1, Strokes: moved})
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Background(ui.Hex("#3A78BC")).Children(func() { ref.Canvas(c) }) }, 260, 220, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		count, maxDelta := 0, 0
		for i, v := range tt.Image().Pix {
			d := int(v) - int(full.Pix[i])
			if d < 0 {
				d = -d
			}
			if d > 0 {
				count++
			}
			maxDelta = max(maxDelta, d)
		}
		// Compositing a cached alpha run has an extra 8-bit rounding step.
		if maxDelta > 1 || count > len(full.Pix)/100 {
			t.Fatal("transparent selection left old ink or changed order", count, maxDelta)
		}
	}
}
