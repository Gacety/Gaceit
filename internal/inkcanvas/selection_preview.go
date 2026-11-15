package inkcanvas

import (
	"math"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

// A selection drag is split into z-order runs: static ink before the
// selection, alternating selected/unselected runs, and ink after it. Pen runs
// are cached as small transparent images; highlighter runs draw directly so
// their composite mode stays exact. The result is the same paint order as a
// full vector render, without recompositing the base on every pointer sample.
type selectionPreviewLayer struct {
	canvas   *ui.Canvas
	dest     ui.Rect // screen DIPs at zero delta
	ids      []int
	selected bool
	direct   bool
}

const (
	maxSelectionPreviewLayers = 512
	maxSelectionPreviewBytes  = 32 << 20
)

func (a *Editor) buildSelectionPreview(p *ui.Painter, rect ui.Rect, c ink.Camera) bool {
	s := &a.selection
	if len(s.ids) == 0 || a.dry[a.dryFront] == nil || a.dryInvalid || len(a.dirtyRegions) > 0 || a.bakedCount != len(a.strokes) {
		return false
	}
	ratio := c.PixelRatio
	w, h := max(1, int(math.Ceil(float64(rect.W*ratio)))), max(1, int(math.Ceil(float64(rect.H*ratio))))
	if w != a.dryW || h != a.dryH {
		return false
	}
	a.tiles.syncFrame(a, newSurfaceFrame(rect, c).pixelCamera(), false)
	selected := make(map[int]bool, len(s.ids))
	minSelected := len(a.strokes)
	for _, id := range s.ids {
		if id < 0 || id >= len(a.strokes) {
			return false
		}
		selected[id] = true
		minSelected = min(minSelected, id)
	}
	if minSelected == len(a.strokes) {
		return false
	}

	// Moving every stroke needs no separation at all: the existing complete
	// dry image already is the selection layer. Use it without rasterising
	// thousands of vectors again; the base is only paper.
	if len(selected) == len(a.strokes) {
		base := ui.NewCanvas(w, h)
		pixelCamera := ink.Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}}
		base.Draw(p, func(cp *ui.Painter) {
			a.paintPaper(cp, ui.Rect{W: float32(w), H: float32(h)}, pixelCamera)
		})
		s.base = base
		s.layers = []selectionPreviewLayer{{canvas: a.dry[a.dryFront], dest: rect, ids: append([]int(nil), s.ids...), selected: true}}
		s.directRange = false
		s.selectionSet = selected
		s.previewRect, s.previewCamera, s.previewDark = rect, c, a.dark
		s.preview = true
		return true
	}

	// Runs cover every stroke from the first selected id onward. Before that,
	// all strokes are unselected and can be baked into the opaque base.
	runs := make([]selectionPreviewLayer, 0, 8)
	for i := minSelected; i < len(a.strokes); {
		sel := selected[i]
		j := i + 1
		for j < len(a.strokes) && selected[j] == sel {
			j++
		}
		runs = append(runs, selectionPreviewLayer{ids: append([]int(nil), makeRange(i, j)...), selected: sel})
		i = j
	}

	base := a.selectionBase(p, c, ratio, w, h, minSelected)
	bytes := w * h * 4
	useDirectRange := false
	for i := range runs {
		run := &runs[i]
		run.dest = a.selectionRunBounds(run.ids, c, ratio, rect)
		if run.dest.W <= 0 || run.dest.H <= 0 {
			continue
		}
		for _, id := range run.ids {
			if a.strokes[id].Tool == ink.Highlighter {
				run.direct = true
				break
			}
		}
		if run.direct {
			continue
		}
		lw := max(1, int(math.Ceil(float64(run.dest.W*ratio))))
		lh := max(1, int(math.Ceil(float64(run.dest.H*ratio))))
		if bytes+lw*lh*4 > maxSelectionPreviewBytes {
			useDirectRange = true
		}
		bytes += lw * lh * 4
	}
	if len(runs) > maxSelectionPreviewLayers {
		useDirectRange = true
	}
	if useDirectRange {
		s.base = base
		s.layers = nil
		s.directRange = true
		s.rangeStart = minSelected
		s.selectionSet = selected
		s.previewRect, s.previewCamera, s.previewDark = rect, c, a.dark
		s.preview = true
		return true
	}
	for i := range runs {
		run := &runs[i]
		if run.direct || run.canvas != nil || run.dest.W <= 0 || run.dest.H <= 0 {
			continue
		}
		lw := max(1, int(math.Ceil(float64(run.dest.W*ratio))))
		lh := max(1, int(math.Ceil(float64(run.dest.H*ratio))))
		layerCamera := ink.Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{
			c.Offset[0]*ratio - float32(math.Floor(float64(run.dest.X*ratio))),
			c.Offset[1]*ratio - float32(math.Floor(float64(run.dest.Y*ratio))),
		}}
		layer := ui.NewCanvas(lw, lh)
		layerRect := ui.Rect{W: float32(lw), H: float32(lh)}
		layer.Draw(p, func(cp *ui.Painter) {
			for _, id := range run.ids {
				a.renderer.DrawRegion(cp, a.renderStroke(a.strokes[id]), layerCamera, layerRect)
			}
		})
		run.canvas = layer
	}

	s.base = base
	s.layers = runs
	s.directRange = false
	s.selectionSet = selected
	s.previewRect, s.previewCamera, s.previewDark = rect, c, a.dark
	s.preview = true
	return true
}

