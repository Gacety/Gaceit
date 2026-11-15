package main

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
}

func (a *app) selectionActive() bool { return a.selection.drawing || a.selection.moving }
func (a *app) clearSelection()       { a.selection = lassoSelection{} }
func (a *app) selectionContains(x, y float32) bool {
	s := &a.selection
	pad := float32(6 / a.viewZoom())
	return len(s.ids) > 0 && x >= s.bounds[0]-pad && y >= s.bounds[1]-pad && x <= s.bounds[2]+pad && y <= s.bounds[3]+pad
}
func (a *app) lassoInput(e ui.InputEvent) bool {
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
			}
			a.clearSelection()
			return true
		}
		if s.moving {
			s.delta = ui.Point{X: e.X - s.anchor.X, Y: e.Y - s.anchor.Y}
			if e.Kind == ui.InputPointerUp {
				a.processSelectionMove()
				if s.applied != (ui.Point{}) {
					a.remember(s.before)
				}
				s.moving = false
				s.before = nil
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
func (a *app) processSelectionMove() {
	s := &a.selection
	if !s.moving || s.delta == s.applied {
		return
	}
	for _, id := range s.ids {
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.strokes[id]))
		stroke := s.before[id]
		stroke.Translation[0] += s.delta.X
		stroke.Translation[1] += s.delta.Y
		if len(stroke.Erasures) > 0 {
			paths := make([][]ink.Point, len(stroke.Erasures))
			for i, path := range stroke.Erasures {
				paths[i] = append([]ink.Point(nil), path...)
				for j := range paths[i] {
					paths[i][j].X += s.delta.X
					paths[i][j].Y += s.delta.Y
				}
			}
			stroke.Erasures = paths
		}
		a.strokes[id] = stroke
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(stroke))
	}
	s.applied = s.delta
	a.updateSelectionBounds()
}
func (a *app) updateSelectionBounds() {
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
func (a *app) drawSelection(p *ui.Painter, camera ink.Camera) {
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
		point(b[0]-pad, b[1]-pad, true)
		point(b[2]+pad, b[1]-pad, false)
		point(b[2]+pad, b[3]+pad, false)
		point(b[0]-pad, b[3]+pad, false)
		path.Close()
	} else {
		return
	}
	p.StrokePath(&path, 3, ui.RGB(250, 249, 245))
	p.StrokePath(path.Dash(0, 6, 4), 1.3, ui.RGB(51, 131, 165))
}
