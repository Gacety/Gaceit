package ink

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// LiveStroke caches the immutable prefix of one append-only sample sequence.
// Reset on pointer-down/cancel. The final half-segment always reaches the newest
// sample, with no coordinate filter, look-ahead, prediction or delayed samples.
// Use from the same UI goroutine as Renderer, once per displayed frame.
type LiveStroke struct {
	prefix               [2]*ui.Canvas
	wet                  *ui.Canvas
	front, stable, count int
	w, h                 int
	camera               Camera
	last                 Point
	drawn                bool
	pencil               PencilStream
	pencilCount          int
	bounds               [4]float32
	boundsCount          int
	boundsCamera         Camera
	origin               [2]float32
	// SegmentsDrawn is the geometry submitted on the most recent Draw call.
	// It excludes texture copies and makes regressions in incremental work visible.
	SegmentsDrawn int
}

// Reset starts a new stroke while reusing the bounded viewport surfaces.
// Each first draw clears its target, so no previous ink leaks into the stroke.
func (l *LiveStroke) Reset() {
	*l = LiveStroke{prefix: l.prefix, wet: l.wet, w: l.w, h: l.h, camera: l.camera}
}

// Release drops rebuildable textures on navigation, close or surface loss.
func (l *LiveStroke) Release() { *l = LiveStroke{} }
func (l *LiveStroke) drawAt(p *ui.Painter, r *Renderer, s Stroke, c Camera, width, height float32) {
	l.SegmentsDrawn = 0
	if !s.Smooth && s.Tool != Pencil {
		r.Draw(p, s, c)
		return
	}
	if len(s.Points) == 0 {
		return
	}
	if c.Zoom <= 0 || c.PixelRatio <= 0 {
		panic("ink: invalid live camera")
	}
	w, h := max(1, int(math.Ceil(float64(width*c.PixelRatio)))), max(1, int(math.Ceil(float64(height*c.PixelRatio))))
	n := len(s.Points)
	// A size/camera change rebuilds the cache using the new physical pixel grid.
	if l.w != w || l.h != h {
		l.Release()
	}
	if l.w != w || l.h != h || l.camera != c || n < l.count || (n == l.count && l.drawn && s.Points[n-1] != l.last) {
		l.Reset()
		l.w = w
		l.h = h
		l.camera = c
	}
	if l.wet == nil {
		l.wet = ui.NewCanvas(w, h)
	}
	ratio := c.PixelRatio
	pixelCamera := Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}}
	full := ui.Rect{W: float32(w), H: float32(h)}
	if s.Tool == Pencil {
		stable, tail := l.pencil.Update(s)
		if len(stable) > l.pencilCount {
			next := 1 - l.front
			if l.prefix[next] == nil {
				l.prefix[next] = ui.NewCanvas(w, h)
			}
			previous, old := l.prefix[l.front], l.pencilCount
			l.prefix[next].Draw(p, func(cp *ui.Painter) {
				if old > 0 {
					cp.Image(previous.Bitmap(), full, ui.FillBox)
				}
				part := s
				part.Stamps = stable[old:]
				r.drawPencil(cp, part, pixelCamera)
			})
			l.SegmentsDrawn = len(stable) - old
			l.pencilCount = len(stable)
			l.front = next
		}
		if l.pencilCount > 0 {
			p.Image(l.prefix[l.front].Bitmap(), ui.Rect{W: width, H: height}, ui.FillBox)
		}
		part := s
		part.Stamps = tail
		r.drawPencil(p, part, c)
		l.SegmentsDrawn += len(tail)
		l.count = n
		l.last = s.Points[n-1]
		l.drawn = true
		return
	}
	target := 0
	if n >= 3 {
		target = n - 1
	}
	if target > l.stable {
		next := 1 - l.front
		if l.prefix[next] == nil {
			l.prefix[next] = ui.NewCanvas(w, h)
		}
		previous := l.prefix[l.front]
		old := l.stable
		l.prefix[next].Draw(p, func(cp *ui.Painter) {
			if old > 0 {
				cp.Image(previous.Bitmap(), full, ui.FillBox)
			}
			for i := old; i < target; i++ {
				curve := sampleCurve(s.Points, i)
				r.drawSegments(cp, Segments(curve[:], s.Translation, 1/(c.Zoom*ratio)), s.Color, pixelCamera, true)
				l.SegmentsDrawn++
			}
		})
		l.front = next
		l.stable = target
	}
	if !l.drawn || n != l.count {
		prefix := l.prefix[l.front]
		l.wet.Draw(p, func(cp *ui.Painter) {
			if l.stable > 0 {
				cp.Image(prefix.Bitmap(), full, ui.FillBox)
			}
			if n < 3 {
				segs := Segments(s.Points, s.Translation, 1/(c.Zoom*ratio))
				r.drawSegments(cp, segs, s.Color, pixelCamera, true)
				l.SegmentsDrawn += len(segs)
			} else {
				curve := sampleCurve(s.Points, n-1)
				r.drawSegments(cp, Segments(curve[:], s.Translation, 1/(c.Zoom*ratio)), s.Color, pixelCamera, true)
				l.SegmentsDrawn++
			}
		})
		l.count = n
		l.last = s.Points[n-1]
		l.drawn = true
	}
	composite := r.normalComposite
	if s.Tool == Highlighter {
		composite = r.composite
	}
	composite.Draw(p, ui.Rect{W: float32(w) / ratio, H: float32(h) / ratio}, shader.Options{Images: [4]*ui.Bitmap{l.wet.Bitmap()}, Uniforms: map[string]any{"Color": s.Color, "Ratio": ratio}})
}

