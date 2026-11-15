package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
)

// SetTransparentInk keeps paper/UI outside the cached ink surfaces. Only the
// embedding app opts in; the independently runnable demo stays opaque.
func (a *Editor) SetTransparentInk(on bool) {
	if a.transparentInk == on {
		return
	}
	a.transparentInk = on
	a.ResetSurfaceCaches()
}

type paperSurface struct {
	canvas     *ui.Canvas
	camera     ink.Camera
	background ui.Color
	w, h       int
}

func (a *Editor) DrawPaper(p *ui.Painter, r ui.Rect) {
	c := a.camera()
	ratio := c.PixelRatio
	w, h := max(1, int(math.Ceil(float64(r.W*ratio)))), max(1, int(math.Ceil(float64(r.H*ratio))))
	cache := &a.paper
	changed := cache.canvas == nil || cache.w != w || cache.h != h
	if changed {
		cache.canvas = ui.NewCanvas(w, h)
	}
	if changed || cache.canvas.Trimmed() || cache.camera != c || cache.background != a.background {
		cache.canvas.Draw(p, func(cp *ui.Painter) {
			a.drawPaper(cp, ui.Rect{W: float32(w), H: float32(h)}, ink.Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}})
		})
		cache.camera = c
		cache.background = a.background
		cache.w = w
		cache.h = h
	}
	p.Image(cache.canvas.Bitmap(), r, ui.FillBox)
}

// Canvas.Draw starts clear. Copy only unchanged pixels; source-over drawing
// transparent paper over an old image would leave deleted ink behind.
func copyOutside(p *ui.Painter, bitmap *ui.Bitmap, full ui.Rect, holes []ui.Rect) {
	remaining := []ui.Rect{full}
	for _, hole := range holes {
		next := make([]ui.Rect, 0, len(remaining)+4)
		for _, r := range remaining {
			next = append(next, outsideBands(r, hole)...)
		}
		remaining = next
	}
	for _, r := range remaining {
		p.Clip(r, 0, func() { p.Image(bitmap, full, ui.FillBox) })
	}
}
