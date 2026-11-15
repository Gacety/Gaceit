package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
	"time"
)

// Physical-pixel tiles, independent of document size. A small gutter prevents
// linear filtering and clipped brush edges from creating seams. Completed tiles
// are replayable full scenes; they never depend on a released incremental base.
const inkTileSize = 256
const inkTileGutter = 2
const inkTilePixels = inkTileSize + 2*inkTileGutter
const inkTileBudget = 32 << 20
const inkTileLimit = inkTileBudget / (inkTilePixels * inkTilePixels * 4)

type tileKey struct{ x, y int64 }
type inkTile struct {
	canvas  *ui.Canvas
	used    uint64
	patches uint8
	cyclic  bool
}
type tileCache struct {
	syncedRevision                 uint64
	synced                         bool
	warm                           tileWarmup
	prefetched                     uint64
	entries                        map[tileKey]inkTile
	scale                          float32
	phase                          [2]float32
	dark                           bool
	sources                        []panDrawable
	index                          panIndex
	clock, hits, misses, evictions uint64
}

func (t *tileCache) reset()                        { *t = tileCache{} }
func (t *tileCache) sync(a *Editor, scale float32) { t.syncMode(a, scale, true) }

// Rasterize at the viewport's physical subpixel phase, so a complete tile
// lands on integer texture pixels. Arbitrary zoom anchors must not resample
// each tile at a different phase from the copied viewport.
func (t *tileCache) syncFrame(a *Editor, c ink.Camera, repair bool) {
	phase := [2]float32{c.Offset[0] - float32(math.Floor(float64(c.Offset[0]))), c.Offset[1] - float32(math.Floor(float64(c.Offset[1])))}
	if t.phase != phase {
		// Pixel phase changes invalidate raster tiles, not immutable world
		// geometry or its spatial index.
		t.entries = map[tileKey]inkTile{}
		t.warm = tileWarmup{}
		t.phase = phase
	}
	if !repair && t.synced && t.syncedRevision == a.editRevision && len(t.sources) == len(a.strokes) && t.scale == c.Zoom && t.dark == a.dark && len(a.dirtyRegions) == 0 && !a.erasing && !a.selection.moving {
		return
	}
	t.syncMode(a, c.Zoom, !repair)
}

// syncForRepair updates source geometry and the spatial index but keeps the
// resident tiles. The repair path clears only dirty sub-rectangles, so a tile
// must still be available as its previous complete scene.
func (t *tileCache) syncForRepair(a *Editor, scale float32) { t.syncMode(a, scale, false) }

func (t *tileCache) syncMode(a *Editor, scale float32, invalidate bool) {
	defer a.ObservePhase("tile_sync", time.Now())
	if t.entries == nil || t.scale != scale || t.dark != a.dark {
		phase := t.phase
		t.reset()
		t.phase = phase
		t.entries = map[tileKey]inkTile{}
		t.scale = scale
		t.dark = a.dark
	}
	t.synced = true
	t.syncedRevision = a.editRevision
	changed := len(t.sources) != len(a.strokes)
	var nextSources []panDrawable
	ensureNext := func() {
		if nextSources == nil {
			nextSources = make([]panDrawable, len(a.strokes))
		}
	}
	matches := func(i, j int) bool {
		before := t.sources[i].stroke
		// Deterministic stamps prepared for imported pencils are a cache detail.
		if before.Tool == ink.Pencil && a.strokes[j].Stamps == nil {
			before.Stamps = nil
		}
		return sameStroke(before, a.strokes[j])
	}
	// Add/delete operations retain document order. Align unchanged headers so
	// removing an early stroke does not invalidate every later stroke's blocks.
	i, j := 0, 0
	for i < len(t.sources) && j < len(a.strokes) {
		if matches(i, j) {
			if changed {
				ensureNext()
			}
			if nextSources != nil {
				nextSources[j] = t.sources[i]
			}
			i++
			j++
			continue
		}
		if nextSources == nil {
			ensureNext()
			copy(nextSources[:j], t.sources[:i])
		}
		changed = true
		if len(t.sources) > len(a.strokes) {
			if invalidate {
				t.invalidate(t.sources[i].bounds)
			}
			i++
		} else if len(t.sources) < len(a.strokes) {
			if invalidate {
				t.invalidate(ink.StrokeBounds(a.strokes[j]))
			}
			j++
		} else {
			if invalidate {
				t.invalidate(t.sources[i].bounds)
				t.invalidate(ink.StrokeBounds(a.strokes[j]))
			}
			i++
			j++
		}
	}
	for ; i < len(t.sources); i++ {
		if invalidate {
			t.invalidate(t.sources[i].bounds)
		}
	}
	for ; j < len(a.strokes); j++ {
		if invalidate {
			t.invalidate(ink.StrokeBounds(a.strokes[j]))
		}
	}

	if !changed {
		return
	}
	ensureNext()
	for i, s := range a.strokes {
		cached := nextSources[i].stroke
		if cached.Tool == ink.Pencil && s.Stamps == nil {
			cached.Stamps = nil
		}
		if sameStroke(cached, s) {
			continue
		}
		if s.Tool == ink.Pencil && s.Stamps == nil {
			s.Stamps = ink.MakeStrokeStamps(s, 1)
		}
		nextSources[i] = panDrawable{stroke: s, bounds: ink.StrokeBounds(s)}
	}
	t.warm = tileWarmup{}
	t.index.update(t.sources, nextSources)
	t.sources = nextSources
}

