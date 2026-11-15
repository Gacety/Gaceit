package inkcanvas

import (
	"math"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

const panCachePadding = 96 // physical pixels, independent of zoom and DPR
const panCacheGuard = 24

type panDrawable struct {
	stroke ink.Stroke
	bounds [4]float32
}

// panSurface is a bounded, overscanned view of the completed ink. Camera-only
// translations move this texture instead of invalidating the page. Recentring
// copies it by whole physical pixels and repaints only newly exposed strips;
// fractional movement is applied only when showing it, never baked repeatedly.
type panSurface struct {
	layers                     [2]*ui.Canvas
	front                      int
	viewportW, viewportH, w, h int
	ratio, zoom                float32
	baseOffset                 [2]float64
	basePx                     [2]float64
	shiftPx                    [2]int64
	offset                     [2]float64
}

func (s *panSurface) reset() { *s = panSurface{} }

func (s *panSurface) tileCamera() ink.Camera {
	return ink.Camera{
		Zoom:       s.zoom * s.ratio,
		PixelRatio: 1,
		Offset: [2]float32{
			float32(s.basePx[0] + float64(s.shiftPx[0])),
			float32(s.basePx[1] + float64(s.shiftPx[1])),
		},
	}
}

func phaseDrift(offset, tilePhase float32) float64 {
	frac := offset - float32(math.Floor(float64(offset)))
	d := math.Abs(float64(frac - tilePhase))
	if d > 0.5 {
		d = 1 - d
	}
	return d
}

func (a *Editor) usePanSurface(c ink.Camera, w, h int) bool {
	return a.current == nil && !a.erasing && !a.dryInvalid && len(a.dirtyRegions) == 0 &&
		a.dry[a.dryFront] != nil && a.bakedCount == len(a.strokes) && w == a.dryW && h == a.dryH &&
		c.PixelRatio == a.dryCamera.PixelRatio && c.Zoom == a.dryCamera.Zoom && c.Offset != a.dryCamera.Offset
}

func (s *panSurface) prepare(p *ui.Painter, a *Editor, viewport ui.Rect, c ink.Camera, w, h int) surfaceView {

	if s.layers[0] == nil || s.viewportW != w || s.viewportH != h || s.ratio != c.PixelRatio || s.zoom != c.Zoom {
		s.reset()
		s.viewportW = w
		s.viewportH = h
		s.w = w + 2*panCachePadding
		s.h = h + 2*panCachePadding
		s.ratio = c.PixelRatio
		s.zoom = c.Zoom
		s.baseOffset = [2]float64{float64(a.dryCamera.Offset[0]), float64(a.dryCamera.Offset[1])}
		s.basePx = [2]float64{float64(a.dryCamera.Offset[0]*s.ratio) + panCachePadding, float64(a.dryCamera.Offset[1]*s.ratio) + panCachePadding}
		s.shiftPx = [2]int64{}
		s.offset = s.baseOffset
		s.layers = [2]*ui.Canvas{ui.NewCanvas(s.w, s.h), nil}
		// The immutable front plus two overscan targets is the whole pan
		// working set; a stale dry spare is no longer needed.
		a.dry[1-a.dryFront] = nil
		a.tiles.syncFrame(a, s.tileCamera(), false)
		page := a.dry[a.dryFront].Bitmap()
		pad := float32(panCachePadding)
		s.layers[0].Draw(p, func(cp *ui.Painter) {
			// A cropped pan view already references an earlier pan scene. Seed
			// the next pan from complete regional scenes, not that dependency
			// chain. This keeps repeated pan -> pen -> pan work bounded.
			if a.dryReusesPan {
				s.paint(cp, a, []ui.Rect{{W: float32(s.w), H: float32(s.h)}})
				return
			}
			if !a.transparentInk {
				cp.Fill(ui.Rect{W: float32(s.w), H: float32(s.h)}, a.background, 0)
			}
			cp.Image(page, ui.Rect{X: pad, Y: pad, W: float32(w), H: float32(h)}, ui.FillBox)
			s.paint(cp, a, []ui.Rect{
				{W: float32(s.w), H: pad},
				{Y: pad + float32(h), W: float32(s.w), H: pad},
				{Y: pad, W: pad, H: float32(h)},
				{X: pad + float32(w), Y: pad, W: pad, H: float32(h)},
			})
		})
	}
	shiftDX := (float64(c.Offset[0]) - s.offset[0]) * float64(s.ratio)
	shiftDY := (float64(c.Offset[1]) - s.offset[1]) * float64(s.ratio)
	if math.Abs(shiftDX) > panCachePadding-panCacheGuard || math.Abs(shiftDY) > panCachePadding-panCacheGuard {
		largeJump := math.Abs(shiftDX) >= float64(s.w) || math.Abs(shiftDY) >= float64(s.h)
		shiftX, shiftY := 0, 0
		if largeJump {
			// Avoid integer overflow and resync tile phase for arbitrary navigation jumps.
			s.baseOffset = [2]float64{float64(c.Offset[0]), float64(c.Offset[1])}
			s.basePx = [2]float64{float64(c.Offset[0]*s.ratio) + panCachePadding, float64(c.Offset[1]*s.ratio) + panCachePadding}
			s.shiftPx = [2]int64{}
			s.offset = s.baseOffset
			a.tiles.syncFrame(a, s.tileCamera(), false)
		} else {
			shiftX, shiftY = int(math.Round(shiftDX)), int(math.Round(shiftDY))
			s.shiftPx[0] += int64(shiftX)
			s.shiftPx[1] += int64(shiftY)
			s.offset[0] = s.baseOffset[0] + float64(s.shiftPx[0])/float64(s.ratio)
			s.offset[1] = s.baseOffset[1] + float64(s.shiftPx[1])/float64(s.ratio)
			cam := s.tileCamera()
			if phaseDrift(cam.Offset[0], a.tiles.phase[0]) > 1e-3 || phaseDrift(cam.Offset[1], a.tiles.phase[1]) > 1e-3 {
				a.tiles.syncFrame(a, cam, false)
			}
		}
		previous := s.layers[s.front]
		next := 1 - s.front
		if s.layers[next] == nil {
			s.layers[next] = ui.NewCanvas(s.w, s.h)
		}
		s.layers[next].Draw(p, func(cp *ui.Painter) {
			if !a.transparentInk {
				cp.Fill(ui.Rect{W: float32(s.w), H: float32(s.h)}, a.background, 0)
			}
			if largeJump {
				s.paint(cp, a, []ui.Rect{{W: float32(s.w), H: float32(s.h)}})
				return
			}
			cp.Image(previous.Bitmap(), ui.Rect{X: float32(shiftX), Y: float32(shiftY), W: float32(s.w), H: float32(s.h)}, ui.FillBox)
			var exposed []ui.Rect
			y0, y1 := float32(0), float32(s.h)
			if shiftY > 0 {
				exposed = append(exposed, ui.Rect{W: float32(s.w), H: float32(shiftY)})
				y0 = float32(shiftY)
			}
			if shiftY < 0 {
				y1 = float32(s.h + shiftY)
				exposed = append(exposed, ui.Rect{Y: y1, W: float32(s.w), H: float32(-shiftY)})
			}
			// Horizontal strips exclude the top/bottom strip, so translucent ink is
			// composited once at the corners as well.
			if shiftX > 0 {
				exposed = append(exposed, ui.Rect{Y: y0, W: float32(shiftX), H: y1 - y0})
			}
			if shiftX < 0 {
				exposed = append(exposed, ui.Rect{X: float32(s.w + shiftX), Y: y0, W: float32(-shiftX), H: y1 - y0})
			}
			s.paint(cp, a, exposed)
		})
		s.front = next
	}
	pad := float32(panCachePadding) / s.ratio
	destination := ui.Rect{
		X: viewport.X + float32(float64(c.Offset[0])-s.offset[0]) - pad,
		Y: viewport.Y + float32(float64(c.Offset[1])-s.offset[1]) - pad,
		W: float32(s.w) / s.ratio, H: float32(s.h) / s.ratio,
	}
	return surfaceView{source: s.layers[s.front], destination: destination, viewport: viewport, camera: c, count: a.bakedCount}
}

func (s *panSurface) paint(p *ui.Painter, a *Editor, regions []ui.Rect) {
	camera := s.tileCamera()
	for _, region := range regions {
		if region.W <= 0 || region.H <= 0 {
			continue
		}
		p.Clip(region, 0, func() {
			a.tiles.paint(p, a, region, camera)
		})
	}
}
