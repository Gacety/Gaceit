package inkcanvas

import (
	"bytes"

	"github.com/egoist/mygo/ui"
	"reflect"
	"study408/internal/ink"
	"testing"
)

func TestSelectionCopyPasteDeleteHistory(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: []ink.Stroke{
		{Points: []ink.Point{{X: 20, Y: 20}, {X: 80, Y: 60}}, Color: [4]float32{0, 0, 0, 1}, Erasures: [][]ink.Point{{{X: 40, Y: 40}}}},
		{Points: []ink.Point{{X: 220, Y: 220}, {X: 280, Y: 260}}, Color: [4]float32{0, 0, 0, 1}},
	}})
	ed.SetTool("lasso")
	ed.selection.ids = []int{0}
	ed.updateSelectionBounds()
	original := ed.Snapshot()
	copied := ed.CopySelection()
	copied[0].Points[0].X = 30
	if ed.strokes[0].Points[0].X != 20 {
		t.Fatal("copy aliases original points")
	}
	copied = ed.CopySelection()
	if !ed.Paste(copied, ui.Point{X: 150, Y: 150}) || len(ed.strokes) != 3 || !ed.HasSelection() {
		t.Fatal("paste did not select independent copy")
	}
	pasted := ed.CopySelection()[0]
	if pasted.Translation[0] == 0 || pasted.Erasures[0][0].X != copied[0].Erasures[0][0].X+pasted.Translation[0] {
		t.Fatal("erasures did not move with pasted ink")
	}
	if !reflect.DeepEqual(original.Strokes, ed.strokes[:2]) {
		t.Fatal("paste changed source strokes")
	}
	copied[0].Erasures[0][0].X = -999
	if ed.strokes[2].Erasures[0][0].X == -999 {
		t.Fatal("paste aliases clipboard")
	}
	afterPaste := ed.Snapshot()
	if !ed.DeleteSelection() || len(ed.strokes) != 2 || ed.HasSelection() {
		t.Fatal("delete failed")
	}
	if !reflect.DeepEqual(ed.strokes, original.Strokes) {
		t.Fatal("delete changed unrelated strokes")
	}
	ed.Undo()
	if !reflect.DeepEqual(ed.Snapshot(), afterPaste) {
		t.Fatal("undo delete failed")
	}
	ed.Undo()
	if !reflect.DeepEqual(ed.Snapshot(), original) {
		t.Fatal("undo paste failed")
	}
	ed.Redo()
	ed.Redo()
	if !reflect.DeepEqual(ed.strokes, original.Strokes) {
		t.Fatal("redo failed")
	}
}

func TestSelectionMoveRepairMatchesFullRebuild(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 80; i++ {
		x, y := float32(20+(i%8)*45), float32(20+(i/8)*28)
		strokes = append(strokes, ink.Stroke{Tool: ink.Pen, Color: [4]float32{0, 0, 0, 1}, Smooth: true, Points: []ink.Point{
			{X: x, Y: y, Radius: 2}, {X: x + 22, Y: y + 8, Radius: 2}, {X: x + 40, Y: y, Radius: 2},
		}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	view := func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }
	tt := ui.NewTester(view, 420, 320)
	tt.Frame()
	ed.SetTool("lasso")
	ed.selection.moving = true
	ed.selection.before = append([]ink.Stroke(nil), strokes...)
	for i := 0; i < len(strokes); i += 3 {
		ed.selection.ids = append(ed.selection.ids, i)
	}
	ed.updateSelectionBounds()
	ed.selection.delta = ui.Point{X: 17, Y: 11}
	ed.processSelectionMove()
	ed.clearSelection()
	tt.Frame()
	moved := ed.Snapshot()

	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.Load(moved)
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 420, 320, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("partial selection repair differs from a full vector render")
	}
	if len(moved.Strokes) != len(strokes) {
		t.Fatal("selection move changed stroke count")
	}
}

func TestSelectionLayerPreviewAndCommitMatchFullRender(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 30; i++ {
		x, y := float32(40+(i%6)*45), float32(40+(i/6)*35)
		s := ink.Stroke{Tool: ink.Pen, Color: [4]float32{0, 0, 0, 1}, Smooth: true, Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 8, Radius: 2}}}
		if i%5 == 0 {
			s.Tool = ink.Highlighter
			s.Color = [4]float32{.8, .6, .1, .4}
		}
		strokes = append(strokes, s)
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 420, 320)
	tt.Frame()
	ed.SetTool("lasso")
	ed.selection.moving = true
	ed.selection.before = append([]ink.Stroke(nil), strokes...)
	for i := len(strokes) / 2; i < len(strokes); i++ {
		ed.selection.ids = append(ed.selection.ids, i)
	}
	ed.updateSelectionBounds()
	tt.Frame()
	if !ed.selection.preview || ed.selection.base == nil {
		t.Fatal("drag did not build a selection base layer")
	}
	ed.selection.delta = ui.Point{X: 1.3, Y: 2.7}
	ed.paletteMode = 0 // omit the dashed selection outline for pixel comparison
	tt.Frame()
	moved := append([]ink.Stroke(nil), strokes...)
	for _, id := range ed.selection.ids {
		moved[id] = translatedStroke(strokes[id], ed.selection.delta)
	}
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.Load(Document{Version: 1, Zoom: 1, Strokes: moved})
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 420, 320, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("selection layer differs from a full vector render")
	}
	ed.selection.commitPending = true
	tt.Frame()
	if ed.selection.moving || ed.selection.preview {
		t.Fatal("selection drop did not commit")
	}
	if len(ed.Snapshot().Strokes) != len(strokes) {
		t.Fatal("selection layer commit lost strokes")
	}
}

