package ink

import "github.com/egoist/mygo/ui"

// Reuse only small complete stroke masks. Unlike viewport screenshots these
// contain every pixel of their stroke, so panning cannot reveal blank edges.
// This is a fixed 4 MiB pixel budget, not a cache of visited canvas regions.
const maxMaskPixels = 1 << 20
const maxMaskEntries = 256
const maxMaskSegments = 8192
const maxSingleMaskPixels = 256 * 256

type maskKey struct {
	segment       segmentKey
	offset        [2]float32
	width, height int
}
type cachedMask struct {
	key      maskKey
	canvas   *ui.Canvas
	segments int
}
type maskCache struct {
	entries                map[maskKey]*ui.Canvas
	order                  []cachedMask
	head, pixels, segments int
}

func (r *Renderer) strokeMask(p *ui.Painter, s Stroke, segs []Segment, c Camera, w, h int, cache bool) *ui.Canvas {
	key := maskKey{segment: segmentKey{&s.Points[0], len(s.Points), s.Smooth, s.Translation, 1 / c.Zoom}, offset: c.Offset, width: w, height: h}
	cache = cache && len(segs) <= maxMaskSegments
	m := &r.masks
	if cache {
		if found := m.entries[key]; found != nil {
			return found
		}
	}
	cv := ui.NewCanvas(w, h)
	cv.Draw(p, func(cp *ui.Painter) {
		r.drawSegments(cp, segs, s.Color, c, true, ui.Rect{W: float32(w), H: float32(h)})
	})
	if !cache {
		return cv
	}
	if m.entries == nil {
		m.entries = map[maskKey]*ui.Canvas{}
	}
	for len(m.order)-m.head >= maxMaskEntries || m.pixels+w*h > maxMaskPixels || m.segments+len(segs) > maxMaskSegments {
		old := m.order[m.head]
		delete(m.entries, old.key)
		m.pixels -= old.key.width * old.key.height
		m.segments -= old.segments
		m.order[m.head] = cachedMask{}
		m.head++
	}
	if m.head > 128 {
		n := copy(m.order, m.order[m.head:])
		clear(m.order[n:])
		m.order = m.order[:n]
		m.head = 0
	}
	m.entries[key] = cv
	m.order = append(m.order, cachedMask{key: key, canvas: cv, segments: len(segs)})
	m.pixels += w * h
	m.segments += len(segs)
	return cv
}
