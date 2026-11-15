package nativeapp

import (
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"math"
	"os"
	"path/filepath"
	"slices"
	"study408/internal/ink"
	"study408/internal/inkcanvas"
	"time"
)

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validInkStrokes(strokes []ink.Stroke) bool {
	for _, s := range strokes {
		if s.Tool > ink.Pencil {
			return false
		}
		for _, v := range s.Color {
			if !finite(float64(v)) || v < 0 || v > 1 {
				return false
			}
		}
		for _, v := range s.Translation {
			if !finite(float64(v)) {
				return false
			}
		}
		for _, pt := range s.Points {
			if !finite(float64(pt.X)) || !finite(float64(pt.Y)) || !finite(float64(pt.Radius)) || pt.Radius < 0 {
				return false
			}
		}
		for _, path := range s.Erasures {
			for _, pt := range path {
				if !finite(float64(pt.X)) || !finite(float64(pt.Y)) || !finite(float64(pt.Radius)) || pt.Radius < 0 {
					return false
				}
			}
		}
		for _, stamp := range s.Stamps {
			for _, row := range stamp {
				for _, v := range row {
					if !finite(float64(v)) {
						return false
					}
				}
			}
		}
	}
	return true
}
func validateInkRecord(r *Record) error {
	if d := r.Ink; d != nil {
		if d.Version != 1 || !finite(d.Zoom) || d.Zoom < .1 || d.Zoom > 8 || !finite(d.X) || !finite(d.Y) || !validInkStrokes(d.Strokes) {
			return fmt.Errorf("不支持或无效的画布数据")
		}
	}
	for _, clip := range r.InkClips {
		for _, v := range clip.Bounds {
			if !finite(float64(v)) {
				return fmt.Errorf("无效的笔记选区")
			}
		}
		if !validInkStrokes(clip.Strokes) {
			return fmt.Errorf("无效的笔记笔迹")
		}
	}
	return nil
}

// Legacy normalized points remain untouched. This is an in-memory projection
// onto a 380x600 reference sheet until the user explicitly edits the new canvas.
func legacyInk(r *Record) inkcanvas.Document {
	d := inkcanvas.Document{Version: 1, Zoom: 1}
	for _, s := range r.Strokes {
		var pts []ink.Point
		radius := float32(1.3)
		if s.Tool == "eraser" {
			radius = 12
		}
		for _, p := range s.Points {
			pts = append(pts, ink.Point{X: 24 + p[0]*380, Y: 110 + p[1]*600, Radius: radius})
		}
		if s.Tool == "eraser" {
			for i := range d.Strokes {
				d.Strokes[i].Erasures = append(d.Strokes[i].Erasures, pts)
			}
		} else {
			d.Strokes = append(d.Strokes, ink.Stroke{Points: pts, Color: [4]float32{0, 0, 0, 1}, Smooth: true})
		}
	}
	return d
}

const (
	canvasInitialZoom = .8
	canvasMinimumZoom = .5
	canvasMaximumZoom = 2
)

func (a *App) ensureInk() bool {
	if a.Window != nil && a.inkSamplingRestore == nil {
		a.inkSamplingRestore = startInkSampling()
	}
	if a.inkEditor == nil {
		ed, err := inkcanvas.New()
		if err != nil {
			a.message = "原生笔迹初始化失败：" + err.Error()
			return false
		}
		ed.SetTransparentInk(true)
		ed.ConfigureZoom(canvasInitialZoom, canvasMinimumZoom, canvasMaximumZoom, .6)
		a.inkEditor = ed
	}
	if a.inkQuestion != a.M.Round.CurrentID {
		a.flushInk()
		a.flushSave()
		a.inkEditor.Cancel()
		d, ok := a.inkSessions[a.M.Round.CurrentID]
		if !ok {
			d = inkcanvas.Document{Version: 1, Zoom: canvasInitialZoom}
		}
		a.inkHomePending = !ok
		a.inkEditor.Load(d)
		a.inkQuestion = a.M.Round.CurrentID
		a.inkDirty = false
	}
	a.inkEditor.Appearance(a.M.Theme == "dark", max(1, a.DisplayRatio))
	if a.canvasEntered.IsZero() {
		a.canvasEntered = time.Now()
	}
	return true
}

