package ink

import "testing"

func TestHitIndexSkipsDistantStrokesAndTracksWholeIDs(t *testing.T) {
	strokes := make([]Stroke, 1000)
	for i := range strokes {
		y := float32(i * 100)
		strokes[i] = Stroke{Points: []Point{{10, y, 2}, {50, y, 2}, {90, y, 2}}}
	}
	index := NewHitIndex(strokes)
	removed := map[int]bool{}
	hits := index.HitsSweep(Point{X: 40, Y: 49990, Radius: 3}, Point{X: 40, Y: 50010, Radius: 3}, removed)
	if len(hits) != 1 || hits[0] != 500 {
		t.Fatalf("wrong indexed selection %v", hits)
	}
	if index.CandidateTests > 10 {
		t.Fatalf("queried %d distant segments", index.CandidateTests)
	}
	removed[500] = true
	if len(index.HitsSweep(Point{X: 40, Y: 49990, Radius: 3}, Point{X: 40, Y: 50010, Radius: 3}, removed)) != 0 {
		t.Fatal("removed stroke selected again")
	}
}
func TestHitIndexCurveDotPencilAndLongSweep(t *testing.T) {
	cases := []Stroke{
		{Points: []Point{{10, 60, 2}, {50, 0, 8}, {90, 60, 2}}},
		{Points: []Point{{50, 30, 5}}},
		{Tool: Pencil, Stamps: []Stamp{{{50, 30, 1, 0}, {20, 10, 200, 0}}}},
	}
	for _, s := range cases {
		index := NewHitIndex([]Stroke{s})
		if got := index.HitsSweep(Point{X: 50, Y: -20, Radius: 2}, Point{X: 50, Y: 100, Radius: 2}, nil); len(got) != 1 {
			t.Fatal("crossing gesture missed")
		}
		if got := index.HitsSweep(Point{X: 500, Y: 500, Radius: 2}, Point{X: 600, Y: 600, Radius: 2}, nil); len(got) != 0 {
			t.Fatal("far gesture selected stroke")
		}
	}
	s := Stroke{Points: []Point{{0, 0, 2}, {10, 0, 2}, {20, 0, 2}}}
	if len(NewHitIndex([]Stroke{s}).HitsSweep(Point{X: -1e6, Y: 0, Radius: 1}, Point{X: 1e6, Y: 0, Radius: 1}, nil)) != 1 {
		t.Fatal("large sweep fallback missed")
	}
}
