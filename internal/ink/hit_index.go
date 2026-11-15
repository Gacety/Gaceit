package ink

import "math"

type hitChord struct {
	a, b   Point
	bounds [4]float32
	stroke int
}
type hitCell struct{ x, y int64 }

// HitIndex prepares selection geometry once per erasing gesture. A spatial
// grid skips unrelated strokes; curves are adaptively flattened, not sampled
// again on every pointer event. Selection tolerance is 0.25 model units.
type HitIndex struct {
	chords         []hitChord
	cells          map[hitCell][]int
	large          []int
	seen           []uint32
	version        uint32
	entries        int
	CandidateTests int
}

func NewHitIndex(strokes []Stroke) *HitIndex {
	h := &HitIndex{cells: map[hitCell][]int{}}
	for id, s := range strokes {
		if s.Tool == Pencil {
			stamps := s.Stamps
			if stamps == nil {
				stamps = MakeStrokeStamps(s, 1)
			}
			for _, stamp := range stamps {
				x, y, hw, hh := stampBounds(stamp)
				p := Point{X: x + s.Translation[0], Y: y + s.Translation[1], Radius: max(hw, hh)}
				h.add(id, p, p, 0)
			}
			continue
		}
		for _, seg := range strokeSegments(s, 1) {
			height := seg.Bounds[3] / 2
			model := func(p Point) Point {
				return Point{X: (p.X+1)*height + seg.Bounds[0], Y: (p.Y+1)*height + seg.Bounds[1], Radius: p.Radius * height}
			}
			a, b, c := model(seg.P[0]), model(seg.P[1]), model(seg.P[2])
			if seg.Mode == 2 {
				if c.Radius > a.Radius {
					a = c
				}
				h.add(id, a, a, 0)
				continue
			}
			if seg.Mode == 1 {
				b.X = (a.X + c.X) / 2
				b.Y = (a.Y + c.Y) / 2
			}
			h.flatten(id, a, b, c, 0)
		}
	}
	h.seen = make([]uint32, len(h.chords))
	return h
}
func (h *HitIndex) flatten(id int, a, b, c Point, depth int) {
	flat := pointSegmentDistance(b, a, c)
	radiusError := float32(math.Abs(float64(b.Radius-(a.Radius+c.Radius)/2))) / 2
	if depth >= 12 || flat <= 0.125 && radiusError <= 0.125 {
		h.add(id, a, c, max(float32(0.25), flat/2+radiusError))
		return
	}
	ab, bc := midpoint(a, b), midpoint(b, c)
	mid := midpoint(ab, bc)
	h.flatten(id, a, ab, mid, depth+1)
	h.flatten(id, mid, bc, c, depth+1)
}
func (h *HitIndex) add(id int, a, b Point, padding float32) {
	a.Radius += padding
	b.Radius += padding
	chord := hitChord{a: a, b: b, bounds: SweepBounds([]Point{a, b}), stroke: id}
	index := len(h.chords)
	h.chords = append(h.chords, chord)
	x0, y0, x1, y1, ok := hitCells(chord.bounds)
	if !ok || h.entries+int((x1-x0+1)*(y1-y0+1)) > 1000000 {
		h.large = append(h.large, index)
		return
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			key := hitCell{x, y}
			h.cells[key] = append(h.cells[key], index)
			h.entries++
		}
	}
}
func hitCells(bounds [4]float32) (x0, y0, x1, y1 int64, ok bool) {
	for _, v := range bounds {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 1e14 {
			return
		}
	}
	x0, y0, x1, y1 = int64(math.Floor(float64(bounds[0])/128)), int64(math.Floor(float64(bounds[1])/128)), int64(math.Floor(float64(bounds[2])/128)), int64(math.Floor(float64(bounds[3])/128))
	ok = x1 >= x0 && y1 >= y0 && float64(x1-x0+1)*float64(y1-y0+1) <= 4096
	return
}
func (h *HitIndex) HitsSweep(from, to Point, removed map[int]bool) []int {
	h.CandidateTests = 0
	h.version++
	if h.version == 0 {
		clear(h.seen)
		h.version = 1
	}
	bounds := SweepBounds([]Point{from, to})
	found := map[int]bool{}
	var out []int
	test := func(i int) {
		if h.seen[i] == h.version {
			return
		}
		h.seen[i] = h.version
		c := h.chords[i]
		if removed[c.stroke] || found[c.stroke] || !BoundsOverlap(bounds, c.bounds) {
			return
		}
		h.CandidateTests++
		if segmentDistance(from, to, c.a, c.b) <= max(from.Radius, to.Radius)+max(c.a.Radius, c.b.Radius) {
			found[c.stroke] = true
			out = append(out, c.stroke)
		}
	}
	x0, y0, x1, y1, ok := hitCells(bounds)
	if !ok {
		for i := range h.chords {
			test(i)
		}
	} else {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				for _, i := range h.cells[hitCell{x, y}] {
					test(i)
				}
			}
		}
		for _, i := range h.large {
			test(i)
		}
	}
	return out
}
