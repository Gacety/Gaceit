package ink

// Completed strokes share immutable samples. Bound prepared geometry by both
// entry count and segment count; no viewport-sized images are kept here.
type segmentKey struct {
	first       *Point
	count       int
	smooth      bool
	translation [2]float32
	pixel       float32
}
type preparedSegments struct {
	key      segmentKey
	segments []Segment
}
type segmentCache struct {
	entries     map[segmentKey][]Segment
	order       []preparedSegments
	head, total int
}

const maxPreparedSegments = 16384
const maxPreparedStrokes = 256

func (r *Renderer) completedSegments(s Stroke, pixel float32) []Segment {
	if len(s.Points) == 0 {
		return nil
	}
	key := segmentKey{&s.Points[0], len(s.Points), s.Smooth, s.Translation, pixel}
	c := &r.geometry
	if v, ok := c.entries[key]; ok {
		return v
	}
	segments := strokeSegments(s, pixel)
	if len(segments) > maxPreparedSegments {
		return segments
	}
	if c.entries == nil {
		c.entries = map[segmentKey][]Segment{}
	}
	for len(c.order)-c.head >= maxPreparedStrokes || c.total+len(segments) > maxPreparedSegments {
		old := c.order[c.head]
		delete(c.entries, old.key)
		c.total -= len(old.segments)
		c.order[c.head] = preparedSegments{}
		c.head++
	}
	if c.head > 128 {
		n := copy(c.order, c.order[c.head:])
		clear(c.order[n:])
		c.order = c.order[:n]
		c.head = 0
	}
	c.entries[key] = segments
	c.order = append(c.order, preparedSegments{key, segments})
	c.total += len(segments)
	return segments
}
func (r *Renderer) ReleaseGeometryCache() { r.geometry = segmentCache{}; r.masks = maskCache{} }
