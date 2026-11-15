package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"testing"
)

func selectTestInk(a *App) {
	a.inkEditor.SetTool("lasso")
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 40, Y: 220}, {Kind: ui.InputPointerMove, X: 210, Y: 220}, {Kind: ui.InputPointerMove, X: 210, Y: 310}, {Kind: ui.InputPointerMove, X: 40, Y: 310}, {Kind: ui.InputPointerUp, X: 40, Y: 220}} {
		a.canvasInput(e)
	}
}
func drawTestInk(a *App) {
	a.inkEditor.SetTool("pen")
	for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 70, Y: 240}, {Kind: ui.InputPointerMove, X: 140, Y: 260}, {Kind: ui.InputPointerUp, X: 180, Y: 280}} {
		a.canvasInput(e)
	}
}
func TestCanvasSelectionKeyboardClipboard(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	drawTestInk(a)
	selectTestInk(a)
	tt.Frame()
	if !a.inkEditor.HasSelection() {
		t.Fatal("lasso did not select stroke")
	}
	if os.Getenv("CAPTURE_LASSO") != "" {
		f, err := os.Create("../../native-review/canvas-lasso.png")
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	tt.Key(ui.Cmd, ui.KeyC)
	data, err := tt.ClipboardData().Read(inkClipboardFormat)
	if err != nil || len(data) == 0 {
		t.Fatal("copy shortcut did not write vector clipboard", err)
	}
	tt.Key(ui.Cmd, ui.KeyV)
	if len(a.inkEditor.Snapshot().Strokes) != 2 || !a.inkEditor.HasSelection() {
		t.Fatal("paste shortcut failed")
	}
	tt.Key(0, ui.KeyBackspace)
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("delete shortcut failed")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if len(a.inkEditor.Snapshot().Strokes) != 2 {
		t.Fatal("undo shortcut failed")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("redo shortcut failed")
	}
	a.canvasNotes = true
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("notes mode hijacked paste")
	}
}
func TestScratchSessionDoesNotSurviveQuit(t *testing.T) {
	a := testApp(t)
	a.save()
	a.SetTool("scratch")
	a.ensureInk()
	id := a.M.Current().ID
	drawTestInk(a)
	if a.savePending {
		t.Fatal("scratch scheduled a disk write")
	}
	a.closeCanvas()
	a.tool = ""
	a.SetTool("scratch")
	a.ensureInk()
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("closing canvas lost session")
	}
	a.open("relax-cn-01-001")
	a.ensureInk()
	if len(a.inkEditor.Snapshot().Strokes) != 0 {
		t.Fatal("question inherited another scratch")
	}
	a.open(id)
	a.ensureInk()
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("question switch lost session")
	}
	selectTestInk(a)
	a.saveInkSelection()
	a.M.Records[id].Notes = "保留的笔记"
	a.save()
	a.FlushWriting()
	b, err := New(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	b.SetTool("scratch")
	b.ensureInk()
	if len(b.inkEditor.Snapshot().Strokes) != 0 {
		t.Fatal("new application restored transient scratch")
	}
	if r := b.M.Records[id]; r.Notes != "保留的笔记" || len(r.InkClips) != 1 {
		t.Fatal("quit lost explicit notes")
	}
}

func TestHistoricalScratchPreservedWithoutRestoring(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	a.ensureInk()
	drawTestInk(a)
	id := a.M.Current().ID
	historical := a.inkEditor.Snapshot()
	a.M.Records[id].Ink = &historical
	a.save()
	b, err := New(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	b.SetTool("scratch")
	b.ensureInk()
	if len(b.inkEditor.Snapshot().Strokes) != 0 {
		t.Fatal("historical scratch automatically restored")
	}
	drawTestInk(b)
	drawTestInk(b)
	b.save()
	b.FlushWriting()
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Records[id].Ink; d == nil || len(d.Strokes) != 1 {
		t.Fatal("session replaced historical document")
	}
}

func TestCanvasNotesButtonWithoutMinimap(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	if _, ok := tt.Find("画布缩略图"); ok {
		t.Fatal("minimap still rendered")
	}
	drawTestInk(a)
	if err := tt.Click("本题笔记"); err != nil {
		t.Fatal(err)
	}
	if !a.canvasNotes {
		t.Fatal("notes button did not open note card")
	}
	if _, ok := tt.Find("本题笔记内容"); !ok {
		t.Fatal("note input missing")
	}
	if err := tt.Click("本题笔记"); err != nil {
		t.Fatal(err)
	}
	if a.canvasNotes {
		t.Fatal("notes button did not close note card")
	}
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("notes toggle lost scratch")
	}
}

func TestScratchSnapshotsOnlyAtBoundaries(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	id := a.M.Current().ID
	for i := 0; i < 5; i++ {
		drawTestInk(a)
		tt.Frame()
	}
	if len(a.inkSessions) != 0 {
		t.Fatal("ordinary strokes copied a session document")
	}
	if len(a.inkEditor.Snapshot().Strokes) != 5 || !a.inkDirty {
		t.Fatal("active editor lost unsnapshotted ink")
	}
	a.open("relax-cn-01-001")
	tt.Frame()
	if len(a.inkSessions[id].Strokes) != 5 {
		t.Fatal("question boundary did not capture latest ink")
	}
	a.open(id)
	tt.Frame()
	if len(a.inkEditor.Snapshot().Strokes) != 5 {
		t.Fatal("returning to question lost session ink")
	}
	a.closeCanvas()
	if len(a.inkSessions[id].Strokes) != 5 {
		t.Fatal("closing canvas lost snapshot")
	}
}
