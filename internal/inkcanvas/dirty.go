package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
)

// Repair only the removed strokes' visible area. Cache textures are reused;
// unaffected pixels are copied and strokes outside that area do not rebuild SDF.
func (a *Editor) repairDry(p *ui.Painter, c ink.Camera) {
	ratio := c.PixelRatio
	regions := make([]ui.Rect, 0, len(a.dirtyRegions))
	for _, world := range a.dirtyRegions {
		x0 := max(float32(0), float32(math.Floor(float64((world[0]*c.Zoom+c.Offset[0])*ratio)))-2)
		y0 := max(float32(0), float32(math.Floor(float64((world[1]*c.Zoom+c.Offset[1])*ratio)))-2)
		x1 := min(float32(a.dryW), float32(math.Ceil(float64((world[2]*c.Zoom+c.Offset[0])*ratio)))+2)
		y1 := min(float32(a.dryH), float32(math.Ceil(float64((world[3]*c.Zoom+c.Offset[1])*ratio)))+2)
		if x1 > x0 && y1 > y0 {
			regions = append(regions, ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0})
		}
	}
	if len(regions) == 0 {
		a.bakedCount = len(a.strokes)
		return
	}
	if a.transparentInk && len(regions) > 1 {
		merged := regions[0]
		for _, r := range regions[1:] {
			merged = unionRect(merged, r)
		}
		regions = []ui.Rect{merged}
	}
	pixelCamera := ink.Camera{Zoom: c.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{c.Offset[0] * ratio, c.Offset[1] * ratio}}
	a.tiles.syncFrame(a, pixelCamera, true)
	a.tiles.repair(p, a, regions, pixelCamera)
	next := 1 - a.dryFront
	if a.dry[next] == nil {
		a.dry[next] = ui.NewCanvas(a.dryW, a.dryH)
	}
	previous := a.dry[a.dryFront]
	a.dry[next].Draw(p, func(cp *ui.Painter) {
		full := ui.Rect{W: float32(a.dryW), H: float32(a.dryH)}
		if a.transparentInk {
			copyOutside(cp, previous.Bitmap(), full, regions)
		} else {
			cp.Image(previous.Bitmap(), full, ui.FillBox)
		}
		for _, region := range regions {
			cp.Clip(region, 0, func() {
				a.tiles.paint(cp, a, region, pixelCamera)
			})
		}
	})
	a.dryFront = next
	a.bakedCount = len(a.strokes)
}
