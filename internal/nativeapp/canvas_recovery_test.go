package nativeapp

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"study408/internal/inkcanvas"
)

func TestCanvasFrameGapRebuildsTextureBase(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.SetTool("scratch")
	if !a.ensureInk() {
		t.Fatal(a.message)
	}
	for i := 0; i < 3; i++ {
		y := float32(260 + i*40)
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 520, Y: y, Button: 0, Pointer: ui.PointerMouse})
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerMove, X: 620, Y: y + 10, Pointer: ui.PointerMouse})
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerUp, X: 700, Y: y - 6, Button: 0, Pointer: ui.PointerMouse})
	}
	a.canvasPaintedAt = time.Now()
	before := ui.Render(a.View, 1440, 900, 1)

	// Simulate the frame gap after sleep or a released surface.
	a.canvasPaintedAt = time.Now().Add(-3 * time.Second)
	after := ui.Render(a.View, 1440, 900, 1)
	if !sameCanvasInk(before, after) {
		t.Fatal("frame-gap recovery did not restore the complete canvas")
	}
}

func TestZoomPreviewSettles(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	if !a.ensureInk() {
		t.Fatal(a.message)
	}
	a.canvasPaintedAt = time.Now()
	_ = ui.Render(a.View, 1440, 900, 1)
	a.inkEditor.ZoomBy(1.5)
	_ = ui.Render(a.View, 1440, 900, 1)
	if !a.inkEditor.PreviewingZoom() {
		t.Fatal("zoom did not start a preview instead of rebuilding every stroke")
	}
	time.Sleep(220 * time.Millisecond)
	_ = ui.Render(a.View, 1440, 900, 1)
	if a.inkEditor.PreviewingZoom() {
		t.Fatal("zoom preview did not settle into a native rebuild")
	}
}

func TestEraserHidesSystemCursor(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	if !a.ensureInk() {
		t.Fatal(a.message)
	}
	tt := ui.NewTester(a.View, 1440, 900)
	box, ok := tt.Find("无限草稿画布")
	if !ok {
		t.Fatal("canvas not found")
	}
	a.inkEditor.SetTool("eraser")
	tt.UsePointer(ui.PointerPen, .5)
	tt.Move(box.X+40, box.Y+box.H-220)
	tt.Frame()
	if tt.Cursor() != ui.CursorNone {
		t.Fatalf("eraser ring kept the system cursor: %v", tt.Cursor())
	}
}

// Compare the whole canvas, including the fixed native controls.
func sameCanvasInk(a, b *image.RGBA) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			i, j := a.PixOffset(x, y), b.PixOffset(x, y)
			for k := 0; k < 4; k++ {
				if a.Pix[i+k] != b.Pix[j+k] {
					return false
				}
			}
		}
	}
	return true
}

func TestZoomPreviewDeadlineSurvivesExtraFrames(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	a.inkEditor.ZoomBy(1.1)
	tt.Frame()
	for i := 0; i < 5; i++ {
		time.Sleep(50 * time.Millisecond)
		tt.Frame()
	}
	if a.inkEditor.PreviewingZoom() {
		t.Fatal("ordinary paint frames kept extending zoom preview")
	}
}

func TestExpandedExplanationRecomposesSurroundingInk(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	// Ink beside the question, outside both the collapsed/expanded card.
	for _, y := range []float32{180, 360, 560} {
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 260, Y: y})
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerMove, X: 350, Y: y + 30})
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerUp, X: 390, Y: y})
	}
	tt.Frame()
	a.choices[0] = "A"
	a.submit(false)
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("本题解析"); !ok {
		t.Fatal("explanation did not expand")
	}
	before := image.NewRGBA(tt.Image().Rect)
	copy(before.Pix, tt.Image().Pix)
	a.inkEditor.ResetSurfaceCaches()
	tt.Frame()
	if !sameCanvasInk(before, tt.Image()) {
		t.Fatal("expansion omitted ink until manual redraw")
	}
	if len(a.inkEditor.Snapshot().Strokes) != 3 {
		t.Fatal("expansion changed vector strokes")
	}
}

func TestCanvasSubmitRecomposesProgressively(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	if !a.ensureInk() {
		t.Fatal(a.message)
	}
	d := inkcanvas.Document{Version: 1, Zoom: canvasInitialZoom}
	for i := 0; i < 40; i++ {
		y := float32(180 + i*6)
		pts := []ink.Point{{X: 120, Y: y, Radius: 1.5}, {X: 220, Y: y + 4, Radius: 1.5}, {X: 320, Y: y - 3, Radius: 1.5}}
		d.Strokes = append(d.Strokes, ink.Stroke{Tool: ink.Pen, Color: [4]float32{1, 1, 1, 1}, Smooth: true, Points: pts})
	}
	a.inkEditor.Load(d)
	_ = ui.Render(a.View, 1440, 900, 1)
	q := a.M.Current()
	a.choices[0] = q.AnswerChoices[0]
	a.submit(false)
	if !a.answered {
		t.Fatal("submit did not grade")
	}
	_ = ui.Render(a.View, 1440, 900, 1)
	if !a.inkEditor.RebuildingNavigation() {
		t.Fatal("card expansion reset the complete ink base instead of rebaking progressively")
	}
	for i := 0; i < 12 && a.inkEditor.RebuildingNavigation(); i++ {
		_ = ui.Render(a.View, 1440, 900, 1)
	}
	if a.inkEditor.RebuildingNavigation() {
		t.Fatal("progressive submit recomposition did not settle")
	}
	if got := len(a.inkEditor.Snapshot().Strokes); got != 40 {
		t.Fatalf("submit recomposition lost strokes: %d", got)
	}
	progressive := ui.Render(a.View, 1440, 900, 1)
	a.inkEditor.ResetSurfaceCaches()
	full := ui.Render(a.View, 1440, 900, 1)
	if !sameCanvasInk(progressive, full) {
		t.Fatal("progressive submit recomposition differs from a complete vector rebuild")
	}
}

func TestNoteClipPreviewDoesNotTakeOverCanvasEditor(t *testing.T) {
	a := testApp(t)
	q := a.M.Current()
	a.M.Records[q.ID].InkClips = []InkClip{{
		ID:      "preview-clip",
		Strokes: []ink.Stroke{{Tool: ink.Pen, Points: []ink.Point{{X: 10, Y: 10, Radius: 1}, {X: 40, Y: 40, Radius: 1}}}},
		Bounds:  [4]float32{10, 10, 40, 40},
	}}
	a.M.Round.Mode = "notes"
	a.selectedNote = q.ID
	a.noteEditing = false
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()
	if a.inkEditor != nil {
		t.Fatal("notes clip preview created the live canvas editor")
	}
	if a.inkPreview == nil {
		t.Fatal("notes clip preview did not create its own editor")
	}
	a.mode("practice")
	a.SetTool("scratch")
	tt.Frame()
	if a.inkEditor == nil {
		t.Fatal("scratch editor was not created")
	}
	if a.inkEditor.HomeZoom() != canvasInitialZoom || a.inkEditor.Zoom() != canvasInitialZoom {
		t.Fatalf("preview leaked canvas configuration: home=%v zoom=%v", a.inkEditor.HomeZoom(), a.inkEditor.Zoom())
	}
}
