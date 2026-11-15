package inkcanvas

import (
	"slices"
	"study408/internal/ink"
	"testing"
)

func TestPanIndexMatchesExactScanAndOrder(t *testing.T) {
	var sources []panDrawable
	for i := 0; i < 5000; i++ {
		x, y := float32(i%100)*80, float32(i/100)*80
		sources = append(sources, panDrawable{bounds: [4]float32{x, y, x + 70, y + 50}})
	}
	sources = append(sources, panDrawable{bounds: [4]float32{-1e8, -1e8, 1e8, 1e8}})
	var idx panIndex
	idx.build(sources)
	for _, b := range [][4]float32{{0, 0, 320, 240}, {-100, -100, 1, 1}, {400, 200, 410, 600}, {-1e7, -1e7, 1e7, 1e7}} {
		var want []int
		for i, s := range sources {
			if ink.BoundsOverlap(s.bounds, b) {
				want = append(want, i)
			}
		}
		if !slices.Equal(idx.query(b, sources), want) {
			t.Fatal("spatial query lost ink or changed paint order", b)
		}
	}
	if idx.entries > panIndexEntryBudget {
		t.Fatal("index exceeded budget")
	}
}
