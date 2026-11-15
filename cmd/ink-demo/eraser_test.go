package main

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

func eraserFixture() ink.Stroke {
	return ink.Stroke{Points: []ink.Point{{X: 10, Y: 40, Radius: 3}, {X: 50, Y: 40, Radius: 3}, {X: 90, Y: 40, Radius: 3}}, Color: [4]float32{0, 0, 0, 1}}
}
func TestStrokeEraserUndoAndCancellation(t *testing.T) {
	a := &app{paletteMode: 1, eraserWidth: 8, strokes: []ink.Stroke{eraserFixture()}}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Y: 40})
	a.processStrokeErase()
	if len(a.strokes) != 0 {
		t.Fatal("whole stroke not removed")
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 50, Y: 40})
	a.undoAction()
	if len(a.strokes) != 1 {
		t.Fatal("undo did not restore stroke")
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Y: 40})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, Canceled: true})
	if len(a.strokes) != 1 {
		t.Fatal("cancel did not restore stroke")
	}
}
func TestPixelEraserCopyOnWriteUndoAndLaterWriting(t *testing.T) {
	a := &app{paletteMode: 1, eraserSelect: 1, eraserWidth: 8, strokes: []ink.Stroke{eraserFixture()}}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 30, Y: 40})
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: 70, Y: 40})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 70, Y: 40})
	if len(a.strokes) != 1 || len(a.strokes[0].Erasures) != 1 || len(a.strokes[0].Erasures[0]) != 2 {
		t.Fatal("pixel eraser didn't retain stroke with one sweep")
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 80, Y: 40})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, Canceled: true})
	if len(a.strokes[0].Erasures) != 1 {
		t.Fatal("cancel mutated prior erasure")
	}
	a.undoAction()
	if len(a.strokes[0].Erasures) != 0 {
		t.Fatal("undo snapshot's erasures mutated")
	}
}
func TestBallpointTabletIsConstantWidth(t *testing.T) {
	a := &app{width: 10, penSelect: 1}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 10, Pressure: 0.1})
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerPen, X: 50, Y: 10, Pressure: 1})
	for _, p := range a.current.Points {
		if p.Radius != 5 {
			t.Fatal("ballpoint responds to pressure")
		}
	}
}
func TestPaletteDockingClampingAndCancel(t *testing.T) {
	a := &app{viewportW: 960, viewportH: 720, panelDock: 1, panelY: 24}
	a.panelX, a.panelY = a.panelPosition()
	if a.panelX != 960-paletteWidth-8 {
		t.Fatal("default isn't right docked")
	}
	a.dragPalette(ui.InputEvent{Kind: ui.InputPointerDown, X: 15, Y: 15})
	a.dragPalette(ui.InputEvent{Kind: ui.InputPointerMove, X: 15 - 374, Y: 15})
	a.dragPalette(ui.InputEvent{Kind: ui.InputPointerUp})
	if a.panelDock != -1 || a.panelX != 8 {
		t.Fatal("left snap failed")
	}
	a.dragPalette(ui.InputEvent{Kind: ui.InputPointerDown, X: 15, Y: 15})
	a.dragPalette(ui.InputEvent{Kind: ui.InputPointerMove, X: 150, Y: 150})
	a.dragPalette(ui.InputEvent{Kind: ui.InputPointerUp, Canceled: true})
	if a.panelDock != -1 || a.panelX != 8 {
		t.Fatal("canceled drag changed docking")
	}
}

func TestPixelEraseMovementKeepsDryCacheAndModelUnchanged(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 1, paletteMode: 1, eraserSelect: 1, eraserWidth: 8, strokes: []ink.Stroke{eraserFixture()}}
	tt := ui.NewTester(a.view, 100, 80)
	front := a.dryFront
	before := tt.Image().RGBAAt(30, 40)
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 30, Y: 40})
	tt.Frame()
	if a.dryInvalid || a.dryFront != front || len(a.strokes[0].Erasures) != 0 {
		t.Fatal("pointer down invalidated dry page or edited model")
	}
	erased := tt.Image().RGBAAt(30, 40)
	if erased.R != 250 || erased.G != 249 || erased.B != 245 || erased == before {
		t.Fatal("preview did not erase immediately")
	}
	for x := 31; x <= 70; x++ {
		a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: float32(x), Y: 40})
		tt.Frame()
		if a.dryInvalid || a.dryFront != front || len(a.strokes[0].Erasures) != 0 {
			t.Fatal("moving eraser rebuilt page")
		}
		if a.erasePreview.SegmentsDrawn != 1 {
			t.Fatalf("movement resubmitted %d mask segments", a.erasePreview.SegmentsDrawn)
		}
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, Canceled: true})
	tt.Frame()
	if a.dryFront != front || tt.Image().RGBAAt(30, 40) != before || len(a.undo) != 0 {
		t.Fatal("cancel did not reuse unchanged page")
	}
}
func TestPixelEraseCommitAndUndoWithBatchOfUnpaintedEvents(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 1, paletteMode: 1, eraserSelect: 1, eraserWidth: 8, strokes: []ink.Stroke{eraserFixture()}}
	tt := ui.NewTester(a.view, 100, 80)
	before := tt.Image().RGBAAt(30, 40)
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 30, Y: 40})
	for x := 31; x <= 70; x++ {
		a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: float32(x), Y: 40})
	}
	tt.Frame()
	if a.erasePreview.SegmentsDrawn != 41 {
		t.Fatal("coalescing dropped input samples")
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 70, Y: 40})
	tt.Frame()
	if len(a.strokes[0].Erasures) != 1 || len(a.undo) != 1 {
		t.Fatal("gesture wasn't committed exactly once")
	}
	if got := tt.Image().RGBAAt(30, 40); got.R != 250 || got.G != 249 || got.B != 245 {
		t.Fatal("committed erase differs at interior")
	}
	a.undoAction()
	tt.Frame()
	if tt.Image().RGBAAt(30, 40) != before {
		t.Fatal("undo failed")
	}
}