// repair patches only the dirty sub-rectangles of resident tiles. The
// selected strokes' old and new bounds are the only regions cleared; unrelated
// pixels and strokes stay in the existing tile texture.
func unionRect(a, b ui.Rect) ui.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func (t *tileCache) repair(p *ui.Painter, a *Editor, regions []ui.Rect, c ink.Camera) {
	t.clock++
	jobs := map[tileKey][]ui.Rect{}
	for _, region := range regions {
		if region.W <= 0 || region.H <= 0 {
			continue
		}
		x0 := int64(math.Floor(float64((region.X - c.Offset[0] + t.phase[0]) / inkTileSize)))
		x1 := int64(math.Ceil(float64((region.X + region.W - c.Offset[0] + t.phase[0]) / inkTileSize)))
		y0 := int64(math.Floor(float64((region.Y - c.Offset[1] + t.phase[1]) / inkTileSize)))
		y1 := int64(math.Ceil(float64((region.Y + region.H - c.Offset[1] + t.phase[1]) / inkTileSize)))
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				k := tileKey{x, y}
				ox := float32(x*inkTileSize) + c.Offset[0] - t.phase[0] - inkTileGutter
				oy := float32(y*inkTileSize) + c.Offset[1] - t.phase[1] - inkTileGutter
				local := ui.Rect{X: region.X - ox, Y: region.Y - oy, W: region.W, H: region.H}
				local.X = max(0, local.X)
				local.Y = max(0, local.Y)
				local.W = min(float32(inkTilePixels)-local.X, local.W)
				local.H = min(float32(inkTilePixels)-local.Y, local.H)
				if local.W > 0 && local.H > 0 {
					jobs[k] = append(jobs[k], local)
				}
			}
		}
	}
	for k, rects := range jobs {
		// One bounded tile is at most 260px square, so coalescing its dirty
		// sub-rectangles avoids hundreds of tiny clip/query calls while keeping
		// the repair local to that tile.
		merged := rects[0]
		for _, r := range rects[1:] {
			merged = unionRect(merged, r)
		}
		rects = []ui.Rect{merged}
		old, ok := t.entries[k]
		if !ok || old.patches >= 8 || (old.cyclic && !old.canvas.Rendered()) {
			delete(t.entries, k)
			t.get(p, a, k)
			continue
		}
		cam := ink.Camera{Zoom: t.scale, PixelRatio: 1, Offset: [2]float32{
			t.phase[0] + inkTileGutter - float32(k.x*inkTileSize),
			t.phase[1] + inkTileGutter - float32(k.y*inkTileSize),
		}}
		next := ui.NewCanvas(inkTilePixels, inkTilePixels)
		next.Draw(p, func(cp *ui.Painter) {
			full := ui.Rect{W: inkTilePixels, H: inkTilePixels}
			if a.transparentInk {
				copyOutside(cp, old.canvas.Bitmap(), full, rects)
			} else {
				cp.Image(old.canvas.Bitmap(), full, ui.FillBox)
			}
			for _, r := range rects {
				cp.Clip(r, 0, func() {
					a.paintPaper(cp, r, cam)
					wb := [4]float32{
						(r.X - cam.Offset[0]) / t.scale,
						(r.Y - cam.Offset[1]) / t.scale,
						(r.X + r.W - cam.Offset[0]) / t.scale,
						(r.Y + r.H - cam.Offset[1]) / t.scale,
					}
					for _, id := range t.index.query(wb, t.sources) {
						a.renderer.DrawRegion(cp, a.renderStroke(t.sources[id].stroke), cam, r)
					}
				})
			}
		})
		t.entries[k] = inkTile{canvas: next, used: t.clock, patches: old.patches + 1, cyclic: old.cyclic}
	}
}