// selectionBase keeps only strokes before the first selected index. When
// there is such a prefix, start from the current complete dry surface and
// clear the changed range, then restore only the prefix strokes. This avoids
// rasterising thousands of untouched strokes at drag start.
func (a *Editor) selectionBase(p *ui.Painter, c ink.Camera, ratio float32, w, h, minSelected int) *ui.Canvas {
	pixelCamera := ink.Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}}
	base := ui.NewCanvas(w, h)
	if minSelected == 0 {
		base.Draw(p, func(cp *ui.Painter) {
			a.paintPaper(cp, ui.Rect{W: float32(w), H: float32(h)}, pixelCamera)
		})
		return base
	}
	a.tiles.syncForRepair(a, c.Zoom*ratio)
	regions := map[tileKey]ui.Rect{}
	pixelBounds := func(id int) [4]float32 {
		b := ink.StrokeBounds(a.strokes[id])
		pad := float32(3) / c.Zoom
		return [4]float32{
			float32(math.Floor(float64((b[0]-pad)*c.Zoom+c.Offset[0]) * float64(ratio))),
			float32(math.Floor(float64((b[1]-pad)*c.Zoom+c.Offset[1]) * float64(ratio))),
			float32(math.Ceil(float64((b[2]+pad)*c.Zoom+c.Offset[0]) * float64(ratio))),
			float32(math.Ceil(float64((b[3]+pad)*c.Zoom+c.Offset[1]) * float64(ratio))),
		}
	}
	for id := minSelected; id < len(a.strokes); id++ {
		b := pixelBounds(id)
		x0 := int64(math.Floor(float64(b[0] / inkTileSize)))
		x1 := int64(math.Ceil(float64(b[2] / inkTileSize)))
		y0 := int64(math.Floor(float64(b[1] / inkTileSize)))
		y1 := int64(math.Ceil(float64(b[3] / inkTileSize)))
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				k := tileKey{x, y}
				ox, oy := float32(x*inkTileSize), float32(y*inkTileSize)
				r := ui.Rect{
					X: max(b[0], ox) - ox,
					Y: max(b[1], oy) - oy,
					W: min(b[2], ox+inkTileSize) - max(b[0], ox),
					H: min(b[3], oy+inkTileSize) - max(b[1], oy),
				}
				if r.W <= 0 || r.H <= 0 {
					continue
				}
				if old, ok := regions[k]; ok {
					regions[k] = unionRect(old, r)
				} else {
					regions[k] = r
				}
			}
		}
	}
	base.Draw(p, func(cp *ui.Painter) {
		full := ui.Rect{W: float32(w), H: float32(h)}
		if a.transparentInk {
			holes := make([]ui.Rect, 0, len(regions))
			for k, region := range regions {
				holes = append(holes, ui.Rect{X: float32(k.x*inkTileSize) + region.X, Y: float32(k.y*inkTileSize) + region.Y, W: region.W, H: region.H})
			}
			copyOutside(cp, a.dry[a.dryFront].Bitmap(), full, holes)
		} else {
			cp.Image(a.dry[a.dryFront].Bitmap(), full, ui.FillBox)
		}
		for k, region := range regions {
			ox, oy := float32(k.x*inkTileSize), float32(k.y*inkTileSize)
			rect := ui.Rect{X: ox + region.X, Y: oy + region.Y, W: region.W, H: region.H}
			cp.Clip(rect, 0, func() {
				a.paintPaper(cp, rect, pixelCamera)
				wb := [4]float32{
					(rect.X - pixelCamera.Offset[0]) / pixelCamera.Zoom,
					(rect.Y - pixelCamera.Offset[1]) / pixelCamera.Zoom,
					(rect.X + rect.W - pixelCamera.Offset[0]) / pixelCamera.Zoom,
					(rect.Y + rect.H - pixelCamera.Offset[1]) / pixelCamera.Zoom,
				}
				for _, id := range a.tiles.index.query(wb, a.tiles.sources) {
					if id < minSelected {
						a.renderer.DrawRegion(cp, a.renderStroke(a.tiles.sources[id].stroke), pixelCamera, rect)
					}
				}
			})
		}
	})
	return base
}