func TestStrokeEraseBatchKeepsIndexAndRepairsOnlyHitArea(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 1, paletteMode: 1, eraserWidth: 6, strokes: []ink.Stroke{
		{Points: []ink.Point{{X: 15, Y: 25, Radius: 3}, {X: 35, Y: 25, Radius: 3}, {X: 55, Y: 25, Radius: 3}}, Color: [4]float32{0, 0, 0, 1}},
		{Points: []ink.Point{{X: 40, Y: 10, Radius: 2}, {X: 40, Y: 25, Radius: 2}, {X: 40, Y: 45, Radius: 2}}, Color: [4]float32{0.8, 0.2, 0.1, 0.5}},
		{Points: []ink.Point{{X: 65, Y: 60, Radius: 3}, {X: 75, Y: 60, Radius: 3}, {X: 90, Y: 60, Radius: 3}}, Color: [4]float32{0, 0, 0, 1}},
	}}
	tt := ui.NewTester(a.view, 100, 80)
	layers := a.dry
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 20, Y: 25})
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: 25, Y: 25})
	if len(a.strokes) != 3 {
		t.Fatal("input event did expensive selection before paint")
	}
	a.hasHover = false // Compare page pixels, excluding the cursor outline overlay.
	tt.Frame()
	if len(a.strokes) != 2 || a.dryInvalid || a.dry != layers {
		t.Fatal("stroke deletion rebuilt full page textures")
	}
	if got := tt.Image().RGBAAt(25, 25); got.R != 250 || got.G != 249 || got.B != 245 {
		t.Fatalf("removed stroke pixels remain: %v, dirty %v front %d", got, a.dirtyRegions, a.dryFront)
	}
	if got := tt.Image().RGBAAt(40, 25); got.G > 210 {
		t.Fatal("overlapping surviving ink was erased")
	}
	if got := tt.Image().RGBAAt(75, 60); got.R > 10 {
		t.Fatal("unaffected region changed")
	}
	index := a.strokeIndex
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: 26, Y: 25})
	tt.Frame()
	if a.strokeIndex != index || a.dry != layers {
		t.Fatal("repeated input rebuilt index or textures")
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 26, Y: 25})
	tt.Frame()
	if len(a.undo) != 1 {
		t.Fatal("one erasing gesture should be one undo")
	}
	a.undoAction()
	tt.Frame()
	if len(a.strokes) != 3 {
		t.Fatal("undo didn't restore all source strokes")
	}
}

func TestStrokePatchMatchesFullRepaintAcrossZoomAndDPR(t *testing.T) {
	for _, ratio := range []float32{1, 2} {
		for _, zoom := range []float64{1, 1.7} {
			r, err := ink.NewRenderer()
			if err != nil {
				t.Fatal(err)
			}
			a := &app{renderer: r, ratio: ratio, zoom: zoom, offsetX: 4, offsetY: 3, paletteMode: 1, eraserWidth: 6, strokes: []ink.Stroke{
				{Points: []ink.Point{{X: 5, Y: 20, Radius: 4}, {X: 50, Y: 20, Radius: 4}, {X: 90, Y: 20, Radius: 4}}, Color: [4]float32{0, 0, 0, 1}},
				{Tool: ink.Highlighter, Points: []ink.Point{{X: 55, Y: 5, Radius: 5}, {X: 60, Y: 25, Radius: 5}, {X: 70, Y: 55, Radius: 5}}, Color: [4]float32{1, 0.6, 0, 0.5}},
				{Tool: ink.Pencil, Stamps: []ink.Stamp{{{70, 23, 0.8, 0.2}, {20, 30, 200, 0}}}, Color: [4]float32{0.2, 0.4, 0.6, 0.8}},
			}}
			tt := ui.NewTester(a.view, 160, 120)
			tt.SetScale(ratio)
			x, y := float32(20*zoom+4), float32(20*zoom+3)
			a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: x, Y: y})
			a.hasHover = false
			tt.Frame()
			if len(a.strokes) != 2 {
				t.Fatal("wrong stroke selection")
			}
			want := ui.Render(func(c *ui.Context) {
				ui.Box(c).Fill().Background(ui.RGB(250, 249, 245)).Draw(func(p *ui.Painter, _ ui.Rect) {
					for _, s := range a.strokes {
						r.DrawViewport(p, s, a.camera(), 160, 120)
					}
				})
			}, 160, 120, ratio)
			got := tt.Image()
			for i, v := range got.Pix {
				d := int(v) - int(want.Pix[i])
				if d > 3 || d < -3 {
					t.Fatalf("DPR%g zoom%g patch mismatch channel%d cached%d full%d", ratio, zoom, i, v, want.Pix[i])
				}
			}
		}
	}
}
