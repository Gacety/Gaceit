package main

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

func TestToolbarClicksSelectToolsWidthAndEraseUndo(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, width: 8, ratio: 1, showTools: true, strokes: []ink.Stroke{{Points: []ink.Point{{X: 100, Y: 200, Radius: 6}}, Color: [4]float32{0, 0, 0, 1}}}}
	tt := ui.NewTester(a.view, 960, 720)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	click("普通笔选项")
	if !a.panelExpanded {
		t.Fatal("pen settings did not open")
	}
	click("圆珠笔")
	if a.penSelect != 1 {
		t.Fatal("pen type unchanged")
	}
	click("笔触粗细")
	// Native slider accepts a drag and its setting affects the next stroke.
	slider, ok := tt.Find("笔触粗细")
	if !ok {
		t.Fatal("slider inaccessible")
	}
	tt.Press(slider.X+slider.W/2, slider.Y+slider.H/2)
	tt.Move(slider.X+slider.W*0.8, slider.Y+slider.H/2)
	tt.Release(slider.X+slider.W*0.8, slider.Y+slider.H/2)
	if a.width < 20 {
		t.Fatalf("width not adjusted: %g", a.width)
	}
	click("荧光笔")
	if a.tool != ink.Highlighter || a.paletteMode != 0 {
		t.Fatal("highlighter not selected")
	}
	click("铅笔")
	if a.tool != ink.Pencil {
		t.Fatal("pencil not selected")
	}
	click("橡皮擦")
	click("区域擦除")
	if _, ok := tt.Find("笔画橡皮"); ok {
		t.Fatal("duplicate eraser entry remains")
	}
	if _, ok := tt.Find("普通橡皮"); ok {
		t.Fatal("duplicate eraser entry remains")
	}
	if a.paletteMode != 1 || a.eraserSelect != 1 {
		t.Fatal("pixel eraser not selected")
	}
	tt.ClickAt(100, 200)
	if len(a.strokes) != 1 || len(a.strokes[0].Erasures) != 1 {
		t.Fatal("pixel eraser UI didn't erase")
	}
	if got := tt.Image().RGBAAt(100, 200); got.R != 250 || got.G != 249 || got.B != 245 {
		t.Fatalf("erasure didn't invalidate rendered cache: %v", got)
	}
	click("撤销上一步")
	if len(a.strokes[0].Erasures) != 0 {
		t.Fatal("UI undo didn't restore")
	}
	click("橡皮擦")
	if !a.panelExpanded {
		click("橡皮擦")
	}
	click("整笔擦除")
	tt.ClickAt(100, 200)
	if len(a.strokes) != 0 {
		t.Fatal("stroke eraser UI didn't remove stroke")
	}
	click("撤销上一步")
	if len(a.strokes) != 1 {
		t.Fatal("stroke undo didn't restore")
	}
	if a.current != nil {
		t.Fatal("toolbar clicks leaked ink onto canvas")
	}
	click("普通笔选项")
	click("线宽 4")
	click("朱红")
	if a.width != 4 || a.colorSelect != 1 {
		t.Fatal("quick settings not applied")
	}
}
func TestToolbarCanDragAndDockWithNativeEvents(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, width: 8, ratio: 1, showTools: true}
	tt := ui.NewTester(a.view, 960, 720)
	tt.Press(a.panelX+14, a.panelY+25)
	tt.Move(30, 120)
	tt.Release(30, 120)
	if a.panelDock != -1 || a.panelX != 8 {
		t.Fatalf("drag failed: dock %d x%g", a.panelDock, a.panelX)
	}
	if len(a.strokes) != 0 {
		t.Fatal("dragging tools drew ink")
	}
	tt.Press(a.panelX+14, a.panelY+25)
	tt.Move(220, 180)
	tt.Release(220, 180)
	if a.panelDock != 0 {
		t.Fatal("toolbar didn't undock")
	}
}
