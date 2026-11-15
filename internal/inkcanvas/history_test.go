package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"reflect"
	"study408/internal/ink"
	"testing"
)

func TestDeltaHistoryMixedEditsRestoresExactStates(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: []ink.Stroke{
		{Points: []ink.Point{{X: 20, Y: 20, Radius: 2}, {X: 80, Y: 20, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}},
		{Points: []ink.Point{{X: 20, Y: 80, Radius: 2}, {X: 80, Y: 80, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}},
		{Points: []ink.Point{{X: 20, Y: 140, Radius: 2}, {X: 80, Y: 140, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}},
	}})
	states := []Document{ed.Snapshot()}
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 140, Y: 40}, {Kind: ui.InputPointerMove, X: 160, Y: 60}, {Kind: ui.InputPointerUp, X: 180, Y: 40}} {
		ed.Input(e)
	}
	states = append(states, ed.Snapshot())
	ed.SetTool("eraser")
	ed.SetEraserRegion(true)
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 45, Y: 0}, {Kind: ui.InputPointerMove, X: 45, Y: 35}, {Kind: ui.InputPointerUp, X: 45, Y: 40}} {
		ed.Input(e)
	}
	states = append(states, ed.Snapshot())
	ed.SetEraserRegion(false)
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 45, Y: 65}, {Kind: ui.InputPointerMove, X: 45, Y: 85}, {Kind: ui.InputPointerUp, X: 45, Y: 95}} {
		ed.Input(e)
	}
	states = append(states, ed.Snapshot())
	ed.SetTool("lasso")
	ed.selection.ids = []int{0, 2}
	ed.updateSelectionBounds()
	copied := ed.CopySelection()
	ed.Paste(copied, ui.Point{X: 220, Y: 180})
	states = append(states, ed.Snapshot())
	ed.DeleteSelection()
	states = append(states, ed.Snapshot())
	for i := len(states) - 2; i >= 0; i-- {
		ed.Undo()
		if !reflect.DeepEqual(ed.Snapshot(), states[i]) {
			t.Fatal("delta undo diverged", i)
		}
	}
	for i := 1; i < len(states); i++ {
		ed.Redo()
		if !reflect.DeepEqual(ed.Snapshot(), states[i]) {
			t.Fatal("delta redo diverged", i)
		}
	}
}
func TestPenHistoryRetainsOnlyChangedHeaders(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: make([]ink.Stroke, 10000)})
	for i := 0; i < 60; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, X: float32(i), Y: 0})
		ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: float32(i) + 1, Y: 1})
	}
	if len(ed.undo) != 50 {
		t.Fatal("undo history limit changed")
	}
	headers := 0
	for _, edit := range ed.undo {
		headers += len(edit.added) + len(edit.removed) + 2*len(edit.changed)
	}
	if headers != 50 {
		t.Fatal("pen history retained unrelated document headers", headers)
	}
	for i := 0; i < 50; i++ {
		ed.Undo()
	}
	if len(ed.strokes) != 10010 {
		t.Fatal("50-step boundary failed")
	}
	ed.Redo()
	ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, X: 20, Y: 20})
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: 21, Y: 21})
	if ed.CanRedo() {
		t.Fatal("new edit failed to clear redo")
	}
}
func BenchmarkDeltaPenCommit(b *testing.B) {
	ed, err := New()
	if err != nil {
		b.Fatal(err)
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: make([]ink.Stroke, 10000)})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, X: 20, Y: 20})
		ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: 21, Y: 21})
		ed.Undo()
	}
}
func BenchmarkLegacyPenListCopies(b *testing.B) {
	strokes := make([]ink.Stroke, 10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		before := append([]ink.Stroke(nil), strokes...)
		after := append(before, ink.Stroke{Points: []ink.Point{{X: 20, Y: 20}, {X: 21, Y: 21}}})
		doc := append([]ink.Stroke(nil), after...)
		if len(doc) != 10001 {
			b.Fatal("invalid benchmark")
		}
	}
}
