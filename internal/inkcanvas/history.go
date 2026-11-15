package inkcanvas

import "study408/internal/ink"

type indexedStroke struct {
	index  int
	stroke ink.Stroke
}
type strokeChange struct {
	index         int
	before, after ink.Stroke
}
type strokeEdit struct {
	at      int
	added   []ink.Stroke
	removed []indexedStroke
	changed []strokeChange
}

// History owns only affected stroke headers. Samples/stamps/erasures remain
// immutable and shared; pointer-down never clones the whole document for ink.
func (a *Editor) pushEdit(edit strokeEdit) {
	clear(a.redo)
	a.redo = nil
	if len(a.undo) == 50 {
		copy(a.undo, a.undo[1:])
		a.undo[len(a.undo)-1] = edit
	} else {
		a.undo = append(a.undo, edit)
	}
	a.editRevision++
}
func sameStroke(a, b ink.Stroke) bool {
	return a.Tool == b.Tool && a.Color == b.Color && a.Smooth == b.Smooth && a.Translation == b.Translation && sameSlice(a.Points, b.Points) && sameSlice(a.Stamps, b.Stamps) && sameSlice(a.Erasures, b.Erasures)
}
func sameSlice[T any](a, b []T) bool { return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0]) }

// Complex gestures may keep a temporary cancellation snapshot, but only their
// sparse changes enter the bounded undo stack once the gesture completes.
func (a *Editor) remember(before []ink.Stroke) {
	edit := strokeEdit{}
	if len(before) == len(a.strokes) {
		for i, s := range before {
			if !sameStroke(s, a.strokes[i]) {
				edit.changed = append(edit.changed, strokeChange{i, s, a.strokes[i]})
			}
		}
	} else if len(before) > len(a.strokes) {
		next := 0
		for i, s := range before {
			if next < len(a.strokes) && sameStroke(s, a.strokes[next]) {
				next++
			} else {
				edit.removed = append(edit.removed, indexedStroke{i, s})
			}
		}
	} else {
		edit.at = len(before)
		edit.added = append([]ink.Stroke(nil), a.strokes[len(before):]...)
	}
	if len(edit.added)+len(edit.removed)+len(edit.changed) > 0 {
		a.pushEdit(edit)
	}
}
func (a *Editor) applyEdit(edit strokeEdit, reverse bool) {
	a.clearSelection()
	if len(edit.added) > 0 {
		if reverse {
			clear(a.strokes[edit.at:])
			a.strokes = a.strokes[:edit.at]
		} else {
			start := len(a.strokes)
			a.strokes = append(a.strokes, edit.added...)
			for i := start; i < len(a.strokes); i++ {
				a.strokes[i].Color = AdaptiveColor(a.strokes[i].Color, a.dark)
			}
		}
	}
	if len(edit.removed) > 0 {
		if reverse {
			restored := make([]ink.Stroke, len(a.strokes)+len(edit.removed))
			old, removed := 0, 0
			for i := range restored {
				if removed < len(edit.removed) && edit.removed[removed].index == i {
					restored[i] = edit.removed[removed].stroke
					restored[i].Color = AdaptiveColor(restored[i].Color, a.dark)
					removed++
				} else {
					restored[i] = a.strokes[old]
					old++
				}
			}
			a.strokes = restored
		} else {
			kept := make([]ink.Stroke, 0, len(a.strokes)-len(edit.removed))
			removed := 0
			for i, s := range a.strokes {
				if removed < len(edit.removed) && edit.removed[removed].index == i {
					removed++
				} else {
					kept = append(kept, s)
				}
			}
			a.strokes = kept
		}
	}
	for _, change := range edit.changed {
		if reverse {
			a.strokes[change.index] = change.before
		} else {
			a.strokes[change.index] = change.after
		}
		a.strokes[change.index].Color = AdaptiveColor(a.strokes[change.index].Color, a.dark)
	}
	// Both sides of the edit can expose underlying ink. Repair their union
	// in original paint order instead of invalidating the complete viewport.
	for _, stroke := range edit.added {
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(stroke))
	}
	for _, removed := range edit.removed {
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(removed.stroke))
	}
	for _, change := range edit.changed {
		a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(change.before), ink.StrokeBounds(change.after))
	}
	a.live.Reset()
	a.editRevision++
}
func (a *Editor) Undo() {
	if !a.CanUndo() {
		return
	}
	i := len(a.undo) - 1
	edit := a.undo[i]
	a.undo[i] = strokeEdit{}
	a.undo = a.undo[:i]
	a.applyEdit(edit, true)
	a.redo = append(a.redo, edit)
}
func (a *Editor) Redo() {
	if !a.CanRedo() {
		return
	}
	i := len(a.redo) - 1
	edit := a.redo[i]
	a.redo[i] = strokeEdit{}
	a.redo = a.redo[:i]
	a.applyEdit(edit, false)
	a.undo = append(a.undo, edit)
}
