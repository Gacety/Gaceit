//go:build darwin && study408_gpuqa

package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

var study408DirectPan bool

// Recreates the previous indexed strip-vector painter for same-backend A/B QA.
func (s *panSurface) study408PaintDirect(p *ui.Painter, a *Editor, regions []ui.Rect) {
	camera := ink.Camera{Zoom: s.zoom * s.ratio, PixelRatio: 1, Offset: [2]float32{float32(s.offset[0]*float64(s.ratio)) + panCachePadding, float32(s.offset[1]*float64(s.ratio)) + panCachePadding}}
	for _, region := range regions {
		if region.W <= 0 || region.H <= 0 {
			continue
		}
		visible := [4]float32{(region.X - camera.Offset[0]) / camera.Zoom, (region.Y - camera.Offset[1]) / camera.Zoom, (region.X + region.W - camera.Offset[0]) / camera.Zoom, (region.Y + region.H - camera.Offset[1]) / camera.Zoom}
		p.Clip(region, 0, func() {
			a.paintPaper(p, region, camera)
			for _, id := range a.tiles.index.query(visible, a.tiles.sources) {
				a.renderer.DrawRegion(p, a.renderStroke(a.tiles.sources[id].stroke), camera, region)
			}
		})
	}
}
