package main

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
)

// Repair only the removed strokes' visible area. Cache textures are reused;
// unaffected pixels are copied and strokes outside that area do not rebuild SDF.
func (a *app) repairDry(p *ui.Painter, c ink.Camera) {
	ratio := c.PixelRatio
	world := a.dirtyRegions[0]
	for _, b := range a.dirtyRegions[1:] {
		world[0] = min(world[0], b[0])
		world[1] = min(world[1], b[1])
		world[2] = max(world[2], b[2])
		world[3] = max(world[3], b[3])
	}
	x0 := max(float32(0), float32(math.Floor(float64((world[0]*c.Zoom+c.Offset[0])*ratio)))-2)
	y0 := max(float32(0), float32(math.Floor(float64((world[1]*c.Zoom+c.Offset[1])*ratio)))-2)
	x1 := min(float32(a.dryW), float32(math.Ceil(float64((world[2]*c.Zoom+c.Offset[0])*ratio)))+2)
	y1 := min(float32(a.dryH), float32(math.Ceil(float64((world[3]*c.Zoom+c.Offset[1])*ratio)))+2)
	if x1 <= x0 || y1 <= y0 {
		a.bakedCount = len(a.strokes)
		return
	}
	region := ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	visible := [4]float32{(x0/ratio - c.Offset[0]) / c.Zoom, (y0/ratio - c.Offset[1]) / c.Zoom, (x1/ratio - c.Offset[0]) / c.Zoom, (y1/ratio - c.Offset[1]) / c.Zoom}
	pixelCamera := ink.Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}}
	next := 1 - a.dryFront
	previous := a.dry[a.dryFront]
	a.dry[next].Draw(p, func(cp *ui.Painter) {
		cp.Image(previous.Bitmap(), ui.Rect{W: float32(a.dryW), H: float32(a.dryH)}, ui.FillBox)
		cp.Clip(region, 0, func() {
			cp.Fill(region, ui.RGB(250, 249, 245), 0)
			for _, s := range a.strokes {
				if ink.BoundsOverlap(ink.StrokeBounds(s), visible) {
					a.renderer.DrawRegion(cp, s, pixelCamera, region)
				}
			}
		})
	})
	a.dryFront = next
	a.bakedCount = len(a.strokes)
}
