package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
	"time"
)

func (a *Editor) viewZoom() float64 {
	if a.zoom <= 0 {
		return 1
	}
	return a.zoom
}
func (a *Editor) camera() ink.Camera {
	return ink.Camera{Zoom: float32(a.viewZoom()), PixelRatio: max(1, a.ratio), Offset: [2]float32{float32(a.offsetX), float32(a.offsetY)}}
}
func (a *Editor) toWorld(x, y float32) ui.Point {
	return ui.Point{X: float32((float64(x) - a.offsetX) / a.viewZoom()), Y: float32((float64(y) - a.offsetY) / a.viewZoom())}
}
func (a *Editor) zoomAt(factor float64, x, y float32) {
	if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) || a.current != nil || a.erasing || a.panning || a.selectionActive() {
		return
	}
	old := a.viewZoom()
	lo, hi := a.zoomLimits()
	next := max(lo, min(hi, old*factor))
	if next == old {
		return
	}
	a.zoomChangedAt = time.Now()
	wx, wy := (float64(x)-a.offsetX)/old, (float64(y)-a.offsetY)/old
	a.offsetX = float64(x) - wx*next
	a.offsetY = float64(y) - wy*next
	a.zoom = next
}
func (a *Editor) observeKeys(e ui.InputEvent) {
	if e.Key == ui.KeySpace {
		if e.Kind == ui.InputKeyDown {
			a.spaceDown = true
		}
		if e.Kind == ui.InputKeyUp {
			a.spaceDown = false
		}
	}
}
func (a *Editor) navigate(e ui.InputEvent) bool {
	switch e.Kind {
	case ui.InputKeyDown, ui.InputKeyUp:
		a.observeKeys(e)
		return e.Key == ui.KeySpace
	case ui.InputScroll:
		if a.current != nil || a.erasing || a.panning || a.selectionActive() {
			return true
		}
		if e.Precise && e.Mods&(ui.Ctrl|ui.Super) == 0 {
			a.offsetX -= float64(e.DX)
			a.offsetY -= float64(e.DY)
		} else {
			dy := max(float32(-240), min(240, e.DY))
			a.zoomAt(a.inputZoomFactor(math.Exp(-float64(dy)*0.003)), e.X, e.Y)
		}
		return true
	case ui.InputZoom:
		a.zoomAt(a.inputZoomFactor(float64(e.Scale)), e.X, e.Y)
		return true
	case ui.InputPointerDown:
		if a.current != nil || a.erasing || a.selectionActive() {
			return e.Button != 0
		}
		if (e.Button == 2 || e.Button == 0 && (a.spaceDown || a.handMode)) && a.current == nil && !a.erasing {
			a.panning = true
			a.panAnchor = ui.Point{X: e.X, Y: e.Y}
			a.panBefore = [2]float64{a.offsetX, a.offsetY}
			return true
		}
	case ui.InputPointerMove:
		if a.panning {
			a.offsetX = a.panBefore[0] + float64(e.X-a.panAnchor.X)
			a.offsetY = a.panBefore[1] + float64(e.Y-a.panAnchor.Y)
			return true
		}
	case ui.InputPointerUp:
		if !a.panning && e.Button != 0 {
			return true
		}
		if a.panning {
			if e.Canceled {
				a.offsetX = a.panBefore[0]
				a.offsetY = a.panBefore[1]
			} else {
				a.offsetX = a.panBefore[0] + float64(e.X-a.panAnchor.X)
				a.offsetY = a.panBefore[1] + float64(e.Y-a.panAnchor.Y)
			}
			a.panning = false
			return true
		}
	}
	return false
}

func (a *Editor) zoomLimits() (float64, float64) {
	if a.minZoom > 0 && a.maxZoom >= a.minZoom {
		return a.minZoom, a.maxZoom
	}
	return .1, 8
}

// ConfigureZoom bounds native layout work without changing stored geometry.
func (a *Editor) ConfigureZoom(initial, minimum, maximum, power float64) {
	a.homeZoom, a.minZoom, a.maxZoom, a.zoomInputPower = initial, minimum, maximum, power
}
func (a *Editor) HomeZoom() float64 {
	if a.homeZoom > 0 {
		return a.homeZoom
	}
	return 1
}
func (a *Editor) inputZoomFactor(factor float64) float64 {
	if a.zoomInputPower <= 0 {
		return factor
	}
	if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return factor
	}
	return math.Exp(max(-math.Log(1.12), min(math.Log(1.12), math.Log(factor)*a.zoomInputPower)))
}
