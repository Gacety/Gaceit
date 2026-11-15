package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

// Conservative cheap estimate, capped before scanning many erasure paths.
// Sampling and pressure remain untouched. An individual stroke stays atomic
// to preserve maximum coverage and translucent blending at curve joins.
func strokeWork(s ink.Stroke, limit int) int {
	// Imported pencil strokes without explicit stamps may generate far more
	// stamps than there are samples; never treat them as cheap preview work.
	if s.Tool == ink.Pencil && s.Stamps == nil && len(s.Points) > 0 {
		return limit + 1
	}
	n := len(s.Points) + len(s.Stamps)
	if n > limit {
		return limit + 1
	}
	for _, path := range s.Erasures {
		n += len(path)
		if n > limit {
			return limit + 1
		}
	}
	return max(1, n)
}
func (a *Editor) previewVectorsAffordable() bool {
	if len(a.strokes) > zoomPreviewVectorLimit {
		return false
	}
	budget := zoomPreviewWorkLimit
	for _, s := range a.strokes {
		budget -= strokeWork(s, budget)
		if budget < 0 {
			return false
		}
	}
	return true
}

func (a *Editor) collectPreviewBandStrokes(bands []ui.Rect, c ink.Camera) ([][]int, bool) {
	if len(bands) == 0 || len(a.strokes) == 0 || c.Zoom <= 0 {
		return nil, false
	}
	if !(a.tiles.synced && a.tiles.syncedRevision == a.editRevision && len(a.tiles.sources) == len(a.strokes)) {
		scale := a.tiles.scale
		syncCam := ink.Camera{Zoom: scale, Offset: a.tiles.phase}
		if a.tiles.entries == nil || scale <= 0 {
			dryScale := a.dryCamera.Zoom * c.PixelRatio
			if dryScale <= 0 {
				dryScale = c.Zoom * c.PixelRatio
			}
			syncCam = ink.Camera{
				Zoom:   dryScale,
				Offset: [2]float32{a.dryCamera.Offset[0] * c.PixelRatio, a.dryCamera.Offset[1] * c.PixelRatio},
			}
		}
		a.tiles.syncFrame(a, syncCam, false)
	}
	seen := make(map[int]struct{})
	bandStrokes := make([][]int, len(bands))
	count := 0
	budget := zoomPreviewWorkLimit
	for i, band := range bands {
		wb := [4]float32{
			(band.X - c.Offset[0]) / c.Zoom,
			(band.Y - c.Offset[1]) / c.Zoom,
			(band.X + band.W - c.Offset[0]) / c.Zoom,
			(band.Y + band.H - c.Offset[1]) / c.Zoom,
		}
		ids := a.tiles.index.query(wb, a.tiles.sources)
		if len(ids) == 0 {
			continue
		}
		copied := make([]int, len(ids))
		copy(copied, ids)
		bandStrokes[i] = copied
		for _, id := range copied {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			count++
			if count > zoomPreviewVectorLimit {
				return nil, false
			}
			budget -= strokeWork(a.strokes[id], budget)
			if budget < 0 {
				return nil, false
			}
		}
	}
	return bandStrokes, true
}
