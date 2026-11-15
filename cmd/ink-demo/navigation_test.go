package main

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.0001 }
func TestZoomKeepsWorldPointUnderCursor(t *testing.T) {
	a := &app{offsetX: -123, offsetY: 91, zoom: 0.75}
	before := a.toWorld(210, 145)
	a.input(ui.InputEvent{Kind: ui.InputZoom, Scale: 2, X: 210, Y: 145})
	after := a.toWorld(210, 145)
	if !near(before.X, after.X) || !near(before.Y, after.Y) || a.viewZoom() != 1.5 {
		t.Fatal("zoom anchor drifted")
	}
	a.zoomAt(1e6, 210, 145)
	if a.viewZoom() != 8 {
		t.Fatal("upper zoom bound")
	}
	a.zoomAt(1e-12, 210, 145)
	if a.viewZoom() != 0.1 {
		t.Fatal("lower zoom bound")
	}
}
func TestPanningIsUnboundedAndDoesNotWrite(t *testing.T) {
	for _, mode := range []int{0, 1, 2} {
		a := &app{width: 8}
		down := ui.InputEvent{Kind: ui.InputPointerDown, X: 20, Y: 30}
		if mode == 0 {
			down.Button = 2
		}
		if mode == 1 {
			a.input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeySpace})
		}
		if mode == 2 {
			a.handMode = true
		}
		a.input(down)
		a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: -3000, Y: 5000})
		a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: -3000, Y: 5000, Button: down.Button})
		if a.offsetX != -3020 || a.offsetY != 4970 || a.current != nil || len(a.strokes) != 0 {
			t.Fatal("pan was bounded or created ink")
		}
	}
}
func TestPanCancelAndMidStrokeNavigation(t *testing.T) {
	a := &app{offsetX: 10, offsetY: -20}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Button: 2, X: 10, Y: 10})
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, X: 50, Y: 50})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, Button: 2, Canceled: true})
	if a.offsetX != 10 || a.offsetY != -20 {
		t.Fatal("canceled pan changed camera")
	}
	a.current = &ink.Stroke{Points: []ink.Point{{X: 1, Y: 1, Radius: 2}}}
	a.zoomAt(2, 50, 50)
	if a.viewZoom() != 1 {
		t.Fatal("camera changed during ink gesture")
	}
}
func TestWritingAndErasingUseModelCoordinatesAfterNavigation(t *testing.T) {
	a := &app{width: 8, zoom: 2, offsetX: 100, offsetY: 50}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 120, Y: 90, Pointer: ui.PointerMouse})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 140, Y: 90, Pointer: ui.PointerMouse})
	p := a.strokes[0].Points
	if p[0].X != 10 || p[0].Y != 20 || p[1].X != 20 || p[1].Y != 20 || p[0].Radius != 4 {
		t.Fatal("screen coordinates baked into stroke")
	}
	a.paletteMode = 1
	a.eraserSelect = 0
	a.eraserWidth = 4
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 120, Y: 90})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 120, Y: 90})
	if len(a.strokes) != 0 {
		t.Fatal("stroke erase missed transformed ink")
	}
	a.undoAction()
	a.eraserSelect = 1
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 120, Y: 90})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 140, Y: 90})
	cut := a.strokes[0].Erasures[0]
	if cut[0].X != 10 || cut[0].Y != 20 || cut[0].Radius != 2 {
		t.Fatal("pixel erasure recorded screen coordinates")
	}
}
func TestCameraCacheRenderAndNegativeWorld(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 2, zoom: 2, offsetX: 60, offsetY: 30, strokes: []ink.Stroke{{Points: []ink.Point{{X: -15, Y: 0, Radius: 3}}, Color: [4]float32{0, 0, 0, 1}}}}
	tt := ui.NewTester(a.view, 100, 80)
	tt.SetScale(2)
	if got := tt.Image().RGBAAt(60, 60); got.R > 5 {
		t.Fatal("negative world ink not projected")
	}
	a.offsetX = 80
	tt.Frame()
	if got := tt.Image().RGBAAt(100, 60); got.R > 5 {
		t.Fatal("pan didn't invalidate cache")
	}
	a.zoom = 1
	tt.Frame()
	if got := tt.Image().RGBAAt(130, 60); got.R > 5 {
		t.Fatal("zoom didn't invalidate cache")
	}
	if a.strokes[0].Points[0].X != -15 {
		t.Fatal("navigation mutated ink")
	}
}
func TestNativeWheelPinchAndCollapseControls(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 1, width: 8, showTools: true}
	tt := ui.NewTester(a.view, 960, 720)
	before := a.toWorld(200, 300)
	tt.Wheel(200, 300, 0, -50)
	if a.viewZoom() <= 1 {
		t.Fatal("wheel didn't zoom")
	}
	after := a.toWorld(200, 300)
	if !near(before.X, after.X) || !near(before.Y, after.Y) {
		t.Fatal("native wheel anchor drift")
	}
	tt.Pinch(200, 300, 0.5)
	if a.viewZoom() >= 1 {
		t.Fatal("pinch didn't zoom out")
	}
	if err := tt.Click("收纳工具栏"); err != nil {
		t.Fatal(err)
	}
	if !a.panelCollapsed || a.panelX != 960-68-8 {
		t.Fatal("collapse didn't retain docking")
	}
	if _, ok := tt.Find("普通笔选项"); ok {
		t.Fatal("collapsed toolbar still consumes full width")
	}
	if err := tt.Click("展开工具栏"); err != nil {
		t.Fatal(err)
	}
	if a.panelCollapsed || a.panelX != 960-paletteWidth-8 {
		t.Fatal("expand didn't restore toolbar")
	}
	if err := tt.Click("移动画布"); err != nil {
		t.Fatal(err)
	}
	tt.Press(200, 300)
	tt.Move(250, 350)
	tt.Release(250, 350)
	if !a.handMode || len(a.strokes) != 0 {
		t.Fatal("hand tool drew ink")
	}
	if err := tt.Click("普通笔选项"); err != nil {
		t.Fatal(err)
	}
	if a.handMode {
		t.Fatal("pen selection didn't leave hand tool")
	}
}

func TestNativeSpaceDragDoesNotDraw(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 1, width: 8, showTools: true}
	tt := ui.NewTester(a.view, 960, 720)
	if err := tt.Click("普通笔选项"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("收起"); err != nil {
		t.Fatal(err)
	}
	tt.KeyDown(ui.KeySpace)
	tt.Press(200, 350)
	tt.Move(280, 420)
	tt.Release(280, 420)
	tt.KeyUp(ui.KeySpace)
	if a.offsetX != 80 || a.offsetY != 70 || a.spaceDown || len(a.strokes) != 0 {
		t.Fatalf("space drag failed x%g y%g stuck%v", a.offsetX, a.offsetY, a.spaceDown)
	}
}
