package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"math"
	"os"
	"testing"
)

func TestCanvasQuestionSharesWorldCamera(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.open("relax-ds-03-060")
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	initial, _ := tt.Find("画布浮动题目")
	stem, _ := tt.Find("题干正文")
	near := func(got, want float32) bool { return math.Abs(float64(got-want)) < 1 }
	// Pan over the world question: its screen position changes by the camera
	// delta, while its world coordinates stay unchanged.
	tt.Scroll(stem.X+30, stem.Y+25, 40, 40)
	frame, _ := tt.Find("画布浮动题目")
	if !near(frame.X, initial.X-40) || !near(frame.Y, initial.Y-40) || frame.W != initial.W {
		t.Fatal("pan did not move the world card correctly", initial, frame)
	}
	tt.Pinch(stem.X+30, stem.Y+25, .8)
	frame, _ = tt.Find("画布浮动题目")
	newStem, _ := tt.Find("题干正文")
	camera := a.inkEditor.Snapshot()
	if !near(frame.W, initial.W*float32(camera.Zoom/.8)) || !near(newStem.W, stem.W*float32(camera.Zoom/.8)) || !near(frame.X, 360*float32(camera.Zoom)+float32(camera.X)) || !near(frame.Y, 110*float32(camera.Zoom)+float32(camera.Y)) {
		t.Fatal("card and text did not scale with ink", frame, newStem, camera)
	}
	if err := tt.Click("第1空 选项B"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "B" {
		t.Fatal("scaled question option did not receive click")
	}
	// A pen hand gesture routes through the ink overlay. Dragging over the
	// scaled card must still move by the real pointer delta, without drift.
	a.inkEditor.SetTool("hand")
	tt.Frame()
	before := a.inkEditor.Snapshot()
	x, y := newStem.X+25, newStem.Y+20
	tt.UsePointer(ui.PointerPen, .5)
	tt.Press(x, y)
	tt.Move(x+60, y+40)
	tt.Release(x+60, y+40)
	tt.UsePointer(ui.PointerMouse, 0)
	camera = a.inkEditor.Snapshot()
	if !near(float32(camera.X-before.X), 60) || !near(float32(camera.Y-before.Y), 40) {
		t.Fatal("scaled drag has coordinate drift", before, camera)
	}
	if err := tt.Click("回到题目"); err != nil {
		t.Fatal(err)
	}
	home, _ := tt.Find("画布浮动题目")
	homeStem, _ := tt.Find("题干正文")
	if home != initial || homeStem != stem || a.inkEditor.Zoom() != .8 {
		t.Fatal("home did not restore the starting question view", home, initial)
	}
	a.canvasNotes = true
	tt.Frame()
	withNotes, _ := tt.Find("画布浮动题目")
	if withNotes != home {
		t.Fatal("note overlay moved the world anchor")
	}
	a.canvasNotes = false
	a.inkEditor.SetTool("pen")
	tt.Frame()
	if err := tt.Click("第1空 选项B"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "B" {
		t.Fatal("world card options are not clickable")
	}
	if len(camera.Strokes) != 0 || len(a.M.Records[a.M.Current().ID].Attempts) != 0 {
		t.Fatal("navigation recorded ink or an answer")
	}
}

func TestCaptureCanvasOpenIcons(t *testing.T) {
	if os.Getenv("CAPTURE_NATIVE_CANVAS") == "" {
		t.Skip("manual visual capture")
	}
	a := testApp(t)
	a.M.Theme = "dark"
	a.open("relax-ds-03-060")
	a.SetTool("scratch")
	for _, zoom := range []float64{1, .75} {
		if a.inkEditor != nil {
			a.inkEditor.ZoomBy(zoom / a.inkEditor.Zoom())
		}
		img := ui.Render(a.View, 1440, 900, 1)
		name := "canvas-phosphor-icons"
		if zoom != 1 {
			name = "canvas-world-75"
		}
		f, err := os.Create("../../native-review/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

func TestCanvasLongQuestionExpandsNaturally(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.open("relax-os-03-037")
	a.SetTool("scratch")
	a.answered = true
	tt := ui.NewTester(a.View, 1440, 2000)
	d := a.inkEditor.Snapshot()
	d.Zoom = .5
	d.X = 0
	d.Y = 0
	a.inkEditor.Load(d)
	tt.Frame()
	card, ok := tt.Find("画布浮动题目")
	if !ok || card.H/.5 <= 660 {
		t.Fatal("long question still height-limited", card)
	}
	for _, label := range []string{"第1空 选项D", "本题解析", "重新作答"} {
		r, ok := tt.Find(label)
		if !ok || r.H <= 0 || r.Y < card.Y || r.Y+r.H > card.Y+card.H+.2 {
			t.Fatal("long question content is clipped", label, r, card)
		}
	}
	if os.Getenv("CAPTURE_NATIVE_CANVAS") != "" {
		f, err := os.Create("../../native-review/canvas-long-full.png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
