// Package inkcanvas adapts the local ink-demo controller for embedded native UI.
// Renderer, smoothing, pressure, selection and cache algorithms come from that
// independent demo; it remains runnable and unchanged.
package inkcanvas

import (
	"time"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

const zoomPreviewVectorLimit = 256
const zoomPreviewWorkLimit = 1024

type Editor struct {
	surfaceState
	frameTrace                                                     frameTrace
	inputTrace                                                     inputTrace
	fullIdleRecoveries, retainedIdleRecoveries                     uint64
	homeZoom, minZoom, maxZoom, zoomInputPower                     float64
	homeX, homeY                                                   float64
	dark                                                           bool
	background                                                     ui.Color
	redo                                                           []strokeEdit
	renderer                                                       *ink.Renderer
	strokes                                                        []ink.Stroke
	current                                                        *ink.Stroke
	tool                                                           ink.Tool
	width, ratio                                                   float32
	pointer                                                        ui.PointerType
	showTools, paletteReady, panelExpanded                         bool
	paletteMode, brushSelect, penSelect, eraserSelect, colorSelect int
	penWidth, eraserWidth                                          float64
	panelX, panelY, viewportW, viewportH                           float32
	panelDock, panelOldDock                                        int
	panelDragging                                                  bool
	panelDragOffset, panelDragStart                                ui.Point
	strokeBall                                                     bool
	undo                                                           []strokeEdit
	erasing, eraseChanged                                          bool
	eraseKind                                                      int
	erasePath                                                      []ink.Point
	eraseBefore                                                    []ink.Stroke
	hoverPoint                                                     ui.Point
	hasHover                                                       bool
	zoom, offsetX, offsetY                                         float64
	spaceDown, handMode, panning, panelCollapsed                   bool
	panAnchor                                                      ui.Point
	panBefore                                                      [2]float64
	strokeIndex                                                    *ink.HitIndex
	strokeRemoved                                                  map[int]bool
	eraseCursor                                                    int
	strokeHitTests                                                 int
	selection                                                      lassoSelection
	previousTool                                                   toolState
	hasPreviousTool                                                bool
	editRevision                                                   uint64
	zoomChangedAt                                                  time.Time
	boundsCache                                                    [4]float32
	boundsRevision                                                 uint64
	boundsValid                                                    bool
}

func (a *Editor) input(e ui.InputEvent) bool {
	defer a.ObservePhase("input", time.Now())
	// Defend direct controller callers too; native capture isolation happens
	// before MyGo can move the cursor, release capture or change focus.
	if a.Busy() && a.pointer == ui.PointerPen && !e.Canceled && e.Pointer != ui.PointerPen &&
		(e.Kind == ui.InputPointerDown || e.Kind == ui.InputPointerMove || e.Kind == ui.InputPointerUp) {
		a.noteForeignInput(e)
		return true
	}
	a.traceInput(e)
	if e.Kind == ui.InputPointerDown && !a.Busy() {
		a.pointer = e.Pointer
	}

	if e.Kind == ui.InputKeyDown && e.Key == ui.KeyEscape && (a.selectionActive() || len(a.selection.ids) > 0) {
		if a.selection.moving {
			for _, id := range a.selection.ids {
				a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.strokes[id]), ink.StrokeBounds(a.selection.before[id]))
			}
			a.strokes = a.selection.before
			a.editRevision++
		}
		a.clearSelection()
		return true
	}
	if e.Kind == ui.InputPointerDown {
		a.panelExpanded = false
	}
	if e.Kind == ui.InputPointerDown || e.Kind == ui.InputPointerMove || e.Kind == ui.InputPointerUp {
		a.hoverPoint = ui.Point{X: e.X, Y: e.Y}
		a.hasHover = true
	}
	if a.navigate(e) {
		return true
	}
	if e.Kind == ui.InputPointerDown || e.Kind == ui.InputPointerMove || e.Kind == ui.InputPointerUp {
		world := a.toWorld(e.X, e.Y)
		e.X = world.X
		e.Y = world.Y
	}
	if a.selectionActive() || a.paletteMode == 2 {
		return a.lassoInput(e)
	}
	if a.erasing || a.paletteMode == 1 {
		taken := a.eraseInput(e)
		return taken || e.Kind == ui.InputPointerMove
	}
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 {
			return false
		}
		a.commitCurrent() // a repeated down must not discard already observed pen samples
		a.panelExpanded = false
		a.clearSelection()
		a.pointer = e.Pointer
		tool := a.tool
		a.strokeBall = (tool == ink.Pen && a.penSelect == 1) || a.pointer != ui.PointerPen
		color := a.currentColor()
		if tool == ink.Highlighter {
			if a.colorSelect == 0 {
				color = [4]float32{1, 0.8, 0.12, 0.45}
			} else {
				color[3] = 0.45
			}
		}
		a.live.Reset()
		a.current = &ink.Stroke{Tool: tool, Color: color, Smooth: true}
		a.add(e)
		return true
	case ui.InputPointerMove:
		if a.current != nil {
			a.add(e)
			return true
		}
	case ui.InputPointerUp:
		if e.Canceled {
			// Keep only samples actually received before capture loss. Never append
			// the cancel location or reconnect this fragment on the next press.
			a.commitCurrent()
			return true
		}
		if a.current != nil {
			a.add(e)
			a.commitCurrent()
			return true
		}
	}
	return false
}

