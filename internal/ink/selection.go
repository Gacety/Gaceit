package ink

import "slices"

// SelectPolygon returns whole strokes enclosed or crossed by a closed lasso.
// Reuse the eraser's adaptive chords; no per-pointer-event curve resampling.
func (h *HitIndex) SelectPolygon(polygon []Point) []int {
	if len(polygon) < 3 {
		return nil
	}
	bounds := SweepBounds(polygon)
	found := map[int]bool{}
	for _, c := range h.chords {
		if found[c.stroke] || !BoundsOverlap(bounds, c.bounds) {
			continue
		}
		if polygonContains(polygon, c.a) || polygonContains(polygon, c.b) {
			found[c.stroke] = true
			continue
		}
		for i, p := range polygon {
			q := polygon[(i+1)%len(polygon)]
			if segmentDistance(p, q, c.a, c.b) <= max(c.a.Radius, c.b.Radius) {
				found[c.stroke] = true
				break
			}
		}
	}
	out := make([]int, 0, len(found))
	for id := range found {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func polygonContains(polygon []Point, p Point) bool {
	inside := false
	j := len(polygon) - 1
	for i, a := range polygon {
		b := polygon[j]
		if (a.Y > p.Y) != (b.Y > p.Y) && float64(p.X) < float64(a.X)+(float64(p.Y)-float64(a.Y))*(float64(b.X)-float64(a.X))/(float64(b.Y)-float64(a.Y)) {
			inside = !inside
		}
		j = i
	}
	return inside
}