// Draw uses a stroke-local physical-pixel rectangle. Growing in 128px cells
// amortizes reallocations; ordinary short writing no longer copies a viewport.
func (l *LiveStroke) Draw(p *ui.Painter, r *Renderer, s Stroke, c Camera, width, height float32) {
	if len(s.Points) == 0 {
		return
	}
	if l.boundsCamera != c || len(s.Points) < l.boundsCount {
		l.boundsCount = 0
	}
	for i := l.boundsCount; i < len(s.Points); i++ {
		q := s.Points[i]
		x := (q.X+s.Translation[0])*c.Zoom + c.Offset[0]
		y := (q.Y+s.Translation[1])*c.Zoom + c.Offset[1]
		rad := q.Radius*c.Zoom + 3/c.PixelRatio
		if s.Tool == Pencil {
			rad = q.Radius*c.Zoom*pencilBoundsExpansion() + 3/c.PixelRatio
		}
		b := [4]float32{x - rad, y - rad, x + rad, y + rad}
		if i == 0 {
			l.bounds = b
		} else {
			l.bounds = [4]float32{min(l.bounds[0], b[0]), min(l.bounds[1], b[1]), max(l.bounds[2], b[2]), max(l.bounds[3], b[3])}
		}
	}
	bounds := l.bounds
	count := len(s.Points)
	ratio := c.PixelRatio
	x0 := max(float32(0), float32(math.Floor(float64(bounds[0]*ratio/128)))*128)
	y0 := max(float32(0), float32(math.Floor(float64(bounds[1]*ratio/128)))*128)
	x1 := min(width*ratio, float32(math.Ceil(float64(bounds[2]*ratio/128)))*128)
	y1 := min(height*ratio, float32(math.Ceil(float64(bounds[3]*ratio/128)))*128)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	local := c
	local.Offset[0] -= x0 / ratio
	local.Offset[1] -= y0 / ratio
	// Migrate the immutable prefix on rectangle growth; never resubmit its
	// geometry merely because the local texture acquired more room.
	if l.boundsCount > 0 && l.boundsCamera == c && (l.w != int(x1-x0) || l.h != int(y1-y0) || l.camera != local) {
		if old := l.prefix[l.front]; old != nil {
			next := ui.NewCanvas(int(x1-x0), int(y1-y0))
			dx, dy := local.Offset[0]-l.camera.Offset[0], local.Offset[1]-l.camera.Offset[1]
			next.Draw(p, func(cp *ui.Painter) {
				cp.Image(old.Bitmap(), ui.Rect{X: dx * ratio, Y: dy * ratio, W: float32(l.w), H: float32(l.h)}, ui.FillBox)
			})
			l.prefix = [2]*ui.Canvas{next, nil}
			l.front = 0
		}
		l.w = int(x1 - x0)
		l.h = int(y1 - y0)
		l.camera = local
		l.wet = nil
		l.drawn = false
	}
	p.Transform(ui.Translation(x0/ratio, y0/ratio), 0, 0, func() { l.drawAt(p, r, s, local, (x1-x0)/ratio, (y1-y0)/ratio) })
	l.bounds = bounds
	l.boundsCount = count
	l.boundsCamera = c
}
func (l *LiveStroke) PencilStamps(s Stroke) []Stamp { return l.pencil.Finish(s) }
