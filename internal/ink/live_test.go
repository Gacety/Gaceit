package ink

import (
	"github.com/egoist/mygo/ui"
	"image"
	"testing"
)

func assertPixelsNear(t *testing.T, got, want *image.RGBA) {
	t.Helper()
	if got.Rect != want.Rect {
		t.Fatal("image sizes differ")
	}
	for i, v := range got.Pix {
		if absInt(int(v)-int(want.Pix[i])) > 3 {
			t.Fatalf("channel %d: cached=%d full=%d", i, v, want.Pix[i])
		}
	}
}
func TestLivePrefixMatchesFullStrokeAndBoundsWork(t *testing.T) {
	for _, tool := range []Tool{Pen, Highlighter} {
		for _, scale := range []float32{1, 2} {
			r, err := NewRenderer()
			if err != nil {
				t.Fatal(err)
			}
			var live LiveStroke
			points := []Point{{10, 15, 3}, {30, 35, 4}, {60, 18, 3}, {85, 55, 3}, {50, 60, 4}, {20, 40, 2}, {75, 45, 3}, {40, 20, 2}}
			s := Stroke{Smooth: true, Tool: tool, Color: [4]float32{0.2, 0.4, 0.6, 0.6}}
			theme := ui.LightTheme()
			theme.Background = ui.Transparent
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) { live.Draw(p, r, s, Camera{Zoom: 1, PixelRatio: scale}, 100, 80) })
			}, 100, 80)
			tt.SetScale(scale)
			for n := 1; n <= len(points); n++ {
				s.Points = points[:n]
				tt.Frame()
				if n >= 4 && live.SegmentsDrawn > 2 {
					t.Fatalf("n=%d submits %d segments", n, live.SegmentsDrawn)
				}
				tip := points[n-1]
				if a := alpha(tt.Image(), int(tip.X*scale), int(tip.Y*scale)); a < 140 {
					t.Fatalf("live tip does not reach latest sample: alpha %d", a)
				}
				want := render(t, []Stroke{s}, scale, false)
				assertPixelsNear(t, tt.Image(), want)
			}
			tt.Frame()
			if live.SegmentsDrawn != 0 {
				t.Fatal("unchanged input rebuilt geometry")
			}
			wet, prefix := live.wet, live.prefix
			live.Reset()
			if live.wet != wet || live.prefix != prefix {
				t.Fatal("reset allocated new viewport surfaces")
			}
			s.Points = points[:2]
			tt.Frame()
			assertPixelsNear(t, tt.Image(), render(t, []Stroke{s}, scale, false))
			// A viewport scale change must rebuild the immutable prefix at the new DPR.
			scale = 1.5
			tt.SetScale(scale)
			tt.Frame()
			assertPixelsNear(t, tt.Image(), render(t, []Stroke{s}, scale, false))
			live.Release()
			if live.wet != nil || live.prefix != ([2]*ui.Canvas{}) {
				t.Fatal("release retained viewport surfaces")
			}
		}
	}
}

func BenchmarkLiveStrokeReuse(b *testing.B) {
	r, err := NewRenderer()
	if err != nil {
		b.Fatal(err)
	}
	var live LiveStroke
	s := Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []Point{{10, 15, 3}, {30, 35, 4}, {60, 18, 3}, {85, 55, 3}}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) { live.Draw(p, r, s, Camera{Zoom: 1, PixelRatio: 1}, 100, 80) })
	}, 100, 80)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		live.Reset()
		tt.Frame()
	}
}
