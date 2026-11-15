package ink

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"math"
)

// ErasePreview incrementally accumulates a swept disk mask over an unchanged
// page. It consumes all appended samples once per rendered frame; it does not
// re-run stroke hit tests or rebuild stroke textures during pointer movement.
type ErasePreview struct {
	layers             [2]*ui.Canvas
	front, count, w, h int
	camera             Camera
	last               Point
	SegmentsDrawn      int
}

func (e *ErasePreview) Reset() { *e = ErasePreview{} }
func (e *ErasePreview) Draw(p *ui.Painter, r *Renderer, page *ui.Bitmap, path []Point, c Camera, width, height float32, paper [4]float32) {
	e.SegmentsDrawn = 0
	if len(path) == 0 {
		return
	}
	if c.Zoom <= 0 || c.PixelRatio <= 0 {
		panic("ink: invalid erase preview camera")
	}
	w, h := max(1, int(math.Ceil(float64(width*c.PixelRatio)))), max(1, int(math.Ceil(float64(height*c.PixelRatio))))
	n := len(path)
	if w != e.w || h != e.h || c != e.camera || n < e.count || (n == e.count && n > 0 && path[n-1] != e.last) {
		e.Reset()
		e.w = w
		e.h = h
		e.camera = c
	}
	if e.layers[0] == nil {
		e.layers = [2]*ui.Canvas{ui.NewCanvas(w, h), ui.NewCanvas(w, h)}
	}
	ratio := c.PixelRatio
	if n > e.count {
		old := e.count
		next := 1 - e.front
		previous := e.layers[e.front]
		camera := Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}}
		e.layers[next].Draw(p, func(cp *ui.Painter) {
			if old > 0 {
				cp.Image(previous.Bitmap(), ui.Rect{W: float32(w), H: float32(h)}, ui.FillBox)
			} else {
				r.drawSegments(cp, Segments(path[:1], [2]float32{}, 1/(c.Zoom*ratio)), [4]float32{}, camera, true)
				e.SegmentsDrawn++
			}
			for i := max(1, old); i < n; i++ {
				r.drawSegments(cp, Segments(path[i-1:i+1], [2]float32{}, 1/(c.Zoom*ratio)), [4]float32{}, camera, true)
				e.SegmentsDrawn++
			}
		})
		e.front = next
		e.count = n
		e.last = path[n-1]
	}
	r.erasePreview.Draw(p, ui.Rect{W: float32(w) / ratio, H: float32(h) / ratio}, shader.Options{
		Images: [4]*ui.Bitmap{page, e.layers[e.front].Bitmap()}, Uniforms: map[string]any{"Ratio": ratio, "Paper": paper},
	})
}
