package ink

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"math"
)

func (r *Renderer) eraseMask(p *ui.Painter, paths [][]Point, c Camera, dst ui.Rect) *ui.Canvas {
	ratio := c.PixelRatio
	mask := ui.NewCanvas(max(1, int(math.Round(float64(dst.W*ratio)))), max(1, int(math.Round(float64(dst.H*ratio)))))
	camera := Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{(c.Offset[0] - dst.X) * ratio, (c.Offset[1] - dst.Y) * ratio}}
	mask.Draw(p, func(cp *ui.Painter) {
		for _, path := range paths {
			if len(path) == 1 {
				r.drawSegments(cp, Segments(path, [2]float32{}, 1/(c.Zoom*ratio)), [4]float32{}, camera, true)
			}
			for i := 1; i < len(path); i++ {
				r.drawSegments(cp, Segments(path[i-1:i+1], [2]float32{}, 1/(c.Zoom*ratio)), [4]float32{}, camera, true)
			}
		}
	})
	return mask
}
func stampBounds(stamp Stamp) (x, y, hw, hh float32) {
	d, e := stamp[0], stamp[1]
	cs, sn := float32(math.Abs(math.Cos(float64(d[3])))), float32(math.Abs(math.Sin(float64(d[3]))))
	hw = (float32(math.Abs(float64(e[0])))*cs + float32(math.Abs(float64(e[1])))*sn) / 2
	hh = (float32(math.Abs(float64(e[0])))*sn + float32(math.Abs(float64(e[1])))*cs) / 2
	return d[0], d[1], hw, hh
}
func (r *Renderer) drawErasedPencil(p *ui.Painter, s Stroke, c Camera, clip *ui.Rect) {
	if s.Stamps == nil {
		s.Stamps = MakeStrokeStamps(s, 1)
	}
	if len(s.Stamps) == 0 {
		return
	}
	x0, y0, x1, y1 := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, stamp := range s.Stamps {
		x, y, hw, hh := stampBounds(stamp)
		x += s.Translation[0]
		y += s.Translation[1]
		x0 = min(x0, x-hw)
		y0 = min(y0, y-hh)
		x1 = max(x1, x+hw)
		y1 = max(y1, y+hh)
	}
	ratio := c.PixelRatio
	left, top := float32(math.Floor(float64((x0*c.Zoom+c.Offset[0])*ratio)))/ratio, float32(math.Floor(float64((y0*c.Zoom+c.Offset[1])*ratio)))/ratio
	w, h := max(1, int(math.Ceil(float64((x1*c.Zoom+c.Offset[0]-left)*ratio)))), max(1, int(math.Ceil(float64((y1*c.Zoom+c.Offset[1]-top)*ratio))))
	if clip != nil {
		right, bottom := min(left+float32(w)/ratio, clip.X+clip.W), min(top+float32(h)/ratio, clip.Y+clip.H)
		left = max(left, clip.X)
		top = max(top, clip.Y)
		w = int(math.Ceil(float64((right - left) * ratio)))
		h = int(math.Ceil(float64((bottom - top) * ratio)))
		if w <= 0 || h <= 0 {
			return
		}
	}
	dst := ui.Rect{X: left, Y: top, W: float32(w) / ratio, H: float32(h) / ratio}
	layer := ui.NewCanvas(w, h)
	layer.Draw(p, func(cp *ui.Painter) {
		r.drawPencil(cp, s, Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{(c.Offset[0] - left) * ratio, (c.Offset[1] - top) * ratio}})
	})
	erased := r.eraseMask(p, s.Erasures, c, dst)
	r.erasedImage.Draw(p, dst, shader.Options{Images: [4]*ui.Bitmap{layer.Bitmap(), erased.Bitmap()}, Uniforms: map[string]any{"Ratio": ratio}})
}

