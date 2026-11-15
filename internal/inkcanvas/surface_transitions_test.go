package inkcanvas

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

func transitionEditor(t *testing.T, transparent bool) (*Editor, *ui.Tester) {
	t.Helper()
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	ed.SetTransparentInk(transparent)
	var strokes []ink.Stroke
	for i := 0; i < 48; i++ {
		x, y := float32(20+i%12*45), float32(30+i/12*40)
		strokes = append(strokes, ink.Stroke{Color: [4]float32{0, 0, 0, 1}, Smooth: true, Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 18, Y: y + 12, Radius: 2}}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 320, 240)
	t.Cleanup(ed.ReleaseCaches)
	return ed, tt
}

func assertTransitionPixels(t *testing.T, ed *Editor, tt *ui.Tester, doc Document, w, h int) {
	t.Helper()
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer ref.ReleaseCaches()
	ref.Appearance(ed.dark, ed.ratio)
	ref.SetTransparentInk(ed.transparentInk)
	ref.Load(doc)
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, w, h, ed.ratio)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		count, delta := 0, 0
		for i, v := range tt.Image().Pix {
			d := int(v) - int(full.Pix[i])
			if d < 0 {
				d = -d
			}
			if d != 0 {
				count++
			}
			delta = max(delta, d)
		}
		t.Fatalf("transition differs from complete vector reconstruction: %d channels, max delta %d", count, delta)
	}
}

func TestSelectionAfterPanUsesLatestCompleteView(t *testing.T) {
	ed, tt := transitionEditor(t, true)
	ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 180})
	tt.Frame()
	doc := ed.Snapshot()
	// Omit selection chrome so the comparison isolates the ink scene.
	ed.selection.ids = makeRange(0, len(ed.strokes))
	ed.updateSelectionBounds()
	ed.selection.before = append([]ink.Stroke(nil), ed.strokes...)
	ed.selection.moving = true
	tt.Frame()
	if !ed.selection.preview {
		t.Fatal("selection preview was not prepared")
	}
	assertTransitionPixels(t, ed, tt, doc, 320, 240)
}

func TestSelectionDragRevealsPreviouslyOffscreenInk(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint(all), func(t *testing.T) {
			ed, tt := transitionEditor(t, true)
			first := 7
			if all {
				first = 0
			}
			ed.selection.ids = makeRange(first, len(ed.strokes))
			ed.updateSelectionBounds()
			ed.selection.before = append([]ink.Stroke(nil), ed.strokes...)
			ed.selection.moving = true
			ed.selection.delta = ui.Point{X: -120}
			tt.Frame()
			doc := ed.Snapshot()
			for _, id := range ed.selection.ids {
				doc.Strokes[id] = translatedStroke(doc.Strokes[id], ed.selection.delta)
			}
			assertTransitionPixels(t, ed, tt, doc, 320, 240)
			ed.selection.commitPending = true
			tt.Frame()
			assertTransitionPixels(t, ed, tt, doc, 320, 240)
		})
	}
}

func TestReleasedSelectionSurvivesViewInvalidation(t *testing.T) {
	for _, change := range []string{"resize", "theme", "reset", "recover"} {
		t.Run(change, func(t *testing.T) {
			ed, tt := transitionEditor(t, true)
			ed.selection.ids = []int{0, 1, 2}
			ed.updateSelectionBounds()
			ed.selection.before = append([]ink.Stroke(nil), ed.strokes...)
			ed.selection.moving = true
			tt.Frame()
			before := ed.Snapshot()
			delta := ui.Point{X: 15, Y: 9}
			ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: delta.X, Y: delta.Y})
			if !ed.selection.commitPending {
				t.Fatal("release was not queued")
			}
			w, h := 320, 240
			switch change {
			case "resize":
				w = 360
				tt.SetSize(w, h)
			case "theme":
				ed.Appearance(true, 1)
			case "reset":
				ed.ResetSurfaceCaches()
			case "recover":
				ed.recoverIdleSurface(nil, ui.Rect{W: 320, H: 240}, false)
			}
			tt.Frame()
			if ed.Busy() || ed.selection.commitPending {
				t.Fatal("view change lost pointer-up and left selection stuck")
			}
			for i := 0; i < 3; i++ {
				if ed.strokes[i].Translation != ([2]float32{delta.X, delta.Y}) {
					t.Fatal("released move was lost", i)
				}
			}
			assertTransitionPixels(t, ed, tt, ed.Snapshot(), w, h)
			ed.Undo()
			tt.Frame()
			for i, stroke := range ed.strokes {
				if !reflect.DeepEqual(stroke.Points, before.Strokes[i].Points) || stroke.Translation != before.Strokes[i].Translation {
					t.Fatal("view change broke move undo")
				}
			}
		})
	}
}

