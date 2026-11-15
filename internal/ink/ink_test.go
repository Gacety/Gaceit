package ink

import (
	"image"
	"math"
	"math/rand"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

func render(t *testing.T, strokes []Stroke, scale float32, background bool) *image.RGBA {
	t.Helper()
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	theme := ui.LightTheme()
	theme.Background = ui.Transparent
	return ui.Render(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Box(c).Fill().Draw(func(p *ui.Painter, rect ui.Rect) {
			if background {
				p.Fill(rect, ui.RGB(255, 255, 255), 0)
			}
			for _, s := range strokes {
				r.Draw(p, s, Camera{Zoom: 1, PixelRatio: scale})
			}
		})
	}, 100, 80, scale)
}
func alpha(img *image.RGBA, x, y int) int { return int(img.RGBAAt(x, y).A) }
func TestSlidingWindowsAndDegenerates(t *testing.T) {
	for n := 0; n <= 6; n++ {
		pts := make([]Point, n)
		for i := range pts {
			pts[i] = Point{float32(i * 10), 10, 1}
		}
		want := max(1, n-2)
		if n == 0 {
			want = 0
		}
		got := Segments(pts, [2]float32{}, 1)
		if len(got) != want {
			t.Fatalf("n=%d: %d windows", n, len(got))
		}
		if n == 1 && got[0].Mode != 2 {
			t.Fatal("single point isn't a circle")
		}
		if n == 2 && got[0].Mode != 1 {
			t.Fatal("two points aren't a line")
		}
	}
}
func TestThresholdsAndContainment(t *testing.T) {
	cases := []struct {
		p    []Point
		mode int
	}{
		{[]Point{{0, 0, 1}, {0.02, 0, 1}, {0, 0.02, 1}}, 2},
		{[]Point{{0, 0, 1}, {50, 0.0049, 1}, {100, 0, 1}}, 1},
		{[]Point{{0, 0, 1}, {50, 0.0051, 1}, {100, 0, 1}}, 0},
		{[]Point{{0, 0, 10}, {5, 50, 1}, {1, 0, 1}}, 2},
	}
	for _, c := range cases {
		if got := Segments(c.p, [2]float32{}, 1)[0].Mode; got != c.mode {
			t.Errorf("got mode %d want %d", got, c.mode)
		}
	}
}
func TestPhaseProjectionAndQuadraticRadius(t *testing.T) {
	if Quad(1, 9, 1, 0.5) != 5 {
		t.Fatal("radius isn't quadratic")
	}
	p, q := Point{2, 3, 4}, Point{5, 7, 6}
	// Endpoint derivative (6,8,4); envelope chord midpoint is C-r*dr*d/|d|².
	if got := phaseOffset(p, q); math.Abs(float64(got-1.6)) > 1e-6 {
		t.Fatal(got)
	}
	if phaseOffset(q, p) != 0 {
		t.Fatal("inward projection must be discarded")
	}
	if phaseOffset(Point{0, 0, 4}, Point{1, 0, 20}) != 4 {
		t.Fatal("missing envelope fallback")
	}
	a := Segments([]Point{{0, 0, 2}, {20, 30, 8}, {40, 0, 1}}, [2]float32{}, 1)[0]
	b := Segments([]Point{{0, 0, 2}, {20, 30, 8}, {40, 0, 1}}, [2]float32{500, -20}, 1)[0]
	if a.Phase != b.Phase || a.P != b.P {
		t.Fatal("translation changed normalized geometry")
	}
}
func TestCirclePhysicalPixelAA(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		img := render(t, []Stroke{{Points: []Point{{40, 40, 10}}, Color: [4]float32{0, 0, 0, 1}}}, scale, false)
		if alpha(img, int(40*scale), int(40*scale)) != 255 {
			t.Fatal("circle center missing")
		}
		// Pixel center exactly on the right edge: coverage is about 1/2 at both DPRs.
		img = render(t, []Stroke{{Points: []Point{{40 + 0.5/scale, 40 + 0.5/scale, 10}}, Color: [4]float32{0, 0, 0, 1}}}, scale, false)
		got := alpha(img, int(50*scale), int(40*scale))
		if got < 124 || got > 131 {
			t.Fatalf("DPR %g: edge alpha %d", scale, got)
		}
		if alpha(img, int(50*scale)+1, int(40*scale)) != 0 {
			t.Fatal("AA wider than 1 physical pixel")
		}
	}
}
func TestQuadraticRadiusLine(t *testing.T) {
	img := render(t, []Stroke{{Points: []Point{{10, 40, 2}, {50, 40, 18}, {90, 40, 2}}, Color: [4]float32{0, 0, 0, 1}}}, 1, false)
	if alpha(img, 50, 48) != 255 || alpha(img, 50, 51) != 0 {
		t.Fatal("line control radius not quadratically interpolated")
	}
}
func TestCubicDistanceAgainstDenseOracle(t *testing.T) {
	// Constant radii isolate the centerline cubic solver. Includes high curvature
	// and the symmetric q=p=0 triple-root point that used to produce NaNs.
	pts := []Point{{15, 60, 5}, {50, -15, 5}, {85, 60, 5}}
	img := render(t, []Stroke{{Points: pts, Color: [4]float32{0, 0, 0, 1}}}, 1, false)
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 150; i++ {
		x, y := rng.Intn(100), rng.Intn(80)
		best := math.Inf(1)
		for j := 0; j <= 20000; j++ {
			u := float32(j) / 20000
			dx, dy := float64(Quad(pts[0].X, pts[1].X, pts[2].X, u))-float64(x)-0.5, float64(Quad(pts[0].Y, pts[1].Y, pts[2].Y, u))-float64(y)-0.5
			best = min(best, math.Hypot(dx, dy))
		}
		want := int(math.Round(max(0, min(1, 0.5-(best-5))) * 255))
		got := alpha(img, x, y)
		if absInt(got-want) > 3 {
			t.Fatalf("(%d,%d): alpha %d, dense oracle %d", x, y, got, want)
		}
	}
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func TestHighlighterSelfOverlapAndSeparateStrokes(t *testing.T) {
	s := Stroke{Tool: Highlighter, Color: [4]float32{1, 0.5, 0, 0.5}, Points: []Point{{15, 40, 8}, {50, 40, 8}, {85, 40, 8}, {50, 40, 8}, {15, 40, 8}}}
	one := render(t, []Stroke{s}, 1, true).RGBAAt(50, 40)
	two := render(t, []Stroke{s, s}, 1, true).RGBAAt(50, 40)
	if absInt(int(one.G)-191) > 2 || absInt(int(one.B)-128) > 2 {
		t.Fatalf("self overlap compounded: %v", one)
	}
	if absInt(int(two.G)-143) > 2 || absInt(int(two.B)-64) > 2 {
		t.Fatalf("separate strokes didn't multiply separately: %v", two)
	}
	// Identical dimensions, different positions must retain independent masks.
	s.Points = []Point{{10, 15, 5}, {25, 15, 5}, {40, 15, 5}}
	s2 := s
	s2.Translation = [2]float32{40, 35}
	img := render(t, []Stroke{s, s2}, 1, true)
	if img.RGBAAt(25, 15).B > 130 || img.RGBAAt(65, 50).B > 130 {
		t.Fatal("mask reused across draws")
	}
}
func TestHighlighterPartialEdgeAndTransparentBackdrop(t *testing.T) {
	s := Stroke{Tool: Highlighter, Color: [4]float32{1, 0, 0, 0.5}, Points: []Point{{30.5, 30.5, 10}}}
	one := render(t, []Stroke{s}, 1, false)
	if got := alpha(one, 30, 30); absInt(got-128) > 1 {
		t.Fatalf("transparent composite alpha %d", got)
	}
	// Repeat identical partial edge segments in a single stroke: max, not over.
	s.Points = []Point{{30.5, 30.5, 10}, {30.5, 30.5, 10}, {30.5, 30.5, 10}, {30.5, 30.5, 10}}
	repeated := render(t, []Stroke{s}, 1, false)
	if a, b := alpha(one, 40, 30), alpha(repeated, 40, 30); a != b || absInt(a-64) > 1 {
		t.Fatalf("edge overlap: %d %d", a, b)
	}
}
func TestPencilLayoutAtlasAndFlips(t *testing.T) {
	rects, err := atlasRects()
	if err != nil {
		t.Fatal(err)
	}
	if len(rects) != 576 {
		t.Fatal(len(rects))
	}
	stamps := MakeStamps([]Point{{15, 35, 5}, {50, 15, 7}, {85, 40, 5}}, 42)
	again := MakeStamps([]Point{{15, 35, 5}, {50, 15, 7}, {85, 40, 5}}, 42)
	if len(stamps) < 5 || len(stamps) != len(again) {
		t.Fatal("arc sampling missing")
	}
	for i, s := range stamps {
		if s != again[i] || s[1][2] < 0 || s[1][2] >= 576 {
			t.Fatal("unstable or invalid stamp")
		}
	}
	// Explicit 2-texel record: rotation, translation, negative X scale.
	s := Stroke{Tool: Pencil, Color: [4]float32{0, 0, 0, 1}, Translation: [2]float32{10, 5}, Stamps: []Stamp{{{35, 30, 1, 0.5}, {-40, 20, 200, 0}}}}
	img := render(t, []Stroke{s}, 1, false)
	sum := 0
	for y := 10; y < 65; y++ {
		for x := 10; x < 85; x++ {
			sum += alpha(img, x, y)
		}
	}
	if sum < 100 {
		t.Fatal("atlas not sampled")
	}
}

func TestCubicTripleRootIsFinite(t *testing.T) {
	// B(t)=(t,t²). At query (0,0.5), its distance derivative has a
	// triple root at zero (p=q=0); this must not divide 0/0.
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	theme := ui.LightTheme()
	theme.Background = ui.Transparent
	img := ui.Render(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
			r.distance.Draw(p, ui.Rect{W: 3, H: 3}, shader.Options{Uniforms: map[string]any{
				"P0": [3]float32{0, 0, 0.5}, "P1": [3]float32{0.5, 0, 0.5}, "P2": [3]float32{1, 1, 0.5},
				"A": [2]float32{0.5, 0}, "B": [2]float32{0, 1}, "KK": 1, "KX": 0, "Height": 2, "Zoom": 1,
				"Origin": [2]float32{-0.5, -1}, "Color": [4]float32{0, 0, 0, 1},
			}})
		})
	}, 3, 3, 1)
	if got := alpha(img, 0, 0); absInt(got-128) > 1 {
		t.Fatalf("triple-root alpha=%d", got)
	}
}

