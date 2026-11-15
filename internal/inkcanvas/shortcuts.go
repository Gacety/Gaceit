package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

// Tool history is shared by toolbar clicks and keyboard changes. Brush widths,
// colors, pen subtype and eraser subtype stay at their last chosen settings.
type toolState struct {
	mode, brush int
	hand        bool
}

func (a *Editor) toolState() toolState {
	return toolState{mode: a.paletteMode, brush: a.brushSelect, hand: a.handMode}
}

func (a *Editor) switchTool(next toolState) {
	if a.current != nil || a.erasing || a.selectionActive() || a.panning || a.panelDragging {
		return
	}
	if current := a.toolState(); current != next {
		a.previousTool, a.hasPreviousTool = current, true
		a.paletteMode, a.brushSelect, a.handMode = next.mode, next.brush, next.hand
		a.tool = ink.Tool(next.brush)
		a.clearSelection()
	}
	a.panelExpanded = false
}

func (a *Editor) toolShortcuts(c *ui.Context) {
	for _, key := range []ui.Key{ui.KeyP, ui.KeyE} {
		if !c.Shortcut(0, key) || c.ShortcutRepeated() {
			continue
		}
		next := a.toolState()
		next.hand = false
		next.mode = 0
		if key == ui.KeyE {
			next.mode = 1
		}
		if current := a.toolState(); !current.hand && current.mode == next.mode {
			if a.hasPreviousTool {
				next = a.previousTool
			} else {
				// With no prior switch, the counterpart is a useful default.
				next.mode = 1 - next.mode
			}
		}
		a.switchTool(next)
	}
}
