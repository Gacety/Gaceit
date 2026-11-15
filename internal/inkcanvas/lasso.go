package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

type lassoSelection struct {
	path            []ink.Point
	ids             []int
	bounds          [4]float32
	drawing, moving bool
	anchor          ui.Point
	delta, applied  ui.Point
	before          []ink.Stroke

	// Transient drag layer: base excludes the selected strokes, layer holds
	// them at zero delta. Movement draws two stable images instead of repairing
	// the base every pointer sample.
	preview       bool
	commitPending bool
	base          *ui.Canvas
	layers        []selectionPreviewLayer
	directRange   bool
	rangeStart    int
	selectionSet  map[int]bool
	previewRect   ui.Rect
	previewCamera ink.Camera
	previewDark   bool
}

func (a *Editor) selectionActive() bool { return a.selection.drawing || a.selection.moving }
func (a *Editor) clearSelection()       { a.selection = lassoSelection{} }
func (s *lassoSelection) resetPreview() {
	s.preview = false
	s.commitPending = false
	s.base = nil
	s.layers = nil
	s.directRange = false
	s.rangeStart = 0
	s.selectionSet = nil
}

// Discard only raster resources; a queued pointer-up still has to commit.
func (s *lassoSelection) releasePreview() {
	pending := s.commitPending
	s.resetPreview()
	s.commitPending = pending
}
func (a *Editor) selectionContains(x, y float32) bool {
	s := &a.selection
	pad := float32(6 / a.viewZoom())
	return len(s.ids) > 0 && x >= s.bounds[0]-pad && y >= s.bounds[1]-pad && x <= s.bounds[2]+pad && y <= s.bounds[3]+pad
}
func (a *Editor) lassoInput(e ui.InputEvent) bool {
	s := &a.selection
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 {
			return false
		}
		if a.selectionActive() {
			return true
		}
		if a.selectionContains(e.X, e.Y) {
			s.resetPreview()
			s.moving = true
			s.anchor = ui.Point{X: e.X, Y: e.Y}
			s.delta, s.applied = ui.Point{}, ui.Point{}
			s.before = append([]ink.Stroke(nil), a.strokes...)
		} else {
			a.clearSelection()
			s.drawing = true
			s.path = []ink.Point{{X: e.X, Y: e.Y}}
		}
	case ui.InputPointerMove, ui.InputPointerUp:
		if e.Canceled {
			if s.moving {
				a.strokes = s.before
				a.dryInvalid = true
				a.editRevision++
			}
			a.clearSelection()
			return true
		}
		if s.moving {
			s.delta = ui.Point{X: e.X - s.anchor.X, Y: e.Y - s.anchor.Y}
			if e.Kind == ui.InputPointerUp {
				if s.preview {
					s.commitPending = true
				} else {
					a.processSelectionMove()
					if s.applied != (ui.Point{}) {
						a.remember(s.before)
					}
					s.moving = false
					s.before = nil
				}
			}
		} else if s.drawing {
			last := s.path[len(s.path)-1]
			dx, dy := (e.X-last.X)*float32(a.viewZoom()), (e.Y-last.Y)*float32(a.viewZoom())
			if dx*dx+dy*dy >= 4 || e.Kind == ui.InputPointerUp {
				s.path = append(s.path, ink.Point{X: e.X, Y: e.Y})
			}
			if e.Kind == ui.InputPointerUp {
				s.ids = ink.NewHitIndex(a.strokes).SelectPolygon(s.path)
				s.drawing = false
				s.path = nil
				a.updateSelectionBounds()
			}
		}
	default:
		return false
	}
	return true
}

// Coalesce pointer motion into one local cache repair per rendered frame.
// Points and stamp data remain shared and immutable for exact undo.
func (a *Editor) processSelectionMove() {
	s := &a.selection
	if !s.moving || s.delta == s.applied {
		return
	}
	for _, id := range s.ids {
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.strokes[id]))
		stroke := translatedStroke(s.before[id], s.delta)
		stroke.Color = AdaptiveColor(stroke.Color, a.dark)
		a.strokes[id] = stroke
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(stroke))
	}
	s.applied = s.delta
	a.updateSelectionBounds()
}
func (a *Editor) updateSelectionBounds() {
	s := &a.selection
	for i, id := range s.ids {
		b := ink.StrokeBounds(a.strokes[id])
		if i == 0 {
			s.bounds = b
			continue
		}
		s.bounds = [4]float32{min(s.bounds[0], b[0]), min(s.bounds[1], b[1]), max(s.bounds[2], b[2]), max(s.bounds[3], b[3])}
	}
}
func (a *Editor) drawSelection(p *ui.Painter, camera ink.Camera) {
	if a.paletteMode != 2 {
		return
	}
	s := &a.selection
	var path ui.Path
	point := func(x, y float32, first bool) {
		x, y = x*camera.Zoom+camera.Offset[0], y*camera.Zoom+camera.Offset[1]
		if first {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	if s.drawing && len(s.path) > 0 {
		for i, pt := range s.path {
			point(pt.X, pt.Y, i == 0)
		}
		path.Close()
	} else if len(s.ids) > 0 {
		pad := float32(5) / camera.Zoom
		b := s.bounds
		if s.moving {
			b[0] += s.delta.X
			b[1] += s.delta.Y
			b[2] += s.delta.X
			b[3] += s.delta.Y
		}
		point(b[0]-pad, b[1]-pad, true)
		point(b[2]+pad, b[1]-pad, false)
		point(b[2]+pad, b[3]+pad, false)
		point(b[0]-pad, b[3]+pad, false)
		path.Close()
	} else {
		return
	}
	p.StrokePath(&path, 3, a.background)
	p.StrokePath(path.Dash(0, 6, 4), 1.3, p.Theme().Accent)
}
