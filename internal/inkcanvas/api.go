package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"slices"
	"study408/internal/ink"
	"time"
)

var inkColors = [][4]float32{{0.08, 0.17, 0.28, 1}, {0.941, 0.706, 0.161, 1}, {0.769, 0.71, 0.992, 1}}

type Document struct {
	Version int          `json:"version"`
	Strokes []ink.Stroke `json:"strokes"`
	Zoom    float64      `json:"zoom"`
	X       float64      `json:"x"`
	Y       float64      `json:"y"`
}

func New() (*Editor, error) {
	r, err := ink.NewRenderer()
	if err != nil {
		return nil, err
	}
	return &Editor{renderer: r, width: 4, penWidth: 4, eraserWidth: 24, ratio: 1, zoom: 1}, nil
}
func (a *Editor) Load(d Document) {
	a.Cancel()
	a.strokes = slices.Clone(d.Strokes)
	a.zoom = d.Zoom
	if a.zoom <= 0 {
		a.zoom = a.HomeZoom()
	}
	lo, hi := a.zoomLimits()
	a.zoom = max(lo, min(hi, a.zoom))
	a.offsetX, a.offsetY = d.X, d.Y
	a.undo = nil
	a.redo = nil
	a.dryInvalid = true
	a.panCache.reset()
	a.editRevision++
	// A document may come from the other theme or from legacy black strokes.
	// Normalize once here rather than walking every stroke on every frame.
	for i := range a.strokes {
		a.strokes[i].Color = AdaptiveColor(a.strokes[i].Color, a.dark)
	}
	a.clearSelection()
}
func (a *Editor) Snapshot() Document {
	return Document{Version: 1, Strokes: slices.Clone(a.strokes), Zoom: a.viewZoom(), X: a.offsetX, Y: a.offsetY}
}
func (a *Editor) Busy() bool {
	return a.current != nil || a.erasing || a.selectionActive() || a.panning
}
func (a *Editor) Cancel() {
	contentChanged := a.erasing || a.selection.moving
	if a.erasing {
		for id := range a.strokeRemoved {
			a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.eraseBefore[id]))
		}
		a.strokes = a.eraseBefore
	}
	if a.selection.moving {
		for _, id := range a.selection.ids {
			a.dirtyRegions = append(a.dirtyRegions, ink.StrokeBounds(a.strokes[id]), ink.StrokeBounds(a.selection.before[id]))
		}
		a.strokes = a.selection.before
	}
	if a.panning {
		a.offsetX, a.offsetY = a.panBefore[0], a.panBefore[1]
	}
	a.inputTrace.last = time.Time{}
	a.current = nil
	a.erasing = false
	a.panning = false
	a.live.Reset()
	a.clearSelection()
	if contentChanged {
		a.editRevision++
	}
}
func AdaptiveColor(c [4]float32, dark bool) [4]float32 {
	neutral := c == inkColors[0] || c == [4]float32{0, 0, 0, 1} || c == [4]float32{1, 1, 1, 1}
	if neutral {
		if dark {
			return [4]float32{1, 1, 1, 1}
		}
		return [4]float32{0, 0, 0, 1}
	}
	return c
}
func (a *Editor) currentColor() [4]float32 { return AdaptiveColor(inkColors[a.colorSelect], a.dark) }
func (a *Editor) Appearance(dark bool, ratio float32) {
	bg := ui.Hex("#faf9f5")
	if dark {
		bg = ui.Hex("#15161C")
	}
	changed := a.dark != dark || a.background != bg
	if changed {
		a.dryInvalid = true
		a.live.Reset()
		a.panCache.reset()
	}
	a.dark = dark
	a.background = bg
	a.ratio = max(1, ratio)
	if !changed {
		return
	}
	// A theme change may adapt neutral ink once. Walking every stroke on each
	// frame made long documents progressively slower while drawing.
	for _, strokes := range [][]ink.Stroke{a.strokes, a.eraseBefore, a.selection.before} {
		for i := range strokes {
			strokes[i].Color = AdaptiveColor(strokes[i].Color, dark)
		}
	}
	if a.current != nil {
		a.current.Color = AdaptiveColor(a.current.Color, dark)
	}
	a.editRevision++
}
func (a *Editor) Input(e ui.InputEvent) bool { return a.input(e) }
func (a *Editor) Canvas(c *ui.Context) ui.Element {
	a.viewportW, a.viewportH = c.Size()
	a.spaceDown = c.KeyDown(ui.KeySpace)
	cursor := ui.CursorCrosshair
	if a.handMode || a.spaceDown {
		cursor = ui.CursorGrab
	}
	if a.panning {
		cursor = ui.CursorGrabbing
	}
	if a.paletteMode == 1 && !a.handMode && !a.spaceDown && !a.panning {
		// The eraser draws its own ring; hiding the system pointer avoids a
		// crosshair sitting inside that ring.
		cursor = ui.CursorNone
	}
	return ui.Box(c).Absolute().Fill().Cursor(cursor).Focusable().Label("无限草稿画布").IsolatePenInput(a.noteForeignInput).Draw(func(p *ui.Painter, r ui.Rect) { a.Draw(p, r) })
}
func (a *Editor) Draw(p *ui.Painter, r ui.Rect) {
	defer a.ObservePhase("frame", time.Now())
	camera := a.camera()
	// While a selection is being dragged, show a stable base without the
	// selected strokes plus a separate translated layer. Commit only on drop.
	if a.selection.moving {
		if a.selection.preview && (a.selection.previewRect != r || a.selection.previewCamera != camera || a.selection.previewDark != a.dark) {
			a.selection.releasePreview()
		}
		if a.selection.commitPending {
			if a.commitSelectionPreview(p) {
				// The new complete dry surface is ready; fall through to normal
				// drawing for the committed selection and overlay.
			} else {
				a.processSelectionMove()
				if a.selection.applied != (ui.Point{}) {
					a.remember(a.selection.before)
				}
				a.selection.moving = false
				a.selection.commitPending = false
				a.selection.before = nil
			}
		}
		if a.selection.moving {
			if !a.selection.preview {
				a.prepareSurface(p, newSurfaceFrame(r, camera), false)
				a.buildSelectionPreview(p, r, camera)
			}
			if a.selection.preview {
				a.drawSelectionPreview(p, r, camera)
				a.drawSelection(p, camera)
				return
			}
		}
	}
	a.processSelectionMove()
	a.processStrokeErase()
	a.drawDry(p, r, camera)
	if a.erasing && a.eraseKind == 1 && len(a.erasePath) > 0 {
		paper := [4]float32{250.0 / 255, 249.0 / 255, 245.0 / 255, 1}
		if a.dark {
			paper = [4]float32{21.0 / 255, 22.0 / 255, 28.0 / 255, 1}
		}
		if a.transparentInk {
			paper = [4]float32{}
		}
		a.erasePreview.Draw(p, a.renderer, a.dry[a.dryFront].Bitmap(), a.erasePath, camera, r.W, r.H, paper)
	}
	if a.current != nil {
		start := time.Now()
		a.live.Draw(p, a.renderer, a.renderStroke(*a.current), camera, r.W, r.H)
		a.ObservePhase("live", start)
	}
	a.drawSelection(p, camera)
	if a.paletteMode == 1 && a.hasHover && !a.handMode && !a.spaceDown && !a.panning {
		diam := float32(a.eraserWidth * a.viewZoom())
		p.Stroke(ui.Rect{X: a.hoverPoint.X - diam/2, Y: a.hoverPoint.Y - diam/2, W: diam, H: diam}, ui.Hex("#A78BFA"), diam/2, 1.5)
	}
}
func (a *Editor) paintPaper(p *ui.Painter, r ui.Rect, c ink.Camera) {
	if !a.transparentInk {
		a.drawPaper(p, r, c)
	}
}
func (a *Editor) drawPaper(p *ui.Painter, r ui.Rect, c ink.Camera) {
	p.Fill(r, a.background, 0)
	step := 26 * c.Zoom
	if step < 8 {
		step *= float32(math.Ceil(float64(8 / step)))
	}
	color := ui.Hex("#d8d7ce")
	if a.dark {
		color = ui.Hex("#33374A")
	}
	rad := min(float32(1.2*c.Zoom), 2*c.PixelRatio)
	x0 := float32(math.Floor(float64((r.X-c.Offset[0])/step)))*step + c.Offset[0]
	y0 := float32(math.Floor(float64((r.Y-c.Offset[1])/step)))*step + c.Offset[1]
	for x := x0; x <= r.X+r.W; x += step {
		for y := y0; y <= r.Y+r.H; y += step {
			p.Fill(ui.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, color, rad)
		}
	}
}
func (a *Editor) SetTool(name string) {
	next := a.toolState()
	next.hand = false
	next.mode = 0
	switch name {
	case "pen":
		next.brush = 0
	case "highlighter":
		next.brush = 1
	case "pencil":
		next.brush = 2
	case "eraser":
		next.mode = 1
	case "lasso":
		next.mode = 2
	case "hand":
		next.hand = true
	}
	a.switchTool(next)
}
func (a *Editor) Tool() string {
	if a.handMode {
		return "hand"
	}
	if a.paletteMode == 1 {
		return "eraser"
	}
	if a.paletteMode == 2 {
		return "lasso"
	}
	return []string{"pen", "highlighter", "pencil"}[a.brushSelect]
}
func (a *Editor) SetWidth(w float32) {
	if !a.Busy() {
		if a.paletteMode == 1 {
			a.eraserWidth = float64(w)
		} else {
			a.width = w
			a.penWidth = float64(w)
		}
	}
}
func (a *Editor) Width() float32 {
	if a.paletteMode == 1 {
		return float32(a.eraserWidth)
	}
	return a.width
}
func (a *Editor) SetColor(index int) {
	if index >= 0 && index < len(inkColors) && !a.Busy() {
		a.colorSelect = index
	}
}
func (a *Editor) ColorIndex() int { return a.colorSelect }
func (a *Editor) SetEraserRegion(region bool) {
	if !a.Busy() {
		a.eraserSelect = 0
		if region {
			a.eraserSelect = 1
		}
	}
}
func (a *Editor) RegionEraser() bool { return a.eraserSelect == 1 }
func (a *Editor) CanUndo() bool      { return !a.Busy() && len(a.undo) > 0 }
func (a *Editor) CanRedo() bool      { return !a.Busy() && len(a.redo) > 0 }
func (a *Editor) Zoom() float64      { return a.viewZoom() }
func (a *Editor) ZoomBy(f float64)   { a.zoomAt(f, a.viewportW/2, a.viewportH/2) }
func (a *Editor) Home() {
	if !a.Busy() {
		a.offsetX = a.homeX
		a.offsetY = a.homeY
		a.zoom = a.HomeZoom()
		a.zoomChangedAt = time.Now()
	}
}
func (a *Editor) Shortcuts(c *ui.Context) { a.toolShortcuts(c) }
func (a *Editor) Selected() ([]ink.Stroke, [4]float32) {
	var out []ink.Stroke
	for _, id := range a.selection.ids {
		out = append(out, a.strokes[id])
	}
	return out, a.selection.bounds
}
func (a *Editor) DrawSnapshot(p *ui.Painter, rect ui.Rect, strokes []ink.Stroke, b [4]float32, dark bool) {
	if len(strokes) == 0 {
		return
	}
	scale := min(rect.W/max(1, b[2]-b[0]), rect.H/max(1, b[3]-b[1]))
	cam := ink.Camera{Zoom: scale, PixelRatio: max(1, a.ratio), Offset: [2]float32{rect.X - b[0]*scale, rect.Y - b[1]*scale}}
	for _, s := range strokes {
		s.Color = AdaptiveColor(s.Color, dark)
		a.renderer.DrawRegion(p, snapshotStroke(s, dark), cam, rect)
	}
}
func (a *Editor) Bounds() [4]float32 {
	if a.boundsValid && a.boundsRevision == a.editRevision {
		return a.boundsCache
	}
	var b [4]float32
	for i, s := range a.strokes {
		v := ink.StrokeBounds(s)
		if i == 0 {
			b = v
		} else {
			b = [4]float32{min(b[0], v[0]), min(b[1], v[1]), max(b[2], v[2]), max(b[3], v[3])}
		}
	}
	a.boundsCache, a.boundsRevision, a.boundsValid = b, a.editRevision, true
	return b
}

