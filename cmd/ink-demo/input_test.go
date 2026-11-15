package main

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
	"time"
)

func TestMouseUsesSelectedToolAtConstantWidth(t *testing.T) {
	for _, tool := range []ink.Tool{ink.Pen, ink.Pencil, ink.Highlighter} {
		a := &app{width: 8, tool: tool}
		a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerMouse, X: 10, Y: 10, Pressure: 0.5})
		for i, p := range []float32{0, 1, 0.1, 0.9} {
			a.input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerMouse, X: 10 + float32(i)*30, Y: 40, Pressure: p, Time: time.Unix(0, int64(i+1))})
		}
		if a.current.Tool != tool {
			t.Fatal("mouse ignored explicit tool selection")
		}
		for _, p := range a.current.Points {
			if p.Radius != 4 {
				t.Fatalf("mouse radius varies: %g", p.Radius)
			}
		}
	}
}
func TestTabletUsesRealPressureIncludingStationaryAndZero(t *testing.T) {
	a := &app{width: 10}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 20, Pressure: 0})
	for _, pressure := range []float32{0.25, 1, 0} {
		a.input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerPen, X: 10, Y: 20, Pressure: pressure})
	}
	points := a.current.Points
	if len(points) != 4 {
		t.Fatal("pressure-only input discarded")
	}
	want := []float32{1, 2, 5, 1}
	for i, p := range points {
		if p.Radius != want[i] {
			t.Fatalf("pressure sample %d: radius %g, want %g", i, p.Radius, want[i])
		}
	}
}
func TestTabletWidthIndependentOfSpeed(t *testing.T) {
	a := &app{width: 10}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 10, Pressure: 0.5})
	for i, x := range []float32{10.1, 80, 80.01, 200} {
		a.input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerPen, X: x, Y: 10, Pressure: 0.5, Time: time.Unix(int64(i), 0)})
	}
	for _, p := range a.current.Points {
		if p.Radius != 3 {
			t.Fatal("speed changed tablet width")
		}
	}
	if len(a.current.Points) != 5 || a.current.Points[3].X != 80.01 {
		t.Fatal("subpixel input delayed or filtered")
	}
}
func TestTabletReleaseDoesNotInventPressureTaper(t *testing.T) {
	a := &app{width: 10}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 10, Pressure: 1})
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, Pointer: ui.PointerPen, X: 11, Y: 10, Pressure: 0})
	p := a.strokes[0].Points
	if len(p) != 2 || p[1].Radius != p[0].Radius || p[1].X != 11 {
		t.Fatal("release invented a thin tip or lost final position")
	}
}
func TestPointerInputSelectsSmoothChain(t *testing.T) {
	a := &app{width: 8}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 10, Pressure: 0.5})
	if a.current == nil || !a.current.Smooth {
		t.Fatal("raw samples still rendered as independent windows")
	}
	a.input(ui.InputEvent{Kind: ui.InputPointerUp, Canceled: true})
	if a.current != nil || len(a.strokes) != 0 {
		t.Fatal("cancel committed a stroke")
	}
}
func TestTabletToolSelectionAndHighlighterWidth(t *testing.T) {
	a := &app{width: 20, tool: ink.Highlighter}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 10, Pressure: 0.1})
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerPen, X: 20, Y: 20, Pressure: 1})
	if a.current.Tool != ink.Highlighter {
		t.Fatal("tablet tool replaced")
	}
	for _, p := range a.current.Points {
		if p.Radius != 10 {
			t.Fatal("highlighter width varies")
		}
	}
}

func TestBallpointSettingDoesNotDisablePencilPressure(t *testing.T) {
	a := &app{width: 10, penSelect: 1, tool: ink.Pencil}
	a.input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 10, Y: 10, Pressure: 0.1})
	a.input(ui.InputEvent{Kind: ui.InputPointerMove, Pointer: ui.PointerPen, X: 50, Y: 10, Pressure: 1})
	if a.current.Points[0].Radius >= a.current.Points[1].Radius {
		t.Fatal("ordinary pen subtype leaked into pencil pressure")
	}
}
