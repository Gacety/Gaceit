package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"study408/internal/ink"
)

func (a *app) viewZoom() float64 {
	if a.zoom <= 0 {
		return 1
	}
	return a.zoom
}
func (a *app) camera() ink.Camera {
	return ink.Camera{Zoom: float32(a.viewZoom()), PixelRatio: max(1, a.ratio), Offset: [2]float32{float32(a.offsetX), float32(a.offsetY)}}
}
func (a *app) toWorld(x, y float32) ui.Point {
	return ui.Point{X: float32((float64(x) - a.offsetX) / a.viewZoom()), Y: float32((float64(y) - a.offsetY) / a.viewZoom())}
}
func (a *app) zoomAt(factor float64, x, y float32) {
	if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) || a.current != nil || a.erasing || a.panning || a.selectionActive() {
		return
	}
	old := a.viewZoom()
	next := max(0.1, min(8, old*factor))
	wx, wy := (float64(x)-a.offsetX)/old, (float64(y)-a.offsetY)/old
	a.offsetX = float64(x) - wx*next
	a.offsetY = float64(y) - wy*next
	a.zoom = next
}
func (a *app) observeKeys(e ui.InputEvent) {
	if e.Key == ui.KeySpace {
		if e.Kind == ui.InputKeyDown {
			a.spaceDown = true
		}
		if e.Kind == ui.InputKeyUp {
			a.spaceDown = false
		}
	}
}
func (a *app) navigate(e ui.InputEvent) bool {
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
			a.zoomAt(math.Exp(-float64(dy)*0.006), e.X, e.Y)
		}
		return true
	case ui.InputZoom:
		a.zoomAt(float64(e.Scale), e.X, e.Y)
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
func (a *app) navigationControls(c *ui.Context) {
	ui.Row(c).Absolute().Left(16).Bottom(16).Height(36).Gap(4).Padding(2, 6).AlignItems(ui.Center).ZIndex(5).
		Background(ui.RGB(250, 250, 249)).Border(1, ui.RGB(215, 220, 221)).Radius(12).Children(func() {
		if paletteIcon(c, "缩小画布", 10, 28, false, false).Clicked() {
			a.zoomAt(1/1.25, a.viewportW/2, a.viewportH/2)
		}
		if ui.Button(c, percent(a.viewZoom())).Label("恢复 100% 缩放").Tooltip("恢复 100% 缩放").Width(62).Height(28).Padding(4, 6).FontSize(12).Clicked() {
			a.zoomAt(1/a.viewZoom(), a.viewportW/2, a.viewportH/2)
		}
		if paletteIcon(c, "放大画布", 11, 28, false, false).Clicked() {
			a.zoomAt(1.25, a.viewportW/2, a.viewportH/2)
		}
		if paletteIcon(c, "回到原点", 12, 28, false, a.current != nil || a.erasing || a.selectionActive()).Clicked() {
			a.zoom = 1
			a.offsetX = 0
			a.offsetY = 0
		}
	})
	ui.Text(c, "滚轮缩放 · 空格或中键拖动").Absolute().Left(211).Bottom(25).FontSize(11).TextColor(ui.RGB(99, 111, 116)).IgnoreInput(true)
}

func percent(zoom float64) string { return fmt.Sprintf("%.0f%%", zoom*100) }
