package inkcanvas

import (
	"math"
	"slices"
	"study408/internal/ink"
)

// Only compact stroke IDs/bounds are indexed, never additional raster tiles.
// Oversized paths and entries past the budget fall back to exact bounds tests.
const panIndexCell = 128
const panIndexEntryBudget = 131072

type panCell struct{ x, y int64 }
type panIndex struct {
	cells      map[panCell][]int
	large      []int
	seen       []uint32
	generation uint32
	candidates []int
	entries    int
}

func panCellRange(b [4]float32) (int64, int64, int64, int64) {
	const capCoordinate = 1e12
	clamp := func(v float32) int64 {
		return int64(math.Floor(max(-capCoordinate, min(capCoordinate, float64(v))) / panIndexCell))
	}
	return clamp(b[0]), clamp(b[1]), clamp(b[2]), clamp(b[3])
}
func (idx *panIndex) build(sources []panDrawable) {
	*idx = panIndex{cells: map[panCell][]int{}, seen: make([]uint32, len(sources))}
	for id, source := range sources {
		x0, y0, x1, y1 := panCellRange(source.bounds)
		w, h := x1-x0+1, y1-y0+1
		if w <= 0 || h <= 0 || w > 64 || h > 64 || w*h > 64 || idx.entries+int(w*h) > panIndexEntryBudget {
			idx.large = append(idx.large, id)
			continue
		}
		for x := x0; x <= x1; x++ {
			for y := y0; y <= y1; y++ {
				cell := panCell{x, y}
				idx.cells[cell] = append(idx.cells[cell], id)
				idx.entries++
			}
		}
	}
}
func (idx *panIndex) query(bounds [4]float32, sources []panDrawable) []int {
	idx.candidates = idx.candidates[:0]
	idx.generation++
	if idx.generation == 0 {
		clear(idx.seen)
		idx.generation = 1
	}
	add := func(id int) {
		if idx.seen[id] != idx.generation {
			idx.seen[id] = idx.generation
			if ink.BoundsOverlap(sources[id].bounds, bounds) {
				idx.candidates = append(idx.candidates, id)
			}
		}
	}
	x0, y0, x1, y1 := panCellRange(bounds)
	if x1-x0 > 128 || y1-y0 > 128 {
		for id := range sources {
			add(id)
		}
	} else {
		for x := x0; x <= x1; x++ {
			for y := y0; y <= y1; y++ {
				for _, id := range idx.cells[panCell{x, y}] {
					add(id)
				}
			}
		}
		for _, id := range idx.large {
			add(id)
		}
	}
	// Preserve document paint order for translucent ink and overlapping strokes.
	slices.Sort(idx.candidates)
	return idx.candidates
}
