package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
)

func TestPenIsolationSurvivesForeignMouseRelease(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c).HandleInput(ed.Input) }) }, 320, 240)
	tt.UsePointer(ui.PointerPen, .6)
	tt.Press(40, 40)
	tt.Move(80, 20)
	tt.UsePointer(ui.PointerMouse, 0)
	tt.Move(250, 200)
	tt.Release(250, 200)
	tt.Press(250, 200)
	if ed.current == nil || len(ed.current.Points) != 2 {
		t.Fatal("foreign mouse changed pen capture")
	}
	tt.UsePointer(ui.PointerPen, .8)
	tt.Move(120, 60)
	tt.Release(140, 40)
	if ed.current != nil || len(ed.strokes) != 1 || len(ed.strokes[0].Points) != 4 {
		t.Fatal("pen did not finish after foreign input")
	}
	for _, pt := range ed.strokes[0].Points {
		if pt.X > 140 || pt.Y > 60 {
			t.Fatal("foreign coordinates entered stroke")
		}
	}
	if got := ed.InputDiagnostics(); got.ForeignEvents != 3 {
		t.Fatal("missing ignored-event diagnostics", got)
	}
	// The isolation is only for an active pen; normal mouse writing still works.
	tt.UsePointer(ui.PointerMouse, 0)
	tt.Press(10, 180)
	tt.Release(60, 180)
	if len(ed.strokes) != 2 {
		t.Fatal("mouse remained locked after pen release")
	}
	tt.UsePointer(ui.PointerPen, .5)
	tt.Press(20, 100)
	tt.Move(50, 110)
	tt.CancelPress()
	if ed.Busy() || len(ed.strokes) != 3 || ed.InputDiagnostics().Cancellations != 1 {
		t.Fatal("genuine system cancellation was swallowed")
	}
}
func TestInputDiagnosticsBoundedAndPreserveSlowSamples(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, Time: at})
	for i := 1; i <= 50; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerMouse, X: 900, Y: 900, Time: at})
		ed.Input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerPen, X: float32(i), Y: float32(i % 5), Time: at.Add(time.Duration(i) * 100 * time.Millisecond)})
	}
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, Pointer: ui.PointerPen, X: 51, Time: at.Add(5100 * time.Millisecond)})
	d := ed.InputDiagnostics()
	if len(d.Recent) != 32 || d.LongGaps != 51 || d.ForeignEvents != 50 || d.MaxGapMS != 100 {
		t.Fatal("unbounded or inaccurate diagnostics", d)
	}
	if len(ed.strokes) != 1 || len(ed.strokes[0].Points) != 52 {
		t.Fatal("diagnostics altered slow samples")
	}
}