// Revision changes after completed edits, so overviews can skip rebuilding
// while a stroke is still being sampled. It is not a content hash.
func (a *Editor) Revision() uint64 { return a.editRevision }

// DrawOverview paints all completed strokes into an overview without cloning
// the stroke slice; callers cache the result and redraw only when Revision
// changes.
func (a *Editor) DrawOverview(p *ui.Painter, rect ui.Rect, b [4]float32, dark bool) {
	if len(a.strokes) == 0 {
		return
	}
	scale := min(rect.W/max(1, b[2]-b[0]), rect.H/max(1, b[3]-b[1]))
	cam := ink.Camera{Zoom: scale, PixelRatio: 1, Offset: [2]float32{rect.X - b[0]*scale, rect.Y - b[1]*scale}}
	for _, s := range a.strokes {
		s.Color = AdaptiveColor(s.Color, dark)
		a.renderer.DrawRegion(p, snapshotStroke(s, dark), cam, rect)
	}
}

func (a *Editor) OverviewBounds() [4]float32 {
	v := a.ViewBounds()
	if len(a.strokes) == 0 {
		return v
	}
	b := a.Bounds()
	return [4]float32{min(b[0], v[0]), min(b[1], v[1]), max(b[2], v[2]), max(b[3], v[3])}
}