func makeRange(from, to int) []int {
	out := make([]int, to-from)
	for i := range out {
		out[i] = from + i
	}
	return out
}

func (a *Editor) selectionRunBounds(ids []int, c ink.Camera, ratio float32, viewport ui.Rect) ui.Rect {
	if len(ids) == 0 {
		return ui.Rect{}
	}
	b := ink.StrokeBounds(a.strokes[ids[0]])
	for _, id := range ids[1:] {
		n := ink.StrokeBounds(a.strokes[id])
		b = [4]float32{min(b[0], n[0]), min(b[1], n[1]), max(b[2], n[2]), max(b[3], n[3])}
	}
	pad := float32(4) / c.Zoom
	vw, vh := float32(math.Ceil(float64(viewport.W*ratio))), float32(math.Ceil(float64(viewport.H*ratio)))
	x0 := max(float32(0), float32(math.Floor(float64((b[0]-pad)*c.Zoom+c.Offset[0])*float64(ratio))))
	y0 := max(float32(0), float32(math.Floor(float64((b[1]-pad)*c.Zoom+c.Offset[1])*float64(ratio))))
	x1 := min(vw, float32(math.Ceil(float64((b[2]+pad)*c.Zoom+c.Offset[0])*float64(ratio))))
	y1 := min(vh, float32(math.Ceil(float64((b[3]+pad)*c.Zoom+c.Offset[1])*float64(ratio))))
	if x1 <= x0 || y1 <= y0 {
		return ui.Rect{}
	}
	return ui.Rect{X: x0 / ratio, Y: y0 / ratio, W: (x1 - x0) / ratio, H: (y1 - y0) / ratio}
}

func translatedDest(dest ui.Rect, delta ui.Point, zoom float32) ui.Rect {
	dest.X += delta.X * zoom
	dest.Y += delta.Y * zoom
	return dest
}

func (a *Editor) drawSelectionPreview(p *ui.Painter, rect ui.Rect, c ink.Camera) {
	a.paintSelectionPreview(p, rect, c, 1)
}

