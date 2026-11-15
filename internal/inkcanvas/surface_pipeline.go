package inkcanvas

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

// Disposable rendering state. It owns no document samples, edit history or
// camera input. Dropping every cache can never remove a user's stroke.
// Embedded in Editor so interaction code can mark damage without owning the
// publication/recovery protocol.
type surfaceState struct {
	transparentInk       bool
	paper                paperSurface
	tiles                tileCache
	idleSurface          idleSurface
	navigationBake       navigationBake
	live                 ink.LiveStroke
	erasePreview         ink.ErasePreview
	dry                  [2]*ui.Canvas
	dryFront, bakedCount int
	dryW, dryH           int
	dryRatio             float32
	dryInvalid           bool
	dryCyclic            bool
	dryCamera            ink.Camera
	dryReusesPan         bool
	dirtyRegions         [][4]float32
	panCache             panSurface
	zoomPreview          bool
	recompose            bool
	recomposeGeneration  uint64
}

// A frame describes the requested view, never the content of a texture. All
// raster paths use these same conversions (DIPs -> physical pixels -> world).
type surfaceFrame struct {
	rect   ui.Rect
	camera ink.Camera
	w, h   int
}

func newSurfaceFrame(r ui.Rect, c ink.Camera) surfaceFrame {
	return surfaceFrame{r, c, max(1, int(math.Ceil(float64(r.W*c.PixelRatio)))), max(1, int(math.Ceil(float64(r.H*c.PixelRatio))))}
}
func (f surfaceFrame) pixelCamera() ink.Camera {
	c := f.camera
	return ink.Camera{Zoom: c.Zoom * c.PixelRatio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * c.PixelRatio, c.Offset[1] * c.PixelRatio}}
}
func (f surfaceFrame) visible() [4]float32 {
	c := f.camera
	return [4]float32{-c.Offset[0] / c.Zoom, -c.Offset[1] / c.Zoom, (f.rect.W - c.Offset[0]) / c.Zoom, (f.rect.H - c.Offset[1]) / c.Zoom}
}
func (f surfaceFrame) pixels() ui.Rect { return ui.Rect{W: float32(f.w), H: float32(f.h)} }

// Only complete scenes may be published/retained. A navigation bake's partial
// targets are private until completion. A pan scene can have an overscan
// destination; its camera always describes the visible viewport.
type surfaceView struct {
	source                *ui.Canvas
	destination, viewport ui.Rect
	camera                ink.Camera
	count                 int
}

func (a *Editor) dryView(f surfaceFrame) surfaceView {
	return surfaceView{a.dry[a.dryFront], f.rect, f.rect, a.dryCamera, a.bakedCount}
}
func (a *Editor) presentSurface(p *ui.Painter, v surfaceView) {
	p.Clip(v.viewport, 0, func() { p.Image(v.source.Bitmap(), v.destination, ui.FillBox) })
	a.retainSurface(v)
}

// publishDry is the single handoff for a complete viewport. Keep metadata and
// the canvas together; no caller may publish a partly drawn replacement.
func (a *Editor) publishDry(layers [2]*ui.Canvas, front int, f surfaceFrame, count int) {
	a.dry, a.dryFront = layers, front
	a.dryW, a.dryH, a.dryRatio, a.dryCamera = f.w, f.h, f.camera.PixelRatio, f.camera
	a.bakedCount = count
}

// Materialize a complete retained view at its own camera before an edit, zoom
// or selection uses it. In particular, panning changes the latest complete
// view without changing dryCamera. Starting from the old dry viewport would
// lose newly exposed ink in selection previews and repeatedly rebuild vectors.
func (a *Editor) adoptSurface(p *ui.Painter, v surfaceView) {
	f := newSurfaceFrame(v.viewport, v.camera)
	if v.source == a.dry[a.dryFront] && v.destination == v.viewport {
		return
	}
	next := ui.NewCanvas(f.w, f.h)
	next.Draw(p, func(cp *ui.Painter) {
		r := v.destination
		r.X = (r.X - v.viewport.X) * f.camera.PixelRatio
		r.Y = (r.Y - v.viewport.Y) * f.camera.PixelRatio
		r.W *= f.camera.PixelRatio
		r.H *= f.camera.PixelRatio
		cp.Image(v.source.Bitmap(), r, ui.FillBox)
	})
	a.publishDry([2]*ui.Canvas{next, nil}, 0, f, v.count)
	a.dryCyclic = false
	a.dryReusesPan = true
	a.panCache.reset()
	a.navigationBake = navigationBake{}
	a.retainSurface(a.dryView(f))
}

func (a *Editor) drawDry(p *ui.Painter, rect ui.Rect, camera ink.Camera) {
	f := newSurfaceFrame(rect, camera)
	v, complete := a.prepareSurface(p, f, true)
	if !complete {
		a.drawZoomPreview(p, rect, camera)
		return
	}
	// Transparent erasing replaces the dry view with an erased view; drawing
	// both would composite old pixels underneath the transparent holes.
	if a.transparentInk && a.erasing && a.eraseKind == 1 && len(a.erasePath) > 0 {
		a.retainSurface(v)
		return
	}
	a.presentSurface(p, v)
	if a.current == nil && !a.erasing && !a.selectionActive() {
		a.tiles.prefetch(p, a, f)
	}
}

