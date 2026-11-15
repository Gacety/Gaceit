package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"image"
	"math"
	"testing"
	"time"
)

func TestPenCaptureStaysThroughQuestionAndFrameRecovery(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.open("relax-ds-03-060")
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	tt.UsePointer(ui.PointerPen, .7)
	tt.Press(1200, 340)
	for i := 1; i <= 32; i++ {
		angle := 2 * math.Pi * float64(i) / 32
		// The pointer passes behind and around the question, keeping the same
		// captured canvas input even when a real surface recovery is needed.
		if i == 16 {
			a.canvasPaintedAt = time.Now().Add(-3 * time.Second)
		}
		tt.Move(720+480*float32(math.Cos(angle)), 340+260*float32(math.Sin(angle)))
		if !tt.Focused("无限草稿画布") {
			t.Fatal("pen lost canvas focus", i)
		}
	}
	tt.Release(1200, 340)
	strokes := a.inkEditor.Snapshot().Strokes
	if len(strokes) != 1 || len(strokes[0].Points) < 33 {
		t.Fatal("continuous curve lost pointer samples", len(strokes))
	}
	before := image.NewRGBA(tt.Image().Rect)
	copy(before.Pix, tt.Image().Pix)
	a.inkEditor.ResetSurfaceCaches()
	tt.Frame()
	if !sameCanvasInk(before, tt.Image()) {
		t.Fatal("incremental ink differs from complete vector rebuild around the question")
	}
	a.FlushWriting()
}

func TestSaveWorkerKeepsNewerEditsAndForcedSaveOrder(t *testing.T) {
	a := testApp(t)
	id := a.M.Current().ID
	a.M.Records[id].Notes = "first"
	a.saveSoon()
	a.startSave()
	a.M.Records[id].Notes = "second"
	a.saveSoon()
	a.collectSave(true)
	if a.M.Records[id].Notes != "second" || !a.savePending {
		t.Fatal("background result replaced a newer edit")
	}
	a.flushSave()
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil || m.Records[id].Notes != "second" {
		t.Fatal("forced save did not supersede the worker", err)
	}
}
