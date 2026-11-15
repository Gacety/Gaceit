package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"slices"
	"study408/internal/ink"
)

func (a *Editor) HasSelection() bool { return len(a.selection.ids) > 0 && !a.Busy() }
func (a *Editor) CopySelection() []ink.Stroke {
	if !a.HasSelection() {
		return nil
	}
	out := make([]ink.Stroke, 0, len(a.selection.ids))
	for _, id := range a.selection.ids {
		out = append(out, cloneStroke(a.strokes[id]))
	}
	return out
}
func cloneStroke(s ink.Stroke) ink.Stroke {
	s.Points = slices.Clone(s.Points)
	s.Stamps = slices.Clone(s.Stamps)
	paths := make([][]ink.Point, len(s.Erasures))
	for i, path := range s.Erasures {
		paths[i] = slices.Clone(path)
	}
	s.Erasures = paths
	return s
}
func translatedStroke(s ink.Stroke, delta ui.Point) ink.Stroke {
	s.Translation[0] += delta.X
	s.Translation[1] += delta.Y
	if len(s.Erasures) > 0 {
		paths := make([][]ink.Point, len(s.Erasures))
		for i, path := range s.Erasures {
			paths[i] = slices.Clone(path)
			for j := range paths[i] {
				paths[i][j].X += delta.X
				paths[i][j].Y += delta.Y
			}
		}
		s.Erasures = paths
	}
	return s
}
func (a *Editor) DeleteSelection() bool {
	if !a.HasSelection() {
		return false
	}
	remove := map[int]bool{}
	for _, id := range a.selection.ids {
		remove[id] = true
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.strokes[id]))
	}
	kept := make([]ink.Stroke, 0, len(a.strokes)-len(remove))
	for i, s := range a.strokes {
		if !remove[i] {
			kept = append(kept, s)
		}
	}
	removed := make([]indexedStroke, 0, len(remove))
	for i, s := range a.strokes {
		if remove[i] {
			removed = append(removed, indexedStroke{i, s})
		}
	}
	a.strokes = kept
	a.pushEdit(strokeEdit{removed: removed})
	a.clearSelection()
	a.live.Reset()
	return true
}
func (a *Editor) Paste(strokes []ink.Stroke, center ui.Point) bool {
	if a.Busy() || len(strokes) == 0 {
		return false
	}
	var bounds [4]float32
	for i, s := range strokes {
		b := ink.StrokeBounds(s)
		if i == 0 {
			bounds = b
		} else {
			bounds = [4]float32{min(bounds[0], b[0]), min(bounds[1], b[1]), max(bounds[2], b[2]), max(bounds[3], b[3])}
		}
	}
	for _, v := range bounds {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	delta := ui.Point{X: center.X - (bounds[0]/2 + bounds[2]/2), Y: center.Y - (bounds[1]/2 + bounds[3]/2)}
	pasted := make([]ink.Stroke, len(strokes))
	for i, s := range strokes {
		pasted[i] = translatedStroke(cloneStroke(s), delta)
		pasted[i].Color = AdaptiveColor(pasted[i].Color, a.dark)
	}
	a.SetTool("lasso")
	// Pending deletion repairs only cover their old bounds; a simultaneous
	// paste must also paint new strokes outside those regions.
	if len(a.dirtyRegions) > 0 {
		a.dryInvalid = true
	}
	start := len(a.strokes)
	a.strokes = append(a.strokes, pasted...)
	a.pushEdit(strokeEdit{at: start, added: pasted})
	a.selection.ids = make([]int, len(pasted))
	for i := range pasted {
		a.selection.ids[i] = start + i
	}
	a.updateSelectionBounds()
	return true
}

// PasteCenter places a copied group near its source when it is still visible;
// otherwise it uses the current view center. Offset repeated copies slightly.
func (a *Editor) PasteCenter(bounds [4]float32, offset float32) ui.Point {
	v := a.ViewBounds()
	x, y := bounds[0]/2+bounds[2]/2, bounds[1]/2+bounds[3]/2
	if x < v[0] || x > v[2] || y < v[1] || y > v[3] {
		x, y = (v[0]+v[2])/2, (v[1]+v[3])/2
	}
	return ui.Point{X: x + offset/float32(a.viewZoom()), Y: y + offset/float32(a.viewZoom())}
}