func TestThemeChangePreservesActiveGestureInk(t *testing.T) {
	ed, _ := transitionEditor(t, true)
	ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, Pressure: .7, X: 40, Y: 40})
	ed.Appearance(true, 1)
	if ed.current.Color != ([4]float32{1, 1, 1, 1}) {
		t.Fatal("active neutral pen became invisible after theme change")
	}
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, Pointer: ui.PointerPen, Canceled: true})
	if len(ed.strokes) != 49 || ed.strokes[48].Points[0].X != 40 {
		t.Fatal("theme change discarded pen samples")
	}
	ed.selection.before = append([]ink.Stroke(nil), ed.strokes...)
	ed.selection.moving = true
	ed.Appearance(false, 1)
	ed.Cancel()
	for _, s := range ed.strokes {
		if s.Color != ([4]float32{0, 0, 0, 1}) {
			t.Fatal("canceled gesture restored ink from the previous theme")
		}
	}
}

func TestInkTransitionSequencesRemainComplete(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		t.Run(fmt.Sprint(transparent), func(t *testing.T) {
			ed, tt := transitionEditor(t, transparent)
			for step := 0; step < 18; step++ {
				switch step % 6 {
				case 0:
					ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 97, DY: -13})
				case 1:
					ed.SetTool("pen")
					for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 40, Y: 30}, {Kind: ui.InputPointerMove, X: 90, Y: 45}, {Kind: ui.InputPointerUp, X: 140, Y: 50}} {
						e.Pointer, e.Pressure = ui.PointerPen, .7
						ed.Input(e)
					}
				case 2:
					ed.ZoomBy(1.25)
					ed.zoomChangedAt = time.Now().Add(-time.Second)
					tt.Frame() // interrupt an unfinished scale bake
					ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: -19, DY: 7})
				case 3:
					ed.RequestRecompose()
					ed.Undo()
				case 4:
					ed.Redo()
					ed.ResetSurfaceCaches()
				case 5:
					ed.Appearance(!ed.dark, 1)
				}
				tt.Frame()
				for i := 0; i < 30 && (ed.RebuildingNavigation() || ed.recompose); i++ {
					tt.Frame()
				}
				if ed.RebuildingNavigation() || ed.recompose {
					t.Fatal("transition did not finish", step)
				}
				assertTransitionPixels(t, ed, tt, ed.Snapshot(), 320, 240)
			}
		})
	}
}

func TestPanPenTransitionsKeepIncrementalBase(t *testing.T) {
	ed, tt := transitionEditor(t, true)
	// Start on a fractional physical grid, then translate by whole pixels.
	// This checks tile phase/copy correctness without comparing a deliberately
	// interpolated fractional bitmap pan to freshly rasterized antialiasing.
	ed.offsetX, ed.offsetY = -.25, -.5
	ed.ResetSurfaceCaches()
	tt.Frame()
	original := ed.Snapshot()
	for i := 0; i < 8; i++ {
		ed.offsetX = -80.25 - float64((i%2)*80)
		ed.offsetY = -10.5
		tt.Frame()
		for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 40, Y: 30}, {Kind: ui.InputPointerMove, X: 90, Y: 45}, {Kind: ui.InputPointerUp, X: 140, Y: 50}} {
			e.Pointer, e.Pressure = ui.PointerPen, .7
			ed.Input(e)
		}
		tt.Frame()
		if !ed.dryReusesPan {
			t.Fatal("pen discarded the completed pan view and rebuilt all vectors", i)
		}
		assertTransitionPixels(t, ed, tt, ed.Snapshot(), 320, 240)
	}
	if !reflect.DeepEqual(original.Strokes, ed.strokes[:len(original.Strokes)]) {
		t.Fatal("repeated view handoffs changed original ink")
	}
}

func TestRepeatedLocalRepairsCheckpointWithoutLosingInk(t *testing.T) {
	ed, tt := transitionEditor(t, true)
	checkpoint := false
	previousPatches := uint8(0)
	for i := 0; i < 12; i++ {
		old := ink.StrokeBounds(ed.strokes[0])
		ed.strokes[0] = translatedStroke(ed.strokes[0], ui.Point{X: 1})
		ed.dirtyRegions = append(ed.dirtyRegions, old, ink.StrokeBounds(ed.strokes[0]))
		tt.Frame()
		assertTransitionPixels(t, ed, tt, ed.Snapshot(), 320, 240)
		patches := ed.tiles.entries[tileKey{0, 0}].patches
		if patches < previousPatches {
			checkpoint = true
		}
		previousPatches = patches
	}
	if !checkpoint {
		t.Fatal("repeated tile patches never discarded their old raster dependency chain")
	}
}
