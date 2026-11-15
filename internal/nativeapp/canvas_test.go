package nativeapp

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"slices"
	"testing"
	"time"
)

func TestCanvasSessionThemesAndPersistentClips(t *testing.T) {
	a := testApp(t)
	a.save()
	a.M.Theme = "dark"
	a.SetTool("scratch")
	if !a.ensureInk() {
		t.Fatal(a.message)
	}
	id := a.M.Round.CurrentID
	original, _ := json.Marshal(a.M.Records[id].Strokes)
	// Real native input path outside the question and floating controls.
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 70, Y: 240, Button: 0, Pointer: ui.PointerMouse}, {Kind: ui.InputPointerMove, X: 140, Y: 260, Pointer: ui.PointerMouse}, {Kind: ui.InputPointerUp, X: 180, Y: 280, Button: 0, Pointer: ui.PointerMouse}} {
		a.canvasInput(e)
	}
	r := a.M.Records[id]
	if d := a.inkEditor.Snapshot(); len(d.Strokes) != 1 || d.Strokes[0].Color != [4]float32{1, 1, 1, 1} {
		t.Fatal("dark ink not committed in white")
	}
	after, _ := json.Marshal(r.Strokes)
	if string(original) != string(after) {
		t.Fatal("legacy drawing overwritten")
	}
	a.M.Theme = "light"
	a.ensureInk()
	if a.inkEditor.Snapshot().Strokes[0].Color != [4]float32{0, 0, 0, 1} {
		t.Fatal("old ink did not adapt to light")
	}
	// Select the complete real stroke using the reused lasso controller.
	a.inkEditor.SetTool("lasso")
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 50, Y: 220}, {Kind: ui.InputPointerMove, X: 200, Y: 220}, {Kind: ui.InputPointerMove, X: 200, Y: 300}, {Kind: ui.InputPointerMove, X: 50, Y: 300}, {Kind: ui.InputPointerUp, X: 50, Y: 220}} {
		a.canvasInput(e)
	}
	a.saveInkSelection()
	if len(r.InkClips) != 1 {
		t.Fatal("lasso clip was not stored")
	}
	if !slices.Contains(a.M.IDs("全部学科", "notes", time.Now()), id) {
		t.Fatal("clip-only note missing from gallery")
	}
	r.NoteTags = []string{"概念不清"}
	r.NotePrivate = true
	a.save()
	r.NoteTags = nil
	r.NotePrivate = false
	a.inkChanged()
	a.flushInk()
	a.saveSoon()
	a.flushSave()
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	saved := m.Records[id]
	if saved.Ink != nil || len(saved.InkClips) != 1 || saved.NotePrivate || len(saved.NoteTags) != 0 {
		t.Fatal("ink, clip or toggled metadata failed reload")
	}
	a.inkEditor.Undo()
	if len(a.inkEditor.Snapshot().Strokes) != 0 {
		t.Fatal("ink undo failed")
	}
	a.inkEditor.Redo()
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("ink redo failed")
	}
}
func TestCanvasNavigationAndCorruptDocument(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	a.ensureInk()
	before := a.inkEditor.Snapshot()
	a.canvasInput(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 40, DY: -70})
	a.canvasInput(ui.InputEvent{Kind: ui.InputZoom, Scale: 1.5, X: 100, Y: 100})
	a.flushInk()
	after := a.inkSessions[a.M.Current().ID]
	if after.X == before.X || after.Zoom <= before.Zoom || after.Zoom > before.Zoom*1.12+.0001 || len(after.Strokes) != len(before.Strokes) {
		t.Fatal("navigation did not preserve ink")
	}
	after.Version = 99
	a.M.Records[a.M.Current().ID].Ink = &after
	a.save()
	data, _ := os.ReadFile(a.M.Disk.Path)
	if _, err := NewModel(assetsForTest(), a.M.Disk); err == nil {
		t.Fatal("unsupported canvas version accepted")
	}
	again, _ := os.ReadFile(a.M.Disk.Path)
	if string(data) != string(again) {
		t.Fatal("invalid input overwritten")
	}
}
func TestCaptureNativeCanvasDesign(t *testing.T) {
	if os.Getenv("CAPTURE_NATIVE_CANVAS") == "" {
		t.Skip("manual visual capture")
	}
	a := testApp(t)
	a.mode("wrong")
	a.M.Theme = "dark"
	a.M.Round.WrongScope = "all"
	a.open("relax-ds-03-060")
	a.SetTool("scratch")
	a.ensureInk()
	a.choices[0] = "B"
	a.inkEditor.Load(legacyInk(&Record{}))
	// Small vector fixture, only in this test's isolated store.
	for y := float32(280); y < 470; y += 60 {
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 65, Y: y})
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerMove, X: 240, Y: y})
		a.canvasInput(ui.InputEvent{Kind: ui.InputPointerUp, X: 260, Y: y})
	}
	r := a.M.Records[a.M.Current().ID]
	r.Notes = "错因：没有先画出队列状态，就凭印象选了答案。\n\n下次：先模拟一遍，再选。"
	r.NoteTags = []string{"概念不清"}
	d := a.inkEditor.Snapshot()
	r.InkClips = []InkClip{{ID: "qa-clip", Strokes: d.Strokes, Bounds: [4]float32{60, 270, 270, 480}}}
	for _, state := range []struct {
		name                  string
		dark, progress, notes bool
		w, h                  int
	}{{"canvas-dark", true, false, false, 1440, 900}, {"canvas-light", false, false, false, 1440, 900}, {"canvas-progress", true, true, false, 1440, 900}, {"canvas-note", true, false, true, 1440, 900}, {"canvas-narrow", false, false, true, 1060, 720}} {
		a.M.Theme = "light"
		if state.dark {
			a.M.Theme = "dark"
		}
		a.canvasProgress = state.progress
		a.canvasNotes = state.notes
		img := ui.Render(a.View, state.w, state.h, 1)
		f, err := os.Create("../../native-review/" + state.name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		_ = png.Encode(f, img)
		f.Close()
	}
	// Native controls really enter/exit the page and expose the progress popup.
	a.M.Theme = "dark"
	a.canvasNotes = false
	tt := ui.NewTester(a.View, 1440, 900)
	if err := tt.Click("打开进度面板"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("练习进度面板"); !ok {
		t.Fatal("progress panel did not open")
	}
	if err := tt.Click("退出草稿"); err != nil {
		t.Fatal(err)
	}
	if a.tool != "" {
		t.Fatal("canvas did not exit")
	}
	if _, ok := tt.Find("错题题目区域"); !ok {
		t.Fatal("reader did not return")
	}
}
