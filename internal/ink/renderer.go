package ink

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// Camera maps model coordinates to painter DIPs. PixelRatio must match the
// destination's physical pixels per DIP (e.g. 2 for a Retina display).
type Camera struct {
	Zoom, PixelRatio float32
	Offset           [2]float32
}
type Renderer struct {
	masks                                                                             maskCache
	geometry                                                                          segmentCache
	distance, coverage, composite, normalComposite, pencil, erasedImage, erasePreview *shader.Shader
	atlas                                                                             *ui.Bitmap
	rects                                                                             [][4]int
}

func NewRenderer() (*Renderer, error) {
	r := &Renderer{}
	for name, dst := range map[string]**shader.Shader{"distance.kage": &r.distance, "composite.kage": &r.composite, "pencil.kage": &r.pencil, "erased-image.kage": &r.erasedImage, "erase-preview.kage": &r.erasePreview} {
		src, err := resources.ReadFile(name)
		if err != nil {
			return nil, err
		}
		*dst, err = shader.Compile(string(src))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	src, err := resources.ReadFile("distance.kage")
	if err != nil {
		return nil, err
	}
	r.coverage = r.distance
	// Ordinary pens do not need a backdrop capture for every segment.
	normal := strings.ReplaceAll(string(src), "max(backdropAt(pos).r,coverage)", "coverage")
	r.distance, err = shader.Compile(normal)
	if err != nil {
		return nil, err
	}
	// A white opaque backdrop turns multiply into ordinary source-over.
	// Compile it separately so pens do not capture the page backdrop.
	compositeSrc, err := resources.ReadFile("composite.kage")
	if err != nil {
		return nil, err
	}
	r.normalComposite, err = shader.Compile(strings.ReplaceAll(string(compositeSrc), "backdropAt(pos)", "vec4(1)"))
	if err != nil {
		return nil, err
	}
	data, err := resources.ReadFile("assets/pencil-stamp-atlas.png")
	if err != nil {
		return nil, err
	}
	r.atlas, err = ui.DecodeBitmap(data)
	if err != nil {
		return nil, err
	}
	r.rects, err = atlasRects()
	return r, err
}

// Draw renders one stroke. Use the view's Draw callback; the renderer owns a
// per-draw mask and is confined to that view's UI goroutine.
func (r *Renderer) Draw(p *ui.Painter, s Stroke, c Camera) { r.draw(p, s, c, nil) }

// DrawViewport bounds temporary textures by the current viewport, not the
// unbounded model-space extent of a long or distant stroke.
func (r *Renderer) DrawViewport(p *ui.Painter, s Stroke, c Camera, width, height float32) {
	r.draw(p, s, c, &ui.Rect{W: width, H: height})
}

// DrawRegion limits temporary masks to a pixel-aligned repair region.
func (r *Renderer) DrawRegion(p *ui.Painter, s Stroke, c Camera, region ui.Rect) {
	r.draw(p, s, c, &region)
}
func (r *Renderer) draw(p *ui.Painter, s Stroke, c Camera, clip *ui.Rect) {
	if c.Zoom <= 0 || c.PixelRatio <= 0 {
		panic("ink: camera scale must be positive")
	}
	if s.Tool == Pencil {
		if len(s.Erasures) > 0 {
			r.drawErasedPencil(p, s, c, clip)
		} else {
			r.drawPencil(p, s, c)
		}
		return
	}
	var segs []Segment
	if clip != nil {
		segs = r.completedSegments(s, 1/(c.Zoom*c.PixelRatio))
	} else {
		segs = strokeSegments(s, 1/(c.Zoom*c.PixelRatio))
	}
	if len(segs) == 0 {
		return
	}
	x0, y0, x1, y1 := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, seg := range segs {
		b := seg.Bounds
		x0 = min(x0, b[0])
		y0 = min(y0, b[1])
		x1 = max(x1, b[0]+b[2])
		y1 = max(y1, b[1]+b[3])
	}
	ratio := c.PixelRatio
	// Align the mask to physical pixels so its 1px AA survives the composite.
	left := float32(math.Floor(float64((x0*c.Zoom+c.Offset[0])*ratio))) / ratio
	top := float32(math.Floor(float64((y0*c.Zoom+c.Offset[1])*ratio))) / ratio
	w := max(1, int(math.Ceil(float64((x1*c.Zoom+c.Offset[0]-left)*ratio))))
	h := max(1, int(math.Ceil(float64((y1*c.Zoom+c.Offset[1]-top)*ratio))))
	cacheMask := clip != nil && len(s.Points) > 0 && w <= 256 && h <= 256 && w*h <= maxSingleMaskPixels
	if clip != nil && (left+float32(w)/ratio < clip.X || top+float32(h)/ratio < clip.Y || left > clip.X+clip.W || top > clip.Y+clip.H) {
		return
	}
	if clip != nil && !cacheMask {
		right, bottom := min(left+float32(w)/ratio, clip.X+clip.W), min(top+float32(h)/ratio, clip.Y+clip.H)
		left = max(left, clip.X)
		top = max(top, clip.Y)
		w = int(math.Ceil(float64((right - left) * ratio)))
		h = int(math.Ceil(float64((bottom - top) * ratio)))
		if w <= 0 || h <= 0 {
			return
		}
	}
	maskCamera := Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{(c.Offset[0] - left) * ratio, (c.Offset[1] - top) * ratio}}
	mask := r.strokeMask(p, s, segs, maskCamera, w, h, cacheMask)
	composite := r.normalComposite
	if s.Tool == Highlighter {
		composite = r.composite
	}
	dst := ui.Rect{X: left, Y: top, W: float32(w) / ratio, H: float32(h) / ratio}
	images := [4]*ui.Bitmap{mask.Bitmap()}
	if len(s.Erasures) > 0 {
		images[1] = r.eraseMask(p, s.Erasures, c, dst).Bitmap()
	}
	draw := func() {
		composite.Draw(p, dst, shader.Options{Uniforms: map[string]any{"Color": s.Color, "Ratio": ratio, "HasErasure": boolInt(len(s.Erasures) > 0)}, Images: images})
	}
	if cacheMask {
		p.Clip(*clip, 0, draw)
	} else {
		draw()
	}
}
func (r *Renderer) drawSegments(p *ui.Painter, segs []Segment, color [4]float32, c Camera, mask bool, clips ...ui.Rect) {
	for _, s := range segs {
		b := s.Bounds
		if len(clips) > 0 {
			visible := clips[0]
			// Reject segments outside the actual repair mask before creating
			// shader uniforms or commands, with a 2px antialias margin.
			x, y := b[0]*c.Zoom+c.Offset[0], b[1]*c.Zoom+c.Offset[1]
			if x+b[2]*c.Zoom < visible.X-2 || y+b[3]*c.Zoom < visible.Y-2 || x > visible.X+visible.W+2 || y > visible.Y+visible.H+2 {
				continue
			}
		}
		point := func(p Point) [3]float32 { return [3]float32{p.X, p.Y, p.Radius} }
		// MyGo emits the rectangle as a four-vertex GPU quad; geometry and cubic
		// coefficients are prepared here once per segment, never per fragment.
		program := r.distance
		if mask {
			program = r.coverage
		}
		program.Draw(p, ui.Rect{X: b[0]*c.Zoom + c.Offset[0], Y: b[1]*c.Zoom + c.Offset[1], W: b[2] * c.Zoom, H: b[3] * c.Zoom}, shader.Options{Uniforms: map[string]any{
			"Origin": [2]float32{b[0]*c.Zoom + c.Offset[0], b[1]*c.Zoom + c.Offset[1]}, "P0": point(s.P[0]), "P1": point(s.P[1]), "P2": point(s.P[2]), "A": s.A, "B": s.B, "KK": s.KK, "KX": s.KX, "Phase": s.Phase, "Mode": s.Mode, "Height": b[3], "Zoom": c.Zoom, "Color": color, "Mask": boolInt(mask),
		}})
	}
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func (r *Renderer) drawPencil(p *ui.Painter, s Stroke, c Camera) {
	stamps := s.Stamps
	if stamps == nil {
		stamps = MakeStrokeStamps(s, 1)
	}
	for _, stamp := range stamps {
		d, e := stamp[0], stamp[1]
		idx := int(e[2])
		if idx < 0 || idx >= len(r.rects) || e[0] == 0 || e[1] == 0 {
			continue
		}
		angle := float64(d[3])
		cs, sn := float32(math.Abs(math.Cos(angle))), float32(math.Abs(math.Sin(angle)))
		hw := (float32(math.Abs(float64(e[0])))*cs + float32(math.Abs(float64(e[1])))*sn) * c.Zoom / 2
		hh := (float32(math.Abs(float64(e[0])))*sn + float32(math.Abs(float64(e[1])))*cs) * c.Zoom / 2
		d[0] += s.Translation[0]
		d[1] += s.Translation[1]
		stamp[0] = d
		x, y := d[0]*c.Zoom+c.Offset[0]-hw, d[1]*c.Zoom+c.Offset[1]-hh
		rect := r.rects[idx]
		ar := [4]float32{float32(rect[0]), float32(rect[1]), float32(rect[2]), float32(rect[3])}
		r.pencil.Draw(p, ui.Rect{X: x, Y: y, W: 2 * hw, H: 2 * hh}, shader.Options{Uniforms: map[string]any{"Stamp": append(stamp[0][:], stamp[1][:]...), "Rect": ar, "Origin": c.Offset, "Zoom": c.Zoom, "Color": s.Color}, Images: [4]*ui.Bitmap{r.atlas}})
	}
}