func (t *tileCache) invalidate(bounds [4]float32) {
	// Scan the bounded resident table, never all world cells in a long stroke.
	for k := range t.entries {
		b := t.worldBounds(k)
		if ink.BoundsOverlap(b, bounds) {
			delete(t.entries, k)
		}
	}
}
func (t *tileCache) worldBounds(k tileKey) [4]float32 {
	x, y := float32(k.x*inkTileSize), float32(k.y*inkTileSize)
	return [4]float32{(x - inkTileGutter - t.phase[0]) / t.scale, (y - inkTileGutter - t.phase[1]) / t.scale, (x + inkTileSize + inkTileGutter - t.phase[0]) / t.scale, (y + inkTileSize + inkTileGutter - t.phase[1]) / t.scale}
}
func (t *tileCache) get(p *ui.Painter, a *Editor, k tileKey) *ui.Canvas {
	t.clock++
	if v, ok := t.entries[k]; ok {
		if !v.cyclic || v.canvas.Rendered() {
			v.used = t.clock
			t.entries[k] = v
			t.hits++
			return v.canvas
		}
		delete(t.entries, k)
	}
	limit := inkTileLimit
	if t.warm.active {
		limit -= 2
	}
	if len(t.entries) >= limit {
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
	tile := ui.NewCanvas(inkTilePixels, inkTilePixels)
	c := ink.Camera{Zoom: t.scale, PixelRatio: 1, Offset: [2]float32{t.phase[0] + inkTileGutter - float32(k.x*inkTileSize), t.phase[1] + inkTileGutter - float32(k.y*inkTileSize)}}
	region := ui.Rect{W: inkTilePixels, H: inkTilePixels}
	tile.Draw(p, func(cp *ui.Painter) {
		a.paintPaper(cp, region, c)
		for _, id := range t.index.query(t.worldBounds(k), t.sources) {
			a.renderer.DrawRegion(cp, a.renderStroke(t.sources[id].stroke), c, region)
		}
	})
	t.entries[k] = inkTile{canvas: tile, used: t.clock}
	t.misses++
	return tile
}
func (t *tileCache) paint(p *ui.Painter, a *Editor, region ui.Rect, c ink.Camera) {
	// Coordinates are physical pixels at this cache's exact zoom and DPR.
	x0 := int64(math.Floor(float64((region.X - c.Offset[0] + t.phase[0]) / inkTileSize)))
	y0 := int64(math.Floor(float64((region.Y - c.Offset[1] + t.phase[1]) / inkTileSize)))
	x1 := int64(math.Ceil(float64((region.X + region.W - c.Offset[0] + t.phase[0]) / inkTileSize)))
	y1 := int64(math.Ceil(float64((region.Y + region.H - c.Offset[1] + t.phase[1]) / inkTileSize)))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			k := tileKey{x, y}
			tile := t.get(p, a, k)
			ox, oy := float32(x*inkTileSize)+c.Offset[0]-t.phase[0], float32(y*inkTileSize)+c.Offset[1]-t.phase[1]
			center := ui.Rect{X: ox, Y: oy, W: inkTileSize, H: inkTileSize}
			p.Clip(center, 0, func() {
				p.Image(tile.Bitmap(), ui.Rect{X: ox - inkTileGutter, Y: oy - inkTileGutter, W: inkTilePixels, H: inkTilePixels}, ui.FillBox)
			})
		}
	}
}