func TestPenOverlapPreservesAnalyticEdge(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		center := float32(30) + 0.5/scale
		s := Stroke{Color: [4]float32{0, 0, 0, 1}, Points: []Point{{center, center, 10}}}
		one := render(t, []Stroke{s}, scale, false)
		s.Points = []Point{{center, center, 10}, {center, center, 10}, {center, center, 10}, {center, center, 10}, {center, center, 10}}
		overlap := render(t, []Stroke{s}, scale, false)
		x, y := int(40*scale), int(30*scale)
		if a, b := alpha(one, x, y), alpha(overlap, x, y); absInt(a-b) > 1 {
			t.Fatalf("DPR %g: same edge darkens with segment count: %d -> %d", scale, a, b)
		}
	}
}

func TestDensePenSamplesDoNotHardenDiagonalEdges(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		for _, opacity := range []float32{0.45, 1} {
			s := Stroke{Color: [4]float32{0.1, 0.2, 0.3, opacity}, Points: []Point{{10, 20.25, 3}, {50, 32.25, 3}, {90, 44.25, 3}}}
			sparse := render(t, []Stroke{s}, scale, false)
			s.Points = nil
			for x := 10; x <= 90; x += 2 {
				s.Points = append(s.Points, Point{float32(x), 20.25 + float32(x-10)*0.3, 3})
			}
			dense := render(t, []Stroke{s}, scale, false)
			for i, a := range sparse.Pix {
				if absInt(int(a)-int(dense.Pix[i])) > 2 {
					t.Fatalf("DPR %g opacity %g: sample density changed channel %d from %d to %d", scale, opacity, i, a, dense.Pix[i])
				}
			}
		}
	}
}
