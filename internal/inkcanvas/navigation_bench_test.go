package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
	"time"
)

func navigationFixture(b *testing.B) (*Editor, *ui.Tester) {
	b.Helper()
	ed, err := New()
	if err != nil {
		b.Fatal(err)
	}
	strokes := make([]ink.Stroke, 240)
	for i := range strokes {
		points := make([]ink.Point, 40)
		for j := range points {
			points[j] = ink.Point{X: float32(i%20)*32 + float32(j)*2, Y: float32(i/20)*24 + float32(j%7)*2, Radius: 1.5}
		}
		strokes[i] = ink.Stroke{Points: points, Smooth: true, Color: [4]float32{0, 0, 0, 1}}
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 480, 320)
	return ed, tt
}
func BenchmarkFastPan(b *testing.B) {
	ed, tt := navigationFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ed.offsetX = float64((i % 2) * -200)
		ed.offsetY = float64((i % 2) * -80)
		tt.Frame()
	}
}
func BenchmarkLargeSettledZoom(b *testing.B) {
	ed, tt := navigationFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ed.ZoomBy(2 / ed.Zoom())
		if i%2 == 1 {
			ed.ZoomBy(.5 / ed.Zoom())
		}
		ed.zoomChangedAt = time.Now().Add(-time.Second)
		tt.Frame()
	}
}
