package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
)

func (f surfaceFrame) contentRect(b [4]float32) ui.Rect {
	c := f.pixelCamera()
	x0 := max(float32(0), float32(math.Floor(float64(b[0]*c.Zoom+c.Offset[0])))-2)
	y0 := max(float32(0), float32(math.Floor(float64(b[1]*c.Zoom+c.Offset[1])))-2)
	x1 := min(float32(f.w), float32(math.Ceil(float64(b[2]*c.Zoom+c.Offset[0])))+2)
	y1 := min(float32(f.h), float32(math.Ceil(float64(b[3]*c.Zoom+c.Offset[1])))+2)
	return ui.Rect{X: x0, Y: y0, W: max(0, x1-x0), H: max(0, y1-y0)}
}

// Empty paper is regenerated cheaply; only the occupied ink rectangle needs
// to be copied between checkpoints. Dense views naturally approach a viewport.
func (a *Editor) copyContent(p *ui.Painter, previous *ui.Canvas, f surfaceFrame, r ui.Rect) {
	if !a.transparentInk {
		p.Image(previous.Bitmap(), f.pixels(), ui.FillBox)
		a.frameTrace.copiedPixels += uint64(f.w * f.h)
		return
	}
	if r.W > 0 && r.H > 0 {
		p.Clip(r, 0, func() { p.Image(previous.Bitmap(), f.pixels(), ui.FillBox) })
		a.frameTrace.copiedPixels += uint64(r.W * r.H)
	}
}
func (a *Editor) occupiedRect(f surfaceFrame) ui.Rect {
	a.tiles.syncFrame(a, f.pixelCamera(), false)
	var out ui.Rect
	for _, id := range a.tiles.index.query(f.visible(), a.tiles.sources) {
		r := f.contentRect(a.tiles.sources[id].bounds)
		if r.W <= 0 || r.H <= 0 {
			continue
		}
		if out.W == 0 {
			out = r
		} else {
			out = unionRect(out, r)
		}
	}
	return out
}
