package ink

import "testing"

func TestSampleCurvesConnectSparseCorners(t *testing.T) {
	points := []Point{{10, 10, 2}, {80, 70, 4}, {30, 130, 3}, {90, 180, 2}}
	curves := SampleCurves(points)
	if curves[0][0] != points[0] || curves[len(curves)-1][2] != points[len(points)-1] {
		t.Fatal("input endpoints lost")
	}
	for i := 1; i < len(curves); i++ {
		prev, next := curves[i-1], curves[i]
		if prev[2] != next[0] {
			t.Fatalf("join %d doesn't share position and radius", i)
		}
		dx, dy := prev[2].X-prev[1].X, prev[2].Y-prev[1].Y
		ex, ey := next[1].X-next[0].X, next[1].Y-next[0].Y
		if dx*ex+dy*ey <= 0 || dx*ey-dy*ex != 0 {
			t.Fatalf("join %d has a tangent cusp", i)
		}
		if i > 1 && i < len(curves)-1 && (dx != ex || dy != ey || prev[2].Radius-prev[1].Radius != next[1].Radius-next[0].Radius) {
			t.Fatal("interior derivatives differ")
		}
	}
	// The turn uses the sample as a control, without drawing a cap over it.
	mid := curves[1]
	x, y := Quad(mid[0].X, mid[1].X, mid[2].X, 0.5), Quad(mid[0].Y, mid[1].Y, mid[2].Y, 0.5)
	if x >= points[1].X || y <= points[0].Y {
		t.Fatal("sparse turn not rounded")
	}
}
func TestSampleCurvesEmptyDotAndLine(t *testing.T) {
	if len(SampleCurves(nil)) != 0 {
		t.Fatal("empty input")
	}
	for n := 1; n <= 2; n++ {
		p := []Point{{10, 20, 3}, {30, 40, 4}}[:n]
		curves := SampleCurves(p)
		if len(curves) != 1 || curves[0][0] != p[0] || curves[0][2] != p[n-1] {
			t.Fatal("short input endpoints changed")
		}
	}
}
func TestSmoothStrokeRoundsTurnAndKeepsEndpointCaps(t *testing.T) {
	s := Stroke{Smooth: true, Points: []Point{{15, 15, 3}, {75, 65, 3}, {15, 65, 3}}, Color: [4]float32{0, 0, 0, 1}}
	rounded := render(t, []Stroke{s}, 1, false)
	if alpha(rounded, 15, 15) < 240 || alpha(rounded, 15, 65) < 240 {
		t.Fatal("endpoint caps lost")
	}
	if alpha(rounded, 75, 65) != 0 {
		t.Fatal("raw corner still carries a protruding endpoint cap")
	}
	// Interior curve B(0.5) = (60,58.75), rather than a sharp join at (75,65).
	if alpha(rounded, 60, 58) < 240 {
		t.Fatal("rounded turn missing")
	}
}
func TestSmoothPencilUsesSameChain(t *testing.T) {
	s := Stroke{Smooth: true, Points: []Point{{10, 10, 2}, {80, 70, 3}, {20, 130, 2}}}
	stamps := MakeStrokeStamps(s, 1)
	if len(stamps) < 3 {
		t.Fatal("missing smooth stamps")
	}
	for _, stamp := range stamps {
		// The smooth chain never reaches the raw control's X=80.
		if stamp[0][0] > 65.1 {
			t.Fatal("pencil still follows overlapping raw windows")
		}
	}
}