// prepareSurface updates caches without painting them onto the caller. This
// separation matters for transparent selection/eraser layers: preparing a base
// must not accidentally paint it twice. Interactive edits require an exact
// current base; camera-only zoom may keep showing the last complete view.
func (a *Editor) prepareSurface(p *ui.Painter, f surfaceFrame, allowPreview bool) (surfaceView, bool) {
	defer a.ObservePhase("prepare", time.Now())
	if a.idleSurface.source != nil && a.idleSurface.source.Trimmed() {
		a.recoverIdleSurface(p, f.rect, false)
	} else if a.hasTrimmedWorkBuffers() {
		a.RecoverAfterIdle(p, f.rect)
	}
	c := f.camera
	if a.dry[a.dryFront] != nil && a.dryCyclic && !a.dry[a.dryFront].Rendered() {
		a.dryInvalid = true
	}
	sameSize := a.dry[a.dryFront] != nil && f.w == a.dryW && f.h == a.dryH && c.PixelRatio == a.dryRatio
	edited := a.dryInvalid || len(a.dirtyRegions) > 0 || a.bakedCount != len(a.strokes)
	if sameSize && !a.dryInvalid && a.idleSurface.source != nil && a.idleSurface.source != a.dry[a.dryFront] &&
		a.idleSurface.viewport == f.rect && (edited || a.current != nil || a.erasing || !allowPreview || a.recompose || c.Zoom != a.dryCamera.Zoom) {
		a.adoptSurface(p, a.idleSurface)
	}
	stable := sameSize && !edited && a.current == nil && !a.erasing
	if stable && c == a.dryCamera && !a.recompose {
		a.navigationBake = navigationBake{}
		a.zoomPreview = false
	}
	if stable && allowPreview && c.Zoom != a.dryCamera.Zoom {
		if a.zoomChangedAt.IsZero() {
			a.zoomChangedAt = time.Now()
		}
		a.zoomPreview = time.Since(a.zoomChangedAt) < 180*time.Millisecond
		if a.zoomPreview {
			// Input supersedes work immediately; do not retain a partial bake
			// with the wrong camera while displaying its predecessor.
			if a.navigationBake.active && a.navigationBake.camera != c {
				a.navigationBake.active = false
			}
			return surfaceView{}, false
		}
	}
	a.zoomPreview = false
	if !stable || !allowPreview {
		a.navigationBake = navigationBake{}
	}
	if stable && a.recompose && c.Zoom == a.dryCamera.Zoom && a.navigationBake.active && c.Offset != a.navigationBake.camera.Offset {
		a.recompose = false
		a.navigationBake = navigationBake{}
	}
	// A translation that interrupts an unfinished zoom establishes a complete
	// base once. Subsequent translations use immediate exposed-strip repair.
	if stable && a.navigationBake.active && c.Zoom == a.navigationBake.camera.Zoom && c.Offset != a.navigationBake.camera.Offset {
		a.navigationBake = navigationBake{}
		a.dryInvalid = true
		stable = false
	}
	if stable && allowPreview && (a.recompose || c != a.dryCamera && (a.navigationBake.active || c.Zoom != a.dryCamera.Zoom)) {
		if !a.bakeNavigation(p, f.rect, c, f.w, f.h) {
			return surfaceView{}, false
		}
		return a.dryView(f), true
	}
	if edited {
		a.panCache.reset()
	}
	if a.usePanSurface(c, f.w, f.h) {
		v := a.panCache.prepare(p, a, f.rect, c, f.w, f.h)
		if !allowPreview {
			a.adoptSurface(p, v)
			return a.dryView(f), true
		}
		return v, true
	}
	if len(a.dirtyRegions) > 0 && !a.dryInvalid && sameSize && c == a.dryCamera {
		a.repairDry(p, c)
	}
	a.dirtyRegions = nil
	if a.dryInvalid || !sameSize || a.dryCamera != c || len(a.strokes) < a.bakedCount {
		a.rebuildSurface(p, f)
	} else if a.bakedCount < len(a.strokes) {
		a.appendSurface(p, f)
	}
	a.recompose = false // the exact current scene already incorporates it
	return a.dryView(f), true
}

// Full reconstruction is also the recovery fallback when there is no valid
// GPU base. It never consults raster caches as the source of document content.
func (a *Editor) rebuildSurface(p *ui.Painter, f surfaceFrame) {
	a.frameTrace.rebuilds++
	a.panCache.reset()
	a.navigationBake = navigationBake{}
	a.live.Reset()
	next := ui.NewCanvas(f.w, f.h)
	next.Draw(p, func(cp *ui.Painter) {
		a.paintPaper(cp, f.pixels(), f.pixelCamera())
		a.tiles.syncFrame(a, f.pixelCamera(), false)
		for _, id := range a.tiles.index.query(f.visible(), a.tiles.sources) {
			a.renderer.DrawViewport(cp, a.renderStroke(a.tiles.sources[id].stroke), f.pixelCamera(), float32(f.w), float32(f.h))
		}
	})
	a.publishDry([2]*ui.Canvas{next, nil}, 0, f, len(a.strokes))
	a.dryCyclic = false
	a.dryReusesPan = false
	a.dryInvalid = false
}

func (a *Editor) appendSurface(p *ui.Painter, f surfaceFrame) {
	a.frameTrace.appends++
	next := 1 - a.dryFront
	if a.dry[next] == nil {
		a.dry[next] = ui.NewCanvas(f.w, f.h)
	}
	previous, start := a.dry[a.dryFront], a.bakedCount
	occupied := a.occupiedRect(f)
	a.dry[next].Draw(p, func(cp *ui.Painter) {
		a.copyContent(cp, previous, f, occupied)
		a.paintStrokeRange(cp, f, start)
	})
	a.publishDry(a.dry, next, f, len(a.strokes))
}

func (a *Editor) paintStrokeRange(p *ui.Painter, f surfaceFrame, start int) {
	visible, c := f.visible(), f.pixelCamera()
	for _, s := range a.strokes[start:] {
		if ink.BoundsOverlap(ink.StrokeBounds(s), visible) {
			a.renderer.DrawViewport(p, a.renderStroke(s), c, float32(f.w), float32(f.h))
		}
	}
}