func TestSelectionLayerPreservesPaintOrder(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	strokes := []ink.Stroke{
		{Tool: ink.Pen, Color: [4]float32{1, 0, 0, 1}, Smooth: true, Points: []ink.Point{{X: 50, Y: 50, Radius: 8}, {X: 150, Y: 50, Radius: 8}}},
		{Tool: ink.Pen, Color: [4]float32{0, 0, 1, 1}, Smooth: true, Points: []ink.Point{{X: 100, Y: 20, Radius: 8}, {X: 100, Y: 100, Radius: 8}}},
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 220, 160)
	tt.Frame()
	ed.SetTool("lasso")
	ed.selection.moving = true
	ed.selection.before = append([]ink.Stroke(nil), strokes...)
	ed.selection.ids = []int{0}
	ed.updateSelectionBounds()
	tt.Frame()
	if !ed.selection.preview {
		t.Fatal("paint-order runs did not build a preview")
	}
	ed.selection.delta = ui.Point{X: 3, Y: 3}
	ed.paletteMode = 0
	tt.Frame()
	moved := append([]ink.Stroke(nil), strokes...)
	moved[0] = translatedStroke(strokes[0], ed.selection.delta)
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.Load(Document{Version: 1, Zoom: 1, Strokes: moved})
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 220, 160, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("segmented selection layer changed paint order")
	}
}

func TestSelectionDirectRangeFallbackMatchesFullRender(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 600; i++ {
		x, y := float32(20+(i%20)*16), float32(20+(i/20)*8)
		strokes = append(strokes, ink.Stroke{Tool: ink.Pen, Color: [4]float32{0, 0, 0, 1}, Smooth: true, Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 8, Radius: 2}}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 320, 240)
	tt.Frame()
	ed.SetTool("lasso")
	ed.selection.moving = true
	ed.selection.before = append([]ink.Stroke(nil), strokes...)
	for i := 0; i < len(strokes); i += 2 {
		ed.selection.ids = append(ed.selection.ids, i)
	}
	ed.updateSelectionBounds()
	tt.Frame()
	if !ed.selection.preview || !ed.selection.directRange {
		t.Fatal("many z-order runs did not use direct-range fallback")
	}
	ed.selection.delta = ui.Point{X: 2, Y: 3}
	ed.paletteMode = 0
	tt.Frame()
	moved := append([]ink.Stroke(nil), strokes...)
	for _, id := range ed.selection.ids {
		moved[id] = translatedStroke(strokes[id], ed.selection.delta)
	}
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.Load(Document{Version: 1, Zoom: 1, Strokes: moved})
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 320, 240, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("direct-range selection preview differs from full render")
	}
}
