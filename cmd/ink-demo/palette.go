package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

const paletteWidth float32 = 622
const paletteHeight float32 = 52

var inkColors = [][4]float32{{0.08, 0.17, 0.28, 1}, {0.94, 0.23, 0.25, 1}, {0.27, 0.61, 0.96, 1}}

func (a *app) palette(c *ui.Context) {
	if !a.paletteReady {
		a.paletteReady = true
		a.panelDock = 1
		a.panelY = 24
		a.penWidth = float64(a.width)
		if a.penWidth <= 0 {
			a.penWidth = 8
		}
		if a.eraserWidth <= 0 {
			a.eraserWidth = 24
		}
		a.brushSelect = int(a.tool)
	}
	a.panelX, a.panelY = a.panelPosition()
	if a.panelCollapsed {
		ui.Row(c).Absolute().Left(a.panelX).Top(a.panelY).Width(68).Height(paletteHeight).Padding(8).Gap(4).AlignItems(ui.Center).ZIndex(10).
			Background(ui.RGB(250, 250, 249)).Border(1, ui.RGB(211, 215, 216)).Radius(18).Children(func() {
			ui.Box(c).Width(12).Height(34).Cursor(ui.CursorGrab).HandleInput(a.dragPalette).Draw(drawGrip)
			if paletteIcon(c, "展开工具栏", 13, 34, false, false).Clicked() {
				a.panelCollapsed = false
			}
		})
		return
	}
	ui.Row(c).Absolute().Left(a.panelX).Top(a.panelY).Width(paletteWidth).Height(paletteHeight).ZIndex(10).
		Padding(8).Gap(6).AlignItems(ui.Center).Background(ui.RGB(250, 250, 249)).Border(1, ui.RGB(211, 215, 216)).Radius(18).
		Children(func() {
			ui.Box(c).Width(16).Height(34).Cursor(ui.CursorGrab).HandleInput(a.dragPalette).Tooltip("拖动工具条，靠近左右边缘吸附").Draw(drawGrip)
			names := []string{"普通笔选项", "荧光笔", "铅笔", "橡皮擦"}
			for i, label := range names {
				selected := !a.handMode && a.paletteMode == 0 && a.brushSelect == i && i < 3 || !a.handMode && a.paletteMode == 1 && i == 3
				width := float32(34)
				if i == 0 || i == 3 {
					width = 44
				}
				if paletteIcon(c, label, i, width, selected, a.selectionActive()).Clicked() {
					if i < 3 {
						wasExpanded := a.panelExpanded
						a.switchTool(toolState{mode: 0, brush: i})
						if i == 0 {
							a.panelExpanded = !wasExpanded
						} else {
							a.panelExpanded = false
						}
					} else {
						wasEraser, wasExpanded := a.paletteMode == 1, a.panelExpanded
						a.switchTool(toolState{mode: 1, brush: a.brushSelect})
						a.panelExpanded = !wasEraser || !wasExpanded
					}
				}
			}
			if paletteIcon(c, "套索选择", 14, 34, a.paletteMode == 2 && !a.handMode, a.current != nil || a.erasing || a.selectionActive()).Clicked() {
				a.switchTool(toolState{mode: 2, brush: a.brushSelect})
			}
			if paletteIcon(c, "移动画布", 7, 34, a.handMode, a.selectionActive()).Clicked() {
				next := a.toolState()
				next.hand = !next.hand
				a.switchTool(next)
			}
			paletteDivider(c)
			presets := []float64{2, 4, 8}
			if a.paletteMode == 1 {
				presets = []float64{8, 16, 32}
			}
			for i, v := range presets {
				selected := a.paletteMode == 0 && a.penWidth == v || a.paletteMode == 1 && a.eraserWidth == v
				b := ui.ButtonBase(c.Key(fmt.Sprintf("width%d", i))).Label(fmt.Sprintf("线宽 %.0f", v)).Tooltip(fmt.Sprintf("线宽 %.0f", v)).Selected(selected).Size(28, 34).Radius(8)
				if selected {
					b.Background(ui.RGB(227, 229, 229))
				} else if b.Hovered() {
					b.Background(ui.RGB(237, 239, 239))
				}
				b.Draw(func(p *ui.Painter, r ui.Rect) {
					p.Line(r.X+7, r.Y+r.H/2, r.X+r.W-7, r.Y+r.H/2, float32(1+i), ui.RGB(68, 77, 83))
				})
				if b.Clicked() {
					if a.paletteMode == 1 {
						a.eraserWidth = v
					} else {
						a.penWidth = v
						a.width = float32(v)
					}
				}
			}
			paletteDivider(c)
			for i, label := range []string{"墨黑", "朱红", "天蓝"} {
				b := ui.ButtonBase(c.Key(label)).Label(label).Tooltip(label).Selected(a.colorSelect == i).Size(26, 34).Radius(8).Background(ui.RGB(250, 250, 249))
				b.Draw(func(p *ui.Painter, r ui.Rect) {
					col := inkColors[i]
					color := ui.RGB(uint8(col[0]*255), uint8(col[1]*255), uint8(col[2]*255))
					p.Fill(ui.Rect{X: r.X + 5, Y: r.Y + 9, W: 16, H: 16}, color, 8)
					if a.colorSelect == i {
						p.Stroke(ui.Rect{X: r.X + 2, Y: r.Y + 6, W: 22, H: 22}, ui.RGB(151, 164, 170), 11, 1)
					}
				})
				if b.Clicked() {
					a.colorSelect = i
				}
			}
			if paletteIcon(c, "粗细设置", 5, 32, a.panelExpanded, a.paletteMode == 2 || a.selectionActive()).Clicked() {
				a.panelExpanded = !a.panelExpanded
			}
			if paletteIcon(c, "撤销上一步", 6, 32, false, len(a.undo) == 0 || a.current != nil || a.erasing || a.selectionActive()).Clicked() {
				a.undoAction()
			}
			if paletteIcon(c, "收纳工具栏", 8, 28, false, false).Clicked() {
				a.panelCollapsed = true
				a.panelExpanded = false
			}
		})
	if !a.panelExpanded || a.paletteMode == 2 {
		return
	}
	top := a.panelY + paletteHeight + 8
	if top+180 > a.viewportH-8 {
		top = max(8, a.panelY-188)
	}
	ui.Column(c).Absolute().Left(a.panelX).Top(top).Width(244).Height(180).ZIndex(11).Padding(14).Gap(10).
		Background(ui.RGB(255, 255, 253)).Border(1, ui.RGB(211, 215, 216)).Radius(12).Children(func() {
		title := "普通笔"
		if a.paletteMode == 1 {
			title = "橡皮擦"
		} else if a.brushSelect == 1 {
			title = "荧光笔"
		} else if a.brushSelect == 2 {
			title = "铅笔"
		}
		ui.Row(c).FillWidth().Children(func() {
			ui.Text(c, title).FillWidth().FontSize(14)
			if ui.Button(c, "收起").Height(24).Clicked() {
				a.panelExpanded = false
			}
		})
		if a.paletteMode == 0 && a.brushSelect == 0 {
			ui.Segmented(c, &a.penSelect, "钢笔", "圆珠笔").FillWidth().Height(30)
		} else if a.paletteMode == 1 {
			ui.Segmented(c, &a.eraserSelect, "整笔擦除", "区域擦除").FillWidth().Height(30)
		} else {
			label := "真实图集纹理"
			if a.brushSelect == 1 {
				label = "同一笔自交处不加深"
			}
			ui.Text(c, label).FontSize(11).TextColor(ui.RGB(85, 100, 106)).Height(30)
		}
		value := &a.penWidth
		lo, hi := 0.5, 32.0
		if a.paletteMode == 1 {
			value = &a.eraserWidth
			lo = 4
			hi = 80
		}
		ui.Row(c).FillWidth().Children(func() {
			ui.Text(c, "粗细").FillWidth().FontSize(12)
			ui.Text(c, fmt.Sprintf("%.1f", *value)).FontSize(12)
		})
		if ui.Slider(c, value, lo, hi).Label("笔触粗细").Step(0.5).FillWidth().Height(24).Changed() && a.paletteMode == 0 {
			a.width = float32(a.penWidth)
		}
	})
	a.width = float32(a.penWidth)
}
func paletteDivider(c *ui.Context) { ui.Box(c).Width(1).Height(22).Background(ui.RGB(220, 224, 225)) }
func paletteIcon(c *ui.Context, label string, kind int, width float32, selected, disabled bool) ui.Element {
	tip := label
	if kind == 0 {
		tip += " · P 切换笔，再按返回"
	} else if kind == 3 {
		tip += " · E 切换橡皮，再按返回"
	}
	b := ui.ButtonBase(c.Key(label)).Label(label).Tooltip(tip).Selected(selected).Disabled(disabled).Size(width, 34).Radius(8)
	if selected {
		b.Background(ui.RGB(186, 224, 229))
	} else if b.Hovered() {
		b.Background(ui.RGB(232, 237, 238))
	}
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		col := ui.RGB(37, 49, 57)
		if disabled {
			col = ui.RGB(174, 181, 185)
		}
		drawToolIcon(p, r, kind, col)
	})
	return b
}
func (a *app) panelPosition() (float32, float32) {
	maxX := max(8, a.viewportW-a.paletteSpan()-8)
	x := a.panelX
	if a.panelDock < 0 {
		x = 8
	}
	if a.panelDock > 0 {
		x = maxX
	}
	return max(8, min(maxX, x)), max(8, min(max(8, a.viewportH-paletteHeight-8), a.panelY))
}
func (a *app) dragPalette(e ui.InputEvent) bool {
	switch e.Kind {
	case ui.InputPointerDown:
		a.panelDragging = true
		a.panelDragStart = ui.Point{X: a.panelX, Y: a.panelY}
		a.panelDragOffset = ui.Point{X: e.X, Y: e.Y}
		a.panelOldDock = a.panelDock
		a.panelDock = 0
		return true
	case ui.InputPointerMove:
		if a.panelDragging {
			a.panelX += e.X - a.panelDragOffset.X
			a.panelY += e.Y - a.panelDragOffset.Y
			a.panelX, a.panelY = a.panelPosition()
			return true
		}
	case ui.InputPointerUp:
		if !a.panelDragging {
			return false
		}
		a.panelDragging = false
		if e.Canceled {
			a.panelX = a.panelDragStart.X
			a.panelY = a.panelDragStart.Y
			a.panelDock = a.panelOldDock
			return true
		}
		if a.panelX <= 32 {
			a.panelDock = -1
		} else if a.viewportW-(a.panelX+a.paletteSpan()) <= 32 {
			a.panelDock = 1
		}
		a.panelX, a.panelY = a.panelPosition()
		return true
	}
	return false
}

func (a *app) paletteSpan() float32 {
	if a.panelCollapsed {
		return 68
	}
	return paletteWidth
}