// add uses only actual position and (for a tablet pen) actual pressure.
// Event cadence and movement speed never affect width.
func (a *Editor) add(e ui.InputEvent) {
	radius := max(0, a.width/2)
	n := len(a.current.Points)
	if a.pointer == ui.PointerPen && !a.strokeBall && a.current.Tool != ink.Highlighter {
		pressure := max(0, min(1, e.Pressure))
		// Nominal width is the full-pressure width; zero is valid, never replaced
		// by a made-up 0.5 pressure. No temporal or spatial pressure smoothing.
		radius *= 0.2 + 0.8*pressure
		// A release often reports pressure=0. Preserve the last contact width
		// while taking its final position, rather than inventing a pressure dip.
		if e.Kind == ui.InputPointerUp && n > 0 {
			radius = a.current.Points[n-1].Radius
		}
	}
	point := ink.Point{X: e.X, Y: e.Y, Radius: radius}
	// Keep subpixel positions and pressure-only changes; drop exact duplicates.
	if n > 0 && point == a.current.Points[n-1] {
		return
	}
	a.current.Points = append(a.current.Points, point)
}

// drawZoomPreview stretches the last full-resolution viewport to the new
// camera. It is a temporary image; the next settled frame bakes vectors at
// the new zoom.
func (a *Editor) drawZoomPreview(p *ui.Painter, rect ui.Rect, c ink.Camera) {
	if a.dry[a.dryFront] == nil || a.dryCamera.Zoom <= 0 {
		return
	}
	old := a.dryCamera
	zr := c.Zoom / old.Zoom
	wx := (rect.X - old.Offset[0]) / old.Zoom
	wy := (rect.Y - old.Offset[1]) / old.Zoom
	dest := ui.Rect{
		X: wx*c.Zoom + c.Offset[0],
		Y: wy*c.Zoom + c.Offset[1],
		W: rect.W * zr,
		H: rect.H * zr,
	}

	// Draw the retained viewport first. Only the genuinely new area outside
	// it gets paper and vectors; filling the whole viewport here used to hide
	// ink that a zoom-out had just exposed.
	p.Clip(rect, 0, func() { p.Image(a.dry[a.dryFront].Bitmap(), dest, ui.FillBox) })
	bands := outsideBands(rect, dest)
	if len(bands) == 0 {
		a.keepIdleSurface(a.dry[a.dryFront], rect, rect, a.dryCamera)
		return
	}
	// Check complexity budgets strictly against strokes overlapping the newly
	// exposed edge bands rather than the entire document, reusing the synced
	// spatial index instead of rebuilding panIndex every zoom frame.
	bandStrokes, ok := a.collectPreviewBandStrokes(bands, c)
	if !ok {
		for _, band := range bands {
			p.Clip(band, 0, func() { a.paintPaper(p, band, c) })
		}
		a.keepIdleSurface(a.dry[a.dryFront], rect, rect, a.dryCamera)
		return
	}
	for i, band := range bands {
		ids := bandStrokes[i]
		p.Clip(band, 0, func() {
			a.paintPaper(p, band, c)
			for _, id := range ids {
				a.renderer.DrawRegion(p, a.renderStroke(a.tiles.sources[id].stroke), c, band)
			}
		})
	}
	a.keepIdleSurface(a.dry[a.dryFront], rect, rect, a.dryCamera)
}

func outsideBands(rect, image ui.Rect) []ui.Rect {
	rx0, ry0 := rect.X, rect.Y
	rx1, ry1 := rect.X+rect.W, rect.Y+rect.H
	ix0, iy0 := max(rx0, image.X), max(ry0, image.Y)
	ix1, iy1 := min(rx1, image.X+image.W), min(ry1, image.Y+image.H)
	if ix0 >= ix1 || iy0 >= iy1 {
		return []ui.Rect{rect}
	}
	out := make([]ui.Rect, 0, 4)
	if iy0 > ry0 {
		out = append(out, ui.Rect{X: rx0, Y: ry0, W: rect.W, H: iy0 - ry0})
	}
	if iy1 < ry1 {
		out = append(out, ui.Rect{X: rx0, Y: iy1, W: rect.W, H: ry1 - iy1})
	}
	if ix0 > rx0 {
		out = append(out, ui.Rect{X: rx0, Y: iy0, W: ix0 - rx0, H: iy1 - iy0})
	}
	if ix1 < rx1 {
		out = append(out, ui.Rect{X: ix1, Y: iy0, W: rx1 - ix1, H: iy1 - iy0})
	}
	return out
}

// PreviewingZoom reports that the next frame should settle the preview.
func (a *Editor) PreviewingZoom() bool { return a.zoomPreview }

// RequestRecompose asks for a progressive rebake at the current camera. It
// keeps the last complete viewport visible instead of blocking one frame.
func (a *Editor) RequestRecompose() {
	a.recomposeGeneration++
	if a.dry[a.dryFront] != nil && !a.dryInvalid {
		a.recompose = true
	}
}

// Committing a fragment on system cancellation preserves observed pen data,
// while an explicit tool/navigation Cancel still discards an unfinished mark.
func (a *Editor) commitCurrent() {
	if a.current == nil {
		return
	}
	if len(a.current.Points) > 0 {
		if a.current.Tool == ink.Pencil {
			a.current.Stamps = a.live.PencilStamps(*a.current)
		}
		a.pushEdit(strokeEdit{at: len(a.strokes), added: []ink.Stroke{*a.current}})
		a.strokes = append(a.strokes, *a.current)
	}
	a.current = nil
	a.live.Reset()
}
