package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestCanvasToolShortcutsAfterQuestionInteraction(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	ed := a.inkEditor
	for _, label := range []string{"第1空 选项A", "题干正文"} {
		if label == "题干正文" {
			body, ok := tt.Find(label)
			if !ok {
				t.Fatal("stem not found")
			}
			tt.Press(body.X+4, body.Y+14)
			tt.Move(body.X+120, body.Y+14)
			tt.Release(body.X+120, body.Y+14)
		} else if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		if !a.canvasTextInteraction {
			t.Fatal("question click did not enter text interaction")
		}
		ed.SetTool("pen")
		tt.Key(0, ui.KeyE)
		if ed.Tool() != "eraser" {
			t.Fatalf("E failed after clicking %s: %s", label, ed.Tool())
		}
		tt.Key(0, ui.KeyP)
		if ed.Tool() != "pen" {
			t.Fatalf("P failed after clicking %s: %s", label, ed.Tool())
		}
		if !a.canvasTextInteraction {
			t.Fatal("tool switch took ownership of text clipboard shortcuts")
		}
	}
	if err := tt.Click("本题笔记"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("本题笔记内容"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyE)
	if ed.Tool() != "pen" {
		t.Fatal("note editing switched the ink tool")
	}
	tt.Key(0, ui.KeyP)
	if ed.Tool() != "pen" {
		t.Fatal("note editing switched the ink tool")
	}
}

func TestCanvasPenOverQuestionAndTrackpadInteraction(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "light"
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	option, ok := tt.Find("第1空 选项A")
	if !ok {
		t.Fatal("option not found")
	}
	ed := a.inkEditor
	ed.SetColor(1)
	ed.SetWidth(8)
	x, y := option.X+70, option.Y+8
	before := tt.Image().RGBAAt(int(x+50), int(y))
	tt.UsePointer(ui.PointerPen, .8)
	tt.Press(x, y)
	tt.Move(x+50, y)
	tt.Release(x+100, y)
	if len(ed.Snapshot().Strokes) != 1 {
		t.Fatal("pen did not pass above question")
	}
	if a.choices[0] != "" {
		t.Fatal("pen marked the answer instead of drawing")
	}
	after := tt.Image().RGBAAt(int(x+50), int(y))
	if after == before || after.R < 150 || after.G < 100 || after.B > 130 {
		t.Fatal("ink hidden behind card", before, after)
	}
	ed.SetTool("hand") // touchpad still selects/copies while the pen hand tool is active
	tt.UsePointer(ui.PointerMouse, 0)
	tt.Move(option.X+option.W/2, option.Y+option.H/2)
	if err := tt.Click("第1空 选项A"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "A" || len(ed.Snapshot().Strokes) != 1 {
		t.Fatal("trackpad did not operate option separately")
	}
	body, ok := tt.Find("题干正文")
	if !ok {
		t.Fatal("stem not found")
	}
	tt.Press(body.X+4, body.Y+14)
	tt.Move(body.X+240, body.Y+14)
	tt.Release(body.X+240, body.Y+14)
	tt.Key(ui.Cmd, ui.KeyC)
	if strings.TrimSpace(tt.Clipboard()) == "" {
		t.Fatal("native stem text could not be selected/copied")
	}
	if len(ed.Snapshot().Strokes) != 1 {
		t.Fatal("trackpad text selection created ink")
	}
	// Explicit full-stem copy is available in the native context menu.
	tt.RightClickAt(body.X+30, body.Y+14)
	if err := tt.ChooseMenuItem("复制完整题干"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	if tt.Clipboard() != formatStemItems(a.M.Current().Text) {
		t.Fatal("full-stem copy lost content")
	}
	if os.Getenv("CAPTURE_NATIVE_DEVICES") != "" {
		f, err := os.Create("../../native-review/canvas-pen-over-question.png")
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

func TestCanvasTouchPansWhileEraserAndLassoSelected(t *testing.T) {
	a := testApp(t)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	ed := a.inkEditor
	// Draw a stroke on blank paper to the left of the question card with a pen.
	tt.UsePointer(ui.PointerPen, 0.7)
	tt.Press(120, 260)
	tt.Move(200, 260)
	tt.Release(200, 260)
	if len(ed.Snapshot().Strokes) != 1 {
		t.Fatalf("expected 1 pen stroke, got %d", len(ed.Snapshot().Strokes))
	}

	for _, tool := range []string{"eraser", "lasso"} {
		ed.SetTool(tool)
		before := ed.Camera().Offset
		tt.UsePointer(ui.PointerTouch, 0)
		tt.Press(140, 260)
		tt.Move(220, 310)
		tt.Release(220, 310)
		after := ed.Camera().Offset
		if after == before {
			t.Fatalf("touch drag with %s selected did not pan canvas", tool)
		}
		if len(ed.Snapshot().Strokes) != 1 {
			t.Fatalf("touch drag with %s selected modified stroke", tool)
		}
	}

	// Reset camera home and verify pen eraser still erases the stroke.
	ed.Home()
	ed.SetTool("eraser")
	tt.Frame()
	tt.UsePointer(ui.PointerPen, 0.7)
	tt.Press(150, 250)
	tt.Move(170, 270)
	tt.Release(170, 270)
	if len(ed.Snapshot().Strokes) != 0 {
		t.Fatalf("pen eraser did not erase stroke, remaining: %d", len(ed.Snapshot().Strokes))
	}
}
