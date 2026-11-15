package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
)

// Compare rendered frames with identical history/path/input. The full-rebuild
// baseline reproduces the former per-event metadata/mask invalidation strategy.
// This is the MyGo CPU backend, not a Metal or end-to-end latency benchmark.
func BenchmarkPixelEraserFrames(b *testing.B) {
	for _, incremental := range []bool{false, true} {
		b.Run(fmt.Sprintf("incremental=%v", incremental), func(b *testing.B) {
			r, err := ink.NewRenderer()
			if err != nil {
				b.Fatal(err)
			}
			a := &app{renderer: r, ratio: 1, paletteMode: 1, eraserSelect: 1, eraserWidth: 12}
			for i := 0; i < 60; i++ {
				s := ink.Stroke{Smooth: true, Color: [4]float32{0.1, 0.2, 0.3, 1}}
				for k := 0; k < 40; k++ {
					s.Points = append(s.Points, ink.Point{X: 10 + float32(k*3), Y: 20 + float32(i%10)*7 + float32(k%3), Radius: 2})
				}
				a.strokes = append(a.strokes, s)
			}
			tt := ui.NewTester(a.view, 160, 120)
			a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 20, Y: 40})
			targets := make([]bool, len(a.strokes))
			at := ink.Point{X: 20, Y: 40, Radius: 6}
			end := ink.Point{X: 100, Y: 40, Radius: 6}
			for i, s := range a.strokes {
				targets[i] = ink.HitsStroke(s, at, end)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x := float32(21 + i%80)
				if incremental {
					a.eraseAt(x, 40)
				} else {
					a.erasePath = append(a.erasePath, ink.Point{X: x, Y: 40, Radius: 6})
					for j := range a.strokes {
						if targets[j] {
							a.strokes[j].Erasures = [][]ink.Point{a.erasePath}
						}
					}
					a.erasing = false
					a.dryInvalid = true
				}
				tt.Frame()
			}
		})
	}
}

// Former full scans/full-page invalidation versus frame-batched indexed hits
// and a localized cache repair, with the same scene and eraser trajectory.
func BenchmarkStrokeEraserFrames(b *testing.B) {
	for _, indexed := range []bool{false, true} {
		b.Run(fmt.Sprintf("indexed=%v", indexed), func(b *testing.B) {
			r, err := ink.NewRenderer()
			if err != nil {
				b.Fatal(err)
			}
			a := &app{renderer: r, ratio: 1, paletteMode: 1, eraserSelect: 0, eraserWidth: 8}
			for i := 0; i < 60; i++ {
				s := ink.Stroke{Smooth: true, Color: [4]float32{0.1, 0.2, 0.3, 1}}
				for k := 0; k < 40; k++ {
					s.Points = append(s.Points, ink.Point{X: 10 + float32(k*3), Y: 20 + float32(i%10)*7 + float32(k%3), Radius: 2})
				}
				a.strokes = append(a.strokes, s)
			}
			tt := ui.NewTester(a.view, 160, 120)
			a.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 20, Y: 40})
			from := ink.Point{X: 20, Y: 40, Radius: 4}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				point := ink.Point{X: 21 + float32(i%80), Y: 40, Radius: 4}
				if indexed {
					a.eraseAt(point.X, point.Y)
				} else {
					a.erasing = false
					kept := make([]ink.Stroke, 0, len(a.strokes))
					for _, s := range a.strokes {
						if ink.HitsStroke(s, from, point) {
							a.dryInvalid = true
						} else {
							kept = append(kept, s)
						}
					}
					a.strokes = kept
				}
				tt.Frame()
				from = point
			}
		})
	}
}