// ResetSurfaceCaches forces the next Draw to rebuild the complete dry layer
// from vector strokes. The GPU texture cache can be dropped after the window
// idles; an incremental layer cannot restore its base from the newest
// commands alone, so this is the recovery path.
func (a *Editor) ResetSurfaceCaches() {
	a.selection.releasePreview()
	a.releaseIdleSurface()
	a.live.Release()
	a.erasePreview.Reset()
	// One reset boundary for every disposable cache. Preserve transparency:
	// it is an embedding configuration, not GPU state.
	a.surfaceState = surfaceState{transparentInk: a.transparentInk, dryInvalid: true, recomposeGeneration: a.recomposeGeneration + 1}
}

func (a *Editor) ViewBounds() [4]float32 {
	c := a.camera()
	return [4]float32{-c.Offset[0] / c.Zoom, -c.Offset[1] / c.Zoom, (a.viewportW - c.Offset[0]) / c.Zoom, (a.viewportH - c.Offset[1]) / c.Zoom}
}

func (a *Editor) SelectionRect() ui.Rect {
	b := a.selection.bounds
	c := a.camera()
	return ui.Rect{X: b[0]*c.Zoom + c.Offset[0], Y: b[1]*c.Zoom + c.Offset[1], W: (b[2] - b[0]) * c.Zoom, H: (b[3] - b[1]) * c.Zoom}
}

