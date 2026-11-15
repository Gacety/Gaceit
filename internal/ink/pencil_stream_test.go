package ink

import (
	"github.com/egoist/mygo/ui"
	"reflect"
	"testing"
)

func TestPencilStreamMatchesOriginalEveryPrefix(t *testing.T) {
	for _, smooth := range []bool{false, true} {
		var stream PencilStream
		s := Stroke{Smooth: smooth, Tool: Pencil}
		for n := 1; n < 100; n++ {
			s.Points = append(s.Points, Point{float32(n * 3), float32(n * n % 80), 1 + float32(n%7)/3})
			if got, want := stream.Finish(s), MakeStrokeStamps(s, 1); !reflect.DeepEqual(got, want) {
				t.Fatalf("stamp sequence changed smooth=%v n=%d got=%d want=%d", smooth, n, len(got), len(want))
			}
		}
	}
}
func TestLivePencilMatchesFullAndUnchangedReusesPrefix(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	var live LiveStroke
	s := Stroke{Smooth: true, Tool: Pencil, Color: [4]float32{.2, .4, .6, .6}}
	tt := ui.NewTester(func(c *ui.Context) {
		theme := ui.LightTheme()
		theme.Background = ui.Transparent
		c.SetTheme(theme)
		ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) { live.Draw(p, r, s, Camera{Zoom: 1, PixelRatio: 1}, 100, 80) })
	}, 100, 80)
	for _, q := range []Point{{10, 15, 3}, {30, 35, 4}, {60, 18, 3}, {85, 55, 3}, {50, 60, 4}, {20, 40, 2}} {
		s.Points = append(s.Points, q)
		tt.Frame()
		assertPixelsNear(t, tt.Image(), render(t, []Stroke{s}, 1, false))
	}
	stable := live.pencilCount
	tt.Frame()
	if live.pencilCount != stable {
		t.Fatal("unchanged pencil regenerated prefix")
	}
	if !reflect.DeepEqual(live.PencilStamps(s), MakeStrokeStamps(s, 1)) {
		t.Fatal("commit changed stamps")
	}
}

func BenchmarkPencilGeneration(b *testing.B) {
	s := Stroke{Smooth: true, Tool: Pencil}
	for n := 0; n < 160; n++ {
		s.Points = append(s.Points, Point{float32(n * 2), float32(n * n % 100), 2})
	}
	b.Run("full_each_frame", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for n := 1; n <= len(s.Points); n++ {
				part := s
				part.Points = s.Points[:n]
				MakeStrokeStamps(part, 1)
			}
		}
	})
	b.Run("incremental", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var stream PencilStream
			for n := 1; n <= len(s.Points); n++ {
				part := s
				part.Points = s.Points[:n]
				stream.Update(part)
			}
		}
	})
}
