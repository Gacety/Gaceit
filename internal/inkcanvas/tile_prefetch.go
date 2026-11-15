package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
	"time"
)

type tileWarmup struct {
	key         tileKey
	layers      [2]*ui.Canvas
	front, next int
	ids         []int
	active      bool
	wrapped     bool
	waiting     bool
}

// Warm one adjacent tile in bounded batches after the visible frame is ready.
// Visible misses always remain synchronous: navigation never shows blank strips.
func (t *tileCache) prefetch(p *ui.Painter, a *Editor, f surfaceFrame) {
	if t.scale != f.camera.Zoom*f.camera.PixelRatio || len(t.sources) != len(a.strokes) || t.entries == nil {
		return
	}
	b := f.visible()
	x0, y0 := int64(math.Floor(float64(b[0]*t.scale/inkTileSize))), int64(math.Floor(float64(b[1]*t.scale/inkTileSize)))
	x1, y1 := int64(math.Ceil(float64(b[2]*t.scale/inkTileSize))), int64(math.Ceil(float64(b[3]*t.scale/inkTileSize)))
	if (x1-x0+2)*(y1-y0+2) > inkTileLimit/2 {
		t.warm = tileWarmup{}
		return
	}
	w := &t.warm
	if w.active && (w.key.x < x0-1 || w.key.x > x1 || w.key.y < y0-1 || w.key.y > y1) {
		*w = tileWarmup{}
	}
	if w.active {
		if _, ok := t.entries[w.key]; ok {
			*w = tileWarmup{}
		}
	}
	if !w.active {
		found := false
		for y := y0 - 1; y <= y1 && !found; y++ {
			for x := x0 - 1; x <= x1; x++ {
				if x >= x0 && x < x1 && y >= y0 && y < y1 {
					continue
				}
				key := tileKey{x, y}
				if _, ok := t.entries[key]; ok {
					continue
				}
				candidates := t.index.query(t.worldBounds(key), t.sources)
				expensive := false
				for _, id := range candidates {
					if strokeWork(t.sources[id].stroke, navigationBakeWorkLimit) > navigationBakeWorkLimit {
						expensive = true
						break
					}
				}
				if expensive {
					continue
				} // speculative work must never submit an oversized atomic stroke
				ids := append([]int(nil), candidates...)
				*w = tileWarmup{key: key, ids: ids, active: true}
				found = true
				break
			}
		}
		if !found {
			return
		}
	}
	// Reserve both warm targets inside the existing total pixel/table budget.
	for len(t.entries) > inkTileLimit-2 {
		t.evict()
	}
	if w.next > 0 {
		previous := w.layers[w.front]
		if previous == nil || !previous.Rendered() {
			if previous != nil && !w.waiting && !w.wrapped {
				previous.Flush(p)
				w.waiting = true
				p.After(8 * time.Millisecond)
				return
			}
			*w = tileWarmup{}
			return
		}
	}
	w.waiting = false
	next := 0
	if w.next > 0 {
		next = 1 - w.front
		if next == 0 {
			w.wrapped = true
		}
	}
	if w.layers[next] == nil {
		w.layers[next] = ui.NewCanvas(inkTilePixels, inkTilePixels)
	}
	start, end := w.next, w.next
	units := 0
	for end < len(w.ids) && end-start < navigationBakeBatch {
		cost := strokeWork(t.sources[w.ids[end]].stroke, navigationBakeWorkLimit)
		if end > start && units+cost > navigationBakeWorkLimit {
			break
		}
		units += cost
		end++
	}
	c := ink.Camera{Zoom: t.scale, PixelRatio: 1, Offset: [2]float32{t.phase[0] + inkTileGutter - float32(w.key.x*inkTileSize), t.phase[1] + inkTileGutter - float32(w.key.y*inkTileSize)}}
	region := ui.Rect{W: inkTilePixels, H: inkTilePixels}
	previous := w.layers[w.front]
	w.layers[next].Draw(p, func(cp *ui.Painter) {
		if start == 0 {
			a.paintPaper(cp, region, c)
		} else {
			cp.Image(previous.Bitmap(), region, ui.FillBox)
		}
		for _, id := range w.ids[start:end] {
			a.renderer.DrawRegion(cp, a.renderStroke(t.sources[id].stroke), c, region)
		}
	})
	w.front = next
	w.next = end
	if end == len(w.ids) {
		t.clock++
		t.entries[w.key] = inkTile{canvas: w.layers[next], used: t.clock, cyclic: w.wrapped}
		*w = tileWarmup{}
		t.prefetched++
	}
	p.After(8 * time.Millisecond)
}
func (t *tileCache) evict() {
	var oldest tileKey
	age := ^uint64(0)
	for key, v := range t.entries {
		if v.used < age {
			oldest, age = key, v.used
		}
	}
	delete(t.entries, oldest)
	t.evictions++
}