// Dark paper needs translucent source-over highlighting; multiply cannot add
// visible color to a dark surface. Geometry, max coverage and stored Tool stay
// unchanged. Light paper keeps the engine's original multiply composition.
func snapshotStroke(s ink.Stroke, dark bool) ink.Stroke {
	if dark && s.Tool == ink.Highlighter {
		s.Tool = ink.Pen
	}
	return s
}
func (a *Editor) renderStroke(s ink.Stroke) ink.Stroke {
	if a.transparentInk && s.Tool == ink.Highlighter {
		s.Tool = ink.Pen
	}
	return snapshotStroke(s, a.dark)
}
func (a *Editor) SetBallpoint(ball bool) {
	if !a.Busy() {
		a.SetTool("pen")
		a.penSelect = 0
		if ball {
			a.penSelect = 1
		}
	}
}
func (a *Editor) Ballpoint() bool { return a.penSelect == 1 }

// Pan input may originate over transformed world-space UI. The embedding UI owns
// its layout and translates the event origin; only the ink camera is updated.
func (a *Editor) WantsPan(e ui.InputEvent) bool {
	return !a.Busy() && (e.Button == 2 || e.Button == 0 && (a.handMode || a.spaceDown))
}
func (a *Editor) Panning() bool       { return a.panning }
func (a *Editor) PanToolActive() bool { return a.handMode || a.spaceDown }

// SetHome defines the starting view without changing the current camera.
func (a *Editor) SetHome(x, y float64) { a.homeX = x; a.homeY = y }

// Camera is shared by world-space native UI and ink rendering.
func (a *Editor) Camera() ink.Camera { return a.camera() }

// ReleaseCaches also drops bounded prepared geometry when leaving the editor.
func (a *Editor) ReleaseCaches() { a.ResetSurfaceCaches(); a.renderer.ReleaseGeometryCache() }