// The screen preview and committed bitmap use the very same layer compositor.
// A clipped raster run is only a cache of its original visible portion. When
// moving it, repair the newly revealed bands from vectors in the same frame.
func (a *Editor) paintSelectionPreview(p *ui.Painter, rect ui.Rect, c ink.Camera, rasterScale float32) {
	s := &a.selection
	if s.base == nil {
		return
	}
	p.Image(s.base.Bitmap(), rect, ui.FillBox)
	draw := func(stroke ink.Stroke, selected bool, region ui.Rect) {
		cam := c
		if selected {
			cam.Offset[0] += s.delta.X * c.Zoom
			cam.Offset[1] += s.delta.Y * c.Zoom
		}
		stroke.Color = AdaptiveColor(stroke.Color, a.dark)
		a.renderer.DrawRegion(p, a.renderStroke(stroke), cam, region)
	}
	if s.directRange {
		for id := s.rangeStart; id < len(s.before); id++ {
			draw(s.before[id], s.selectionSet[id], rect)
		}
		return
	}
	for _, layer := range s.layers {
		dest := ui.Rect{X: layer.dest.X * rasterScale, Y: layer.dest.Y * rasterScale, W: layer.dest.W * rasterScale, H: layer.dest.H * rasterScale}
		if layer.selected {
			dest = translatedDest(dest, s.delta, c.Zoom)
		}
		if layer.direct {
			for _, id := range layer.ids {
				draw(s.before[id], layer.selected, rect)
			}
			continue
		}
		if layer.canvas != nil {
			p.Image(layer.canvas.Bitmap(), dest, ui.FillBox)
		}
		if !layer.selected || len(layer.ids) == 0 {
			continue
		}
		moved := c
		moved.Offset[0] += s.delta.X * c.Zoom
		moved.Offset[1] += s.delta.Y * c.Zoom
		first, last := layer.ids[0], layer.ids[len(layer.ids)-1]
		for _, band := range outsideBands(rect, dest) {
			p.Clip(band, 0, func() {
				wb := [4]float32{(band.X - moved.Offset[0]) / moved.Zoom, (band.Y - moved.Offset[1]) / moved.Zoom, (band.X + band.W - moved.Offset[0]) / moved.Zoom, (band.Y + band.H - moved.Offset[1]) / moved.Zoom}
				for _, id := range a.tiles.index.query(wb, a.tiles.sources) {
					if id >= first && id <= last {
						draw(a.tiles.sources[id].stroke, true, band)
					}
				}
			})
		}
	}
}

// commitSelectionPreview is called from Draw because Canvas.Draw needs a
// Painter. It commits translated vectors and then composes the cached z-order
// runs at the final delta; no full-view vector rebuild is needed.
func (a *Editor) commitSelectionPreview(p *ui.Painter) bool {
	s := &a.selection
	if !s.preview || s.base == nil {
		return false
	}
	if s.delta == (ui.Point{}) {
		s.resetPreview()
		s.moving = false
		s.before = nil
		return true
	}
	for _, id := range s.ids {
		a.strokes[id] = translatedStroke(s.before[id], s.delta)
		a.strokes[id].Color = AdaptiveColor(a.strokes[id].Color, a.dark)
	}
	a.remember(s.before)
	a.updateSelectionBounds()

	w, h := s.base.Size()
	final := ui.NewCanvas(w, h)
	ratio := s.previewCamera.PixelRatio
	pixelCamera := ink.Camera{Zoom: s.previewCamera.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{
		s.previewCamera.Offset[0] * ratio,
		s.previewCamera.Offset[1] * ratio,
	}}
	final.Draw(p, func(cp *ui.Painter) {
		a.paintSelectionPreview(cp, ui.Rect{W: float32(w), H: float32(h)}, pixelCamera, ratio)
	})
	a.publishDry([2]*ui.Canvas{final, nil}, 0, newSurfaceFrame(s.previewRect, s.previewCamera), len(a.strokes))
	a.dryInvalid = false
	a.dirtyRegions = nil
	a.panCache.reset()
	a.tiles.sync(a, s.previewCamera.Zoom*s.previewCamera.PixelRatio)
	a.keepIdleSurface(final, s.previewRect, s.previewRect, s.previewCamera)
	s.resetPreview()
	s.moving = false
	s.before = nil
	return true
}
