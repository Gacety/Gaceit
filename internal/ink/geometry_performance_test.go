package ink

import (
	"reflect"
	"testing"
)

func performanceStroke() Stroke {
	points := make([]Point, 1600)
	for i := range points {
		points[i] = Point{X: float32(i) / 3, Y: float32(i % 19), Radius: 1.5 + float32(i%5)/4}
	}
	return Stroke{Points: points, Smooth: true, Translation: [2]float32{20, 30}}
}
func referenceSmoothSegments(s Stroke, pixel float32) []Segment {
	var out []Segment
	for _, curve := range SampleCurves(s.Points) {
		out = append(out, Segments(curve[:], s.Translation, pixel)...)
	}
	return out
}
func TestPreparedStrokeGeometryUnchanged(t *testing.T) {
	s := performanceStroke()
	for _, n := range []int{0, 1, 2, 3, 4, 100, 1600} {
		s.Points = s.Points[:n]
		for _, pixel := range []float32{.25, .5, 1, 2} {
			got := strokeSegments(s, pixel)
			want := referenceSmoothSegments(s, pixel)
			if !reflect.DeepEqual(got, want) {
				t.Fatal("stroke geometry changed", n, pixel)
			}
		}
		s = performanceStroke()
	}
}
func BenchmarkPrepareStrokeSegments(b *testing.B) {
	s := performanceStroke()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strokeSegments(s, .5)
	}
}