func pointSegmentDistance(p, a, b Point) float32 {
	dx, dy := b.X-a.X, b.Y-a.Y
	t := float32(0)
	if den := dx*dx + dy*dy; den > 0 {
		t = clamp(((p.X-a.X)*dx+(p.Y-a.Y)*dy)/den, 0, 1)
	}
	return length(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}
func segmentDistance(a, b, c, d Point) float32 {
	cross := func(p, q, r Point) float32 { return (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X) }
	ab1, ab2, cd1, cd2 := cross(a, b, c), cross(a, b, d), cross(c, d, a), cross(c, d, b)
	if ab1*ab2 < 0 && cd1*cd2 < 0 {
		return 0
	}
	return min(pointSegmentDistance(a, c, d), pointSegmentDistance(b, c, d), pointSegmentDistance(c, a, b), pointSegmentDistance(d, a, b))
}

// HitsStroke uses short chord approximations (at most 1 model unit) of the
// rendered centerline for selection. Pixel erasing itself uses analytic masks.
func HitsStroke(s Stroke, from, to Point) bool {
	if s.Tool == Pencil && s.Stamps == nil {
		s.Stamps = MakeStrokeStamps(s, 1)
	}
	if s.Tool == Pencil && len(s.Stamps) > 0 {
		for _, stamp := range s.Stamps {
			x, y, hw, hh := stampBounds(stamp)
			p := Point{X: x + s.Translation[0], Y: y + s.Translation[1]}
			if pointSegmentDistance(p, from, to) <= max(from.Radius, to.Radius)+max(hw, hh) {
				return true
			}
		}
		return false
	}
	for _, seg := range strokeSegments(s, 1) {
		h := seg.Bounds[3] / 2
		eval := func(t float32) Point {
			return Point{X: (Quad(seg.P[0].X, seg.P[1].X, seg.P[2].X, t)+1)*h + seg.Bounds[0], Y: (Quad(seg.P[0].Y, seg.P[1].Y, seg.P[2].Y, t)+1)*h + seg.Bounds[1], Radius: Quad(seg.P[0].Radius, seg.P[1].Radius, seg.P[2].Radius, t) * h}
		}
		a, b, c := eval(0), eval(0.5), eval(1)
		count := max(1, int(math.Ceil(float64(2*(length(b.X-a.X, b.Y-a.Y)+length(c.X-b.X, c.Y-b.Y))))))
		prev := a
		for j := 1; j <= count; j++ {
			next := eval(float32(j) / float32(count))
			if segmentDistance(prev, next, from, to) <= max(prev.Radius, next.Radius)+max(from.Radius, to.Radius) {
				return true
			}
			prev = next
		}
	}
	return false
}

// SweepBounds is a conservative AABB of swept disks, also valid for quadratic
// control-point convex hulls expanded by the maximum interpolated radius.
func SweepBounds(points []Point) [4]float32 {
	b := [4]float32{float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))}
	radius := float32(0)
	for _, p := range points {
		b[0] = min(b[0], p.X)
		b[1] = min(b[1], p.Y)
		b[2] = max(b[2], p.X)
		b[3] = max(b[3], p.Y)
		radius = max(radius, p.Radius)
	}
	b[0] -= radius
	b[1] -= radius
	b[2] += radius
	b[3] += radius
	return b
}
func BoundsOverlap(a, b [4]float32) bool {
	return a[0] <= b[2] && a[2] >= b[0] && a[1] <= b[3] && a[3] >= b[1]
}
func StrokeBounds(s Stroke) [4]float32 {
	if s.Tool == Pencil && s.Stamps == nil {
		s.Stamps = MakeStrokeStamps(s, 1)
	}
	b := SweepBounds(s.Points)
	if s.Tool == Pencil && len(s.Stamps) > 0 {
		b = SweepBounds(nil)
		for _, stamp := range s.Stamps {
			x, y, hw, hh := stampBounds(stamp)
			b[0] = min(b[0], x-hw)
			b[1] = min(b[1], y-hh)
			b[2] = max(b[2], x+hw)
			b[3] = max(b[3], y+hh)
		}
	}
	b[0] += s.Translation[0]
	b[2] += s.Translation[0]
	b[1] += s.Translation[1]
	b[3] += s.Translation[1]
	return b
}