// Scratch is deliberately not part of the persistent learning model. Closing
// a panel or changing questions keeps it in memory; a new App starts empty.
func (a *App) flushInk() {
	if a.inkEditor == nil || a.inkQuestion == "" || !a.inkDirty {
		return
	}
	if a.inkSessions == nil {
		a.inkSessions = map[string]inkcanvas.Document{}
	}
	a.inkSessions[a.inkQuestion] = a.inkEditor.Snapshot()
	a.inkDirty = false
}
func (a *App) closeCanvas() {
	a.canvasTextInteraction = false
	if a.inkSamplingRestore != nil {
		a.inkSamplingRestore()
		a.inkSamplingRestore = nil
	}
	if a.inkEditor != nil {
		a.inkEditor.Cancel()
	}
	a.flushInk()
	a.flushSave()
	if a.inkEditor != nil {
		a.inkEditor.ReleaseCaches()
	}
	if a.inkPreview != nil {
		a.inkPreview.ReleaseCaches()
		a.inkPreview = nil
	}
	if !a.canvasEntered.IsZero() {
		a.canvasElapsed += time.Since(a.canvasEntered)
		a.canvasEntered = time.Time{}
	}
	a.canvasNotes = false
	a.canvasProgress = false
	a.canvasSettings = false
	a.canvasPaintedAt = time.Time{}
}
func (a *App) inkChanged() { a.inkDirty = true; a.inkChangedAt = time.Now() }
func (a *App) canvasInput(e ui.InputEvent) bool {
	if a.inkEditor == nil {
		return false
	}
	if e.Kind == ui.InputPointerDown && e.Pointer == ui.PointerPen {
		a.canvasTextInteraction = false
	}
	revision := a.inkEditor.Revision()
	taken := a.inkEditor.Input(e)
	if taken {
		if a.inkEditor.Revision() != revision || e.Kind == ui.InputPointerUp || e.Kind == ui.InputScroll || e.Kind == ui.InputZoom {
			a.inkChanged()
		}
	}
	return taken
}
func (a *App) saveInkSelection() {
	if a.inkEditor == nil {
		return
	}
	strokes, b := a.inkEditor.Selected()
	if len(strokes) == 0 {
		return
	}
	r := a.M.Records[a.M.Round.CurrentID]
	r.InkClips = append(r.InkClips, InkClip{ID: fmt.Sprintf("clip-%d", time.Now().UnixNano()), CreatedAt: stamp(time.Now()), Strokes: slices.Clone(strokes), Bounds: b})
	a.canvasNotes = true
	a.flushInk()
	a.save()
}
func (a *App) UpdateDisplayRatio() {
	if a.Window == nil {
		return
	}
	b := a.Window.Bounds()
	a.DisplayRatio = float32(mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}).ScaleFactor)
	a.Window.Invalidate()
}

func newInkPreview() (*inkcanvas.Editor, error) { return inkcanvas.New() }

func (a *App) FlushWriting() error {
	a.closeCanvas()
	a.flushSave()
	if a.inkDirty {
		return fmt.Errorf("%s", a.message)
	}
	return nil
}

// Make a single private, non-overwriting copy before adding the first advanced
// ink document to an existing learning file. Fresh installations need no copy.
func (a *App) backupBeforeInk() error {
	if a.M.Disk == nil {
		return nil
	}
	path := filepath.Join(filepath.Dir(a.M.Disk.Path), "progress-before-native-ink-v1.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(a.M.Disk.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

// Question input arrives in its transformed local coordinates. Convert it
// back to screen DIPs using the rendered frame, then route canvas navigation.
// The complete question is laid out in world space without internal scrolling.
func (a *App) canvasQuestionInput(e ui.InputEvent, screenOrigin ui.Point) bool {
	switch e.Kind {
	case ui.InputScroll, ui.InputZoom:
	case ui.InputPointerDown:
		if e.Pointer != ui.PointerPen && e.Button == 0 {
			return false
		}
		if !a.inkEditor.WantsPan(e) {
			return false
		}
	case ui.InputPointerMove, ui.InputPointerUp:
		if !a.inkEditor.Panning() {
			return false
		}
	default:
		return false
	}
	// The card is laid out at screen scale, so local input is already DIPs.
	e.X += screenOrigin.X
	e.Y += screenOrigin.Y
	return a.canvasInput(e)
}

// A genuine reactivation may restore another native surface. Request a
// validity-checked recovery on the next paint without changing input focus.
func (a *App) RestoreWritingSurface() {
	if a.tool != "scratch" || a.inkEditor == nil {
		return
	}
	a.inkRecoveryPending = true
	a.canvasPaintedAt = time.Time{}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// Trackpad/mouse use native card interaction; on blank paper a primary drag
// navigates instead of laying down accidental ink. Explicit lasso/eraser tools
// remain usable with a mouse outside the card.
func (a *App) canvasPaperInput(e ui.InputEvent) bool {
	if e.Kind == ui.InputPointerDown {
		a.canvasTextInteraction = false
	}
	if e.Pointer == ui.PointerPen {
		return a.canvasInput(e)
	}
	if a.inkEditor.Tool() == "pen" || a.inkEditor.Tool() == "pencil" || a.inkEditor.Tool() == "highlighter" || a.inkEditor.Panning() {
		if e.Kind == ui.InputPointerDown || e.Kind == ui.InputPointerUp {
			if e.Button == 0 {
				e.Button = 2
			}
		}
	}
	return a.canvasInput(e)
}
