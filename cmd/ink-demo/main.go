// Native MyGo ink sandbox, deliberately independent of the workbook WebView.
package main

import (
	"flag"
	"image/png"
	"log"
	"math"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

type app struct {
	renderer                                                       *ink.Renderer
	strokes                                                        []ink.Stroke
	current                                                        *ink.Stroke
	tool                                                           ink.Tool
	width, ratio                                                   float32
	pointer                                                        ui.PointerType
	window                                                         *mygo.Window
	live                                                           ink.LiveStroke
	erasePreview                                                   ink.ErasePreview
	dry                                                            [2]*ui.Canvas
	dryFront, bakedCount                                           int
	dryW, dryH                                                     int
	dryRatio                                                       float32
	showTools, paletteReady, panelExpanded                         bool
	paletteMode, brushSelect, penSelect, eraserSelect, colorSelect int
	penWidth, eraserWidth                                          float64
	panelX, panelY, viewportW, viewportH                           float32
	panelDock, panelOldDock                                        int
	panelDragging                                                  bool
	panelDragOffset, panelDragStart                                ui.Point
	strokeBall                                                     bool
	dryInvalid                                                     bool
	undo                                                           [][]ink.Stroke
	writeBefore                                                    []ink.Stroke
	erasing, eraseChanged                                          bool
	eraseKind                                                      int
	erasePath                                                      []ink.Point
	eraseBefore                                                    []ink.Stroke
	hoverPoint                                                     ui.Point
	hasHover                                                       bool
	zoom, offsetX, offsetY                                         float64
	dryCamera                                                      ink.Camera
	spaceDown, handMode, panning, panelCollapsed                   bool
	panAnchor                                                      ui.Point
	panBefore                                                      [2]float64
	strokeIndex                                                    *ink.HitIndex
	strokeRemoved                                                  map[int]bool
	eraseCursor                                                    int
	strokeHitTests                                                 int
	dirtyRegions                                                   [][4]float32
	panCache                                                       panSurface
	selection                                                      lassoSelection
	previousTool                                                   toolState
	hasPreviousTool                                                bool
}

func (a *app) input(e ui.InputEvent) bool {
	if e.Kind == ui.InputKeyDown && e.Key == ui.KeyEscape && (a.selectionActive() || len(a.selection.ids) > 0) {
		if a.selection.moving {
			a.strokes = a.selection.before
			a.dryInvalid = true
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
		a.panelExpanded = false
		a.clearSelection()
		a.pointer = e.Pointer
		tool := a.tool
		a.strokeBall = (tool == ink.Pen && a.penSelect == 1) || a.pointer != ui.PointerPen
		a.writeBefore = append([]ink.Stroke(nil), a.strokes...)
		color := inkColors[a.colorSelect]
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
			a.current = nil
			return true
		}
		if a.current != nil {
			a.add(e)
			if a.current.Tool == ink.Pencil {
				a.current.Stamps = ink.MakeStrokeStamps(*a.current, 1)
			}
			a.remember(a.writeBefore)
			a.strokes = append(a.strokes, *a.current)
			a.current = nil
			return true
		}
	}
	return false
}

// add uses only actual position and (for a tablet pen) actual pressure.
// Event cadence and movement speed never affect width.
func (a *app) add(e ui.InputEvent) {
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
func (a *app) view(c *ui.Context) {
	a.toolShortcuts(c)
	a.viewportW, a.viewportH = c.Size()
	// Window key state survives closing a focused popover control.
	a.spaceDown = c.KeyDown(ui.KeySpace)
	ui.Box(c).Fill().Background(ui.RGB(250, 249, 245)).Children(func() {
		cursor := ui.CursorDefault
		if a.panning {
			cursor = ui.CursorGrabbing
		} else if a.handMode || a.spaceDown {
			cursor = ui.CursorGrab
		} else if a.paletteMode == 2 {
			cursor = ui.CursorCrosshair
			w := a.toWorld(a.hoverPoint.X, a.hoverPoint.Y)
			if a.selectionContains(w.X, w.Y) || a.selection.moving {
				cursor = ui.CursorMove
			}
		}
		ui.Box(c).Absolute().Fill().Cursor(cursor).Focusable().AutoFocus().HandleInput(a.input).Draw(func(p *ui.Painter, r ui.Rect) {
			camera := a.camera()
			a.processSelectionMove()
			a.processStrokeErase()
			a.drawDry(p, r, camera)
			if a.erasing && a.eraseKind == 1 && len(a.erasePath) > 0 {
				a.erasePreview.Draw(p, a.renderer, a.dry[a.dryFront].Bitmap(), a.erasePath, camera, r.W, r.H, [4]float32{250.0 / 255, 249.0 / 255, 245.0 / 255, 1})
			}
			if a.current != nil {
				a.live.Draw(p, a.renderer, *a.current, camera, r.W, r.H)
			}
			a.drawSelection(p, camera)
			if a.paletteMode == 1 && a.hasHover && !a.handMode && !a.spaceDown && !a.panning {
				diameter := float32(max(1, a.eraserWidth) * a.viewZoom())
				box := ui.Rect{X: a.hoverPoint.X - diameter/2, Y: a.hoverPoint.Y - diameter/2, W: diameter, H: diameter}
				p.Stroke(box, ui.RGB(255, 255, 255), diameter/2, 2/camera.PixelRatio)
				p.Stroke(box, ui.RGB(75, 91, 101), diameter/2, 1/camera.PixelRatio)
			}
		})
		if a.showTools {
			a.palette(c)
			a.navigationControls(c)
		}
	})
}
func fixtures() []ink.Stroke {
	return []ink.Stroke{
		{Color: [4]float32{0.08, 0.17, 0.28, 1}, Points: []ink.Point{{X: 70, Y: 80, Radius: 2}, {X: 230, Y: 15, Radius: 16}, {X: 400, Y: 100, Radius: 3}}},
		{Color: [4]float32{0.15, 0.3, 0.75, 1}, Points: []ink.Point{{X: 70, Y: 160, Radius: 3}, {X: 240, Y: 160, Radius: 10}, {X: 410, Y: 160, Radius: 3}}},
		{Color: [4]float32{0.08, 0.17, 0.28, 1}, Points: []ink.Point{{X: 460, Y: 80, Radius: 13}}},
		{Tool: ink.Highlighter, Color: [4]float32{1, 0.8, 0.05, 0.5}, Points: []ink.Point{{X: 80, Y: 220, Radius: 15}, {X: 380, Y: 320, Radius: 15}, {X: 80, Y: 320, Radius: 15}, {X: 380, Y: 220, Radius: 15}, {X: 80, Y: 220, Radius: 15}}},
		{Tool: ink.Pencil, Color: [4]float32{0.15, 0.15, 0.15, 0.9}, Points: []ink.Point{{X: 80, Y: 390, Radius: 6}, {X: 240, Y: 340, Radius: 10}, {X: 420, Y: 410, Radius: 6}}},
	}
}
func main() {
	tool := flag.String("tool", "pen", "pen, highlighter, pencil")
	width := flag.Float64("width", 8, "model width")
	render := flag.String("render", "", "write a fixture PNG without opening a window")
	demo := flag.Bool("demo", false, "start with conformance fixtures")
	scale := flag.Float64("scale", 2, "physical pixel ratio for PNG output")
	flag.Parse()
	a := &app{width: float32(*width), ratio: float32(*scale), showTools: true}
	switch *tool {
	case "pen":
	case "highlighter":
		a.tool = ink.Highlighter
	case "pencil":
		a.tool = ink.Pencil
	default:
		log.Fatal("unknown tool")
	}
	var err error
	a.renderer, err = ink.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}
	if *demo || *render != "" {
		a.strokes = fixtures()
	}
	if *render != "" {
		img := ui.Render(a.view, 960, 720, a.ratio)
		f, err := os.Create(*render)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, img); err != nil {
			log.Fatal(err)
		}
		return
	}
	mygo.App.WhenReady(func() {
		a.window = mygo.NewWindow(mygo.WindowOptions{Title: "Ink · " + *tool, Width: 960, Height: 720, MinWidth: 640, MinHeight: 520, Content: ui.View(a.view)})
		a.updateDisplayRatio()
		a.window.OnMove(a.updateDisplayRatio)
		a.window.OnResize(a.updateDisplayRatio)
		a.window.OnBlur(func() { a.spaceDown = false })
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// drawDry submits completed strokes only once, retaining their composited pixels.
func (a *app) drawDry(p *ui.Painter, rect ui.Rect, camera ink.Camera) {
	ratio := camera.PixelRatio
	w, h := max(1, int(math.Ceil(float64(rect.W*ratio)))), max(1, int(math.Ceil(float64(rect.H*ratio))))
	// A new edit invalidates the overscan snapshot, but camera translation
	// alone does not. Keep the normal viewport cache aligned for ink/erasers.
	if a.dryInvalid || len(a.dirtyRegions) > 0 || a.bakedCount != len(a.strokes) {
		a.panCache.reset()
	}
	if a.usePanSurface(camera, w, h) {
		a.panCache.draw(p, a, rect, camera, w, h)
		return
	}
	if len(a.dirtyRegions) > 0 && !a.dryInvalid && a.dry[0] != nil && w == a.dryW && h == a.dryH && camera == a.dryCamera {
		a.repairDry(p, camera)
	}
	a.dirtyRegions = nil
	if a.dryInvalid || a.dry[0] == nil || w != a.dryW || h != a.dryH || a.dryRatio != ratio || a.dryCamera != camera || len(a.strokes) < a.bakedCount {
		a.panCache.reset()
		a.dryInvalid = false
		a.dry = [2]*ui.Canvas{ui.NewCanvas(w, h), ui.NewCanvas(w, h)}
		a.dryW = w
		a.dryH = h
		a.dryRatio = ratio
		a.dryCamera = camera
		a.bakedCount = 0
		a.dryFront = 0
		a.live.Reset()
		a.dry[0].Draw(p, func(cp *ui.Painter) { cp.Fill(ui.Rect{W: float32(w), H: float32(h)}, ui.RGB(250, 249, 245), 0) })
	}
	if a.bakedCount < len(a.strokes) {
		next := 1 - a.dryFront
		previous := a.dry[a.dryFront]
		start := a.bakedCount
		a.dry[next].Draw(p, func(cp *ui.Painter) {
			cp.Image(previous.Bitmap(), ui.Rect{W: float32(w), H: float32(h)}, ui.FillBox)
			visible := [4]float32{-camera.Offset[0] / camera.Zoom, -camera.Offset[1] / camera.Zoom, (rect.W - camera.Offset[0]) / camera.Zoom, (rect.H - camera.Offset[1]) / camera.Zoom}
			pixelCamera := ink.Camera{Zoom: camera.Zoom * ratio, PixelRatio: 1, Offset: [2]float32{camera.Offset[0] * ratio, camera.Offset[1] * ratio}}
			for _, s := range a.strokes[start:] {
				if ink.BoundsOverlap(ink.StrokeBounds(s), visible) {
					a.renderer.DrawViewport(cp, s, pixelCamera, float32(w), float32(h))
				}
			}
		})
		a.dryFront = next
		a.bakedCount = len(a.strokes)
	}
	p.Image(a.dry[a.dryFront].Bitmap(), rect, ui.FillBox)
}
func (a *app) updateDisplayRatio() {
	b := a.window.Bounds()
	a.ratio = float32(mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}).ScaleFactor)
	a.window.Invalidate()
}
