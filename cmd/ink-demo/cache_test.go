package main

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

func TestDryCacheMatchesFullCompositeAndStaysStable(t *testing.T) {
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, ratio: 1}
	tt := ui.NewTester(a.view, 100, 80)
	for i := 0; i < 5; i++ {
		s := ink.Stroke{Smooth: true, Color: [4]float32{0.2, 0.3, 0.4, 1}, Points: []ink.Point{{X: 10, Y: 20 + float32(i*5), Radius: 3}, {X: 50, Y: 40, Radius: 4}, {X: 90, Y: 20, Radius: 3}}}
		if i%2 == 1 {
			s.Tool = ink.Highlighter
			s.Color = [4]float32{1, 0.6, 0, 0.5}
		}
		a.strokes = append(a.strokes, s)
		tt.Frame()
		want := ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.RGB(250, 249, 245)).Draw(func(p *ui.Painter, _ ui.Rect) {
				for _, s := range a.strokes {
					r.Draw(p, s, ink.Camera{Zoom: 1, PixelRatio: 1})
				}
			})
		}, 100, 80, 1)
		got := tt.Image()
		for j, v := range got.Pix {
			d := int(v) - int(want.Pix[j])
			if d > 3 || d < -3 {
				t.Fatalf("stroke %d pixel channel %d: cached %d full %d", i, j, v, want.Pix[j])
			}
		}
	}
	front := a.dryFront
	tt.Frame()
	if a.dryFront != front || a.bakedCount != len(a.strokes) {
		t.Fatal("unchanged history rebuilt")
	}
	a.current = &ink.Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: 10, Y: 60, Radius: 3}}}
	tt.Frame()
	if a.dryFront != front {
		t.Fatal("wet ink invalidated dry cache")
	}
}
