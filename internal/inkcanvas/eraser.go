package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

func (a *Editor) eraseInput(e ui.InputEvent) bool {
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 {
			return false
		}
		a.clearSelection()
		a.erasing = true
		a.eraseBefore = append([]ink.Stroke(nil), a.strokes...)
		a.erasePath = nil
		a.erasePreview.Reset()
		a.eraseChanged = false
		a.eraseKind = a.eraserSelect
		a.strokeIndex = nil
		a.strokeRemoved = map[int]bool{}
		a.eraseCursor = 0
		a.strokeHitTests = 0
		a.eraseAt(e.X, e.Y)
		return true
	case ui.InputPointerMove:
		if a.erasing {
			a.eraseAt(e.X, e.Y)
			return true
		}
	case ui.InputPointerUp:
		if !a.erasing {
			return false
		}
		if e.Canceled {
			if a.eraseKind == 0 {
				for id := range a.strokeRemoved {
					a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.eraseBefore[id]))
				}
				a.strokes = a.eraseBefore
				a.editRevision++
			}
		} else {
			a.eraseAt(e.X, e.Y)
			if a.eraseKind == 1 {
				a.commitPixelErase()
			} else {
				a.processStrokeErase()
			}
			if a.eraseChanged {
				a.remember(a.eraseBefore)
			}
		}
		a.erasing = false
		a.erasePath = nil
		a.eraseBefore = nil
		a.strokeIndex = nil
		a.strokeRemoved = nil
		return true
	}
	return false
}
func (a *Editor) eraseAt(x, y float32) {
	p := ink.Point{X: x, Y: y, Radius: float32(max(1, a.eraserWidth)) / 2}
	if n := len(a.erasePath); n > 0 {
		if p == a.erasePath[n-1] {
			return
		}
	}
	a.erasePath = append(a.erasePath, p)
	// Both erasers retain every sample. Work is coalesced at render-frame cadence.
}

func (a *Editor) processStrokeErase() {
	if !a.erasing || a.eraseKind != 0 || a.eraseCursor >= len(a.erasePath) {
		return
	}
	if a.strokeIndex == nil {
		a.strokeIndex = ink.NewHitIndex(a.eraseBefore)
	}
	changed := false
	for i := a.eraseCursor; i < len(a.erasePath); i++ {
		from := a.erasePath[i]
		if i > 0 {
			from = a.erasePath[i-1]
		}
		for _, id := range a.strokeIndex.HitsSweep(from, a.erasePath[i], a.strokeRemoved) {
			a.strokeRemoved[id] = true
			a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.eraseBefore[id]))
			changed = true
		}
		a.strokeHitTests += a.strokeIndex.CandidateTests
	}
	a.eraseCursor = len(a.erasePath)
	if !changed {
		return
	}
	kept := make([]ink.Stroke, 0, len(a.eraseBefore))
	for id, s := range a.eraseBefore {
		if !a.strokeRemoved[id] {
			kept = append(kept, s)
		}
	}
	a.strokes = kept
	a.eraseChanged = true
}

func (a *Editor) commitPixelErase() {
	if len(a.erasePath) == 0 {
		return
	}
	bounds := ink.SweepBounds(a.erasePath)
	for i, s := range a.strokes {
		if !ink.BoundsOverlap(ink.StrokeBounds(s), bounds) {
			continue
		}
		hit := ink.HitsStroke(s, a.erasePath[0], a.erasePath[0])
		for j := 1; j < len(a.erasePath) && !hit; j++ {
			hit = ink.HitsStroke(s, a.erasePath[j-1], a.erasePath[j])
		}
		if !hit {
			continue
		}
		paths := append([][]ink.Point(nil), s.Erasures...)
		s.Erasures = append(paths, a.erasePath)
		a.strokes[i] = s
		a.eraseChanged = true
	}
	if a.eraseChanged {
		a.dirtyRegions = append(a.dirtyRegions, bounds)
	}
}
