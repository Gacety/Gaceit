package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"time"
)

const navigationBakeBatch = 8
const navigationBakeWorkLimit = 4096

type navigationBake struct {
	layers               [2]*ui.Canvas
	content              ui.Rect
	front, next          int
	camera               ink.Camera
	width, height, count int
	recomposeGen         uint64
	revision             uint64
	active               bool
	wrapped              bool
	waiting              bool
}

// Keep the last complete viewport visible while a replacement is baked in
// bounded chunks. Three viewport textures at peak (old + two ping-pong),
// independent of document length. Input may supersede this work at any frame.
func (a *Editor) bakeNavigation(p *ui.Painter, rect ui.Rect, c ink.Camera, w, h int) bool {
	b := &a.navigationBake
	if !b.active || b.camera != c || b.width != w || b.height != h || b.count != len(a.strokes) || b.revision != a.editRevision {
		// Reuse allocated scratch targets when input supersedes an unfinished bake.
		layers := b.layers
		if b.width != w || b.height != h {
			layers = [2]*ui.Canvas{}
		}
		if layers[0] == nil && a.dry[1-a.dryFront] != nil {
			layers[0] = a.dry[1-a.dryFront]
			a.dry[1-a.dryFront] = nil
		}
		if layers[0] == nil {
			layers[0] = ui.NewCanvas(w, h)
		}
		*b = navigationBake{layers: layers, camera: c, width: w, height: h, count: len(a.strokes), recomposeGen: a.recomposeGeneration, revision: a.editRevision, active: true}
		a.panCache.reset()
	}
	if b.next > 0 {
		previous := b.layers[b.front]
		if previous == nil || !previous.Rendered() {
			if previous != nil && !b.waiting && !b.wrapped {
				previous.Flush(p)
				b.waiting = true
				p.After(8 * time.Millisecond)
				return false
			}
			b.next = 0
			b.front = 0
			b.content = ui.Rect{}
			b.wrapped = false
			b.waiting = false
		}
	}
	b.waiting = false
	next := 0
	if b.next > 0 {
		next = 1 - b.front
		if next == 0 {
			b.wrapped = true
		}
	}
	if b.layers[next] == nil {
		b.layers[next] = ui.NewCanvas(w, h)
	}
	start, end := b.next, b.next
	previous := b.layers[b.front]
	pixelCamera := ink.Camera{Zoom: c.Zoom * c.PixelRatio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * c.PixelRatio, c.Offset[1] * c.PixelRatio}}
	visible := [4]float32{-c.Offset[0] / c.Zoom, -c.Offset[1] / c.Zoom, (rect.W - c.Offset[0]) / c.Zoom, (rect.H - c.Offset[1]) / c.Zoom}
	work := make([]ink.Stroke, 0, navigationBakeBatch)
	workUnits := 0
	for end < len(a.strokes) && len(work) < navigationBakeBatch {
		stroke := a.strokes[end]
		if ink.BoundsOverlap(ink.StrokeBounds(stroke), visible) {
			cost := strokeWork(stroke, navigationBakeWorkLimit)
			if len(work) > 0 && workUnits+cost > navigationBakeWorkLimit {
				break
			}
			work = append(work, stroke)
			workUnits += cost
		}
		end++
	}
	b.layers[next].Draw(p, func(cp *ui.Painter) {
		if start == 0 {
			a.paintPaper(cp, ui.Rect{W: float32(w), H: float32(h)}, pixelCamera)
		} else {
			a.copyContent(cp, previous, newSurfaceFrame(rect, c), b.content)
		}
		for _, s := range work {
			r := newSurfaceFrame(rect, c).contentRect(ink.StrokeBounds(s))
			if r.W > 0 && r.H > 0 {
				if b.content.W == 0 {
					b.content = r
				} else {
					b.content = unionRect(b.content, r)
				}
			}
			a.renderer.DrawViewport(cp, a.renderStroke(s), pixelCamera, float32(w), float32(h))
		}
	})
	b.front, b.next = next, end
	if end == len(a.strokes) {
		a.publishDry(b.layers, b.front, newSurfaceFrame(rect, c), len(a.strokes))
		a.dryCyclic = b.wrapped
		a.dryReusesPan = false
		requested := a.recomposeGeneration != b.recomposeGen
		a.navigationBake = navigationBake{}
		if !requested {
			a.recompose = false
		}
		return true
	}
	p.After(8 * time.Millisecond)
	return false
}

func (a *Editor) RebuildingNavigation() bool { return a.navigationBake.active }
