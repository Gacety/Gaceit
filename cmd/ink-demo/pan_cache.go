package main

import (
	"math"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

const panCachePadding = 192 // physical pixels, independent of zoom and DPR
const panCacheGuard = 48

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
	offset                     [2]float64
	sources                    []panDrawable
}

func (s *panSurface) reset() { *s = panSurface{} }

func (a *app) usePanSurface(c ink.Camera, w, h int) bool {
	return a.current == nil && !a.erasing && !a.dryInvalid && len(a.dirtyRegions) == 0 &&
		a.dry[0] != nil && a.bakedCount == len(a.strokes) && w == a.dryW && h == a.dryH &&
		c.PixelRatio == a.dryCamera.PixelRatio && c.Zoom == a.dryCamera.Zoom && c.Offset != a.dryCamera.Offset
}

func (s *panSurface) draw(p *ui.Painter, a *app, viewport ui.Rect, c ink.Camera, w, h int) {
	if s.layers[0] == nil || s.viewportW != w || s.viewportH != h || s.ratio != c.PixelRatio || s.zoom != c.Zoom {
		s.reset()
		s.viewportW = w
		s.viewportH = h
		s.w = w + 2*panCachePadding
		s.h = h + 2*panCachePadding
		s.ratio = c.PixelRatio
		s.zoom = c.Zoom
		s.offset = [2]float64{float64(a.dryCamera.Offset[0]), float64(a.dryCamera.Offset[1])}
		s.layers = [2]*ui.Canvas{ui.NewCanvas(s.w, s.h), ui.NewCanvas(s.w, s.h)}
		s.sources = make([]panDrawable, 0, len(a.strokes))
		for _, stroke := range a.strokes {
			// Imported/demo pencil paths may not have their deterministic stamps yet.
			// Resolve once here, rather than regenerating them for every exposed strip.
			if stroke.Tool == ink.Pencil && stroke.Stamps == nil {
				stroke.Stamps = ink.MakeStrokeStamps(stroke, 1)
			}
			s.sources = append(s.sources, panDrawable{stroke: stroke, bounds: ink.StrokeBounds(stroke)})
		}
		page := a.dry[a.dryFront].Bitmap()
		pad := float32(panCachePadding)
		s.layers[0].Draw(p, func(cp *ui.Painter) {
			cp.Fill(ui.Rect{W: float32(s.w), H: float32(s.h)}, ui.RGB(250, 249, 245), 0)
			cp.Image(page, ui.Rect{X: pad, Y: pad, W: float32(w), H: float32(h)}, ui.FillBox)
			s.paint(cp, a, []ui.Rect{
				{W: float32(s.w), H: pad},
				{Y: pad + float32(h), W: float32(s.w), H: pad},
				{Y: pad, W: pad, H: float32(h)},
				{X: pad + float32(w), Y: pad, W: pad, H: float32(h)},
			})
		})
	}
	dx := (float64(c.Offset[0]) - s.offset[0]) * float64(s.ratio)
	dy := (float64(c.Offset[1]) - s.offset[1]) * float64(s.ratio)
	if math.Abs(dx) > panCachePadding-panCacheGuard || math.Abs(dy) > panCachePadding-panCacheGuard {
		largeJump := math.Abs(dx) >= float64(s.w) || math.Abs(dy) >= float64(s.h)
		shiftX, shiftY := 0, 0
		if largeJump {
			// Avoid integer overflow for arbitrary navigation jumps.
			s.offset = [2]float64{float64(c.Offset[0]), float64(c.Offset[1])}
		} else {
			shiftX, shiftY = int(math.Round(dx)), int(math.Round(dy))
			s.offset[0] += float64(shiftX) / float64(s.ratio)
			s.offset[1] += float64(shiftY) / float64(s.ratio)
		}
		previous := s.layers[s.front]
		next := 1 - s.front
		s.layers[next].Draw(p, func(cp *ui.Painter) {
			cp.Fill(ui.Rect{W: float32(s.w), H: float32(s.h)}, ui.RGB(250, 249, 245), 0)
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
	p.Clip(viewport, 0, func() { p.Image(s.layers[s.front].Bitmap(), destination, ui.FillBox) })
}

func (s *panSurface) paint(p *ui.Painter, a *app, regions []ui.Rect) {
	camera := ink.Camera{Zoom: s.zoom * s.ratio, PixelRatio: 1, Offset: [2]float32{
		float32(s.offset[0]*float64(s.ratio)) + panCachePadding,
		float32(s.offset[1]*float64(s.ratio)) + panCachePadding,
	}}
	for _, region := range regions {
		if region.W <= 0 || region.H <= 0 {
			continue
		}
		visible := [4]float32{(region.X - camera.Offset[0]) / camera.Zoom, (region.Y - camera.Offset[1]) / camera.Zoom,
			(region.X + region.W - camera.Offset[0]) / camera.Zoom, (region.Y + region.H - camera.Offset[1]) / camera.Zoom}
		p.Clip(region, 0, func() {
			p.Fill(region, ui.RGB(250, 249, 245), 0)
			for _, source := range s.sources {
				if ink.BoundsOverlap(source.bounds, visible) {
					a.renderer.DrawRegion(p, source.stroke, camera, region)
				}
			}
		})
	}
}
