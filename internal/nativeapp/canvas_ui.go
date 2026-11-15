package nativeapp

import (
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
)

func (a *App) canvasSurface() ui.Color { return a.exactColor("#232632", "#f4f3ed") }
func (a *App) canvasBorder() ui.Color  { return a.exactColor("#363A48", "#d7d8cd") }
func (a *App) canvasFloat(c *ui.Context) ui.Element {
	return ui.Row(c).Absolute().Background(a.canvasSurface()).Border(1, a.canvasBorder()).Radius(16).Gap(8).Padding(8).AlignItems(ui.Center).ZIndex(5).Shadow(0, 8, 24, 0, a.canvasShadow(.35))
}
func (a *App) canvasButton(c *ui.Context, label string) ui.Element {
	return a.exactButton(c, label).Height(44).Padding(0, 16).Radius(12).FontSize(15).TextColor(c.Theme().Text)
}
func (a *App) canvasIcon(c *ui.Context, label string, selected bool) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Label(label).Tooltip(label).Size(44, 44).Radius(12).AlignItems(ui.Center).Justify(ui.Center).TextColor(c.Theme().TextMuted)
	if selected {
		b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48"))
	}
	b.Children(func() {
		if icon := canvasIcons[label]; icon != nil {
			ui.Icon(c, icon).Size(22, 22)
		}
	})
	return b
}
func (a *App) canvasPage(c *ui.Context) {
	started := time.Now()
	defer func() {
		if a.inkEditor != nil {
			a.inkEditor.ObservePhase("native_build", started)
		}
	}()
	c.SetTheme(a.reviewTheme())
	if !a.ensureInk() {
		ui.Text(c, a.message).Padding(40)
		if a.exactButton(c, "退出草稿").Clicked() {
			a.tool = ""
		}
		return
	}
	ed := a.inkEditor
	w, h := c.Size()
	ed.SetHome(float64(w)/2-720*ed.HomeZoom(), 0)
	a.syncCanvasCardSurface()
	if a.inkHomePending {
		ed.Home()
		a.inkHomePending = false
	}
	// Read-only question interaction owns text copy/selection, but must not
	// disable the tablet's P/E tool keys. Only note editing or error reporting suppresses those.
	if !a.canvasNotes && !a.reportOpen {
		ed.Shortcuts(c)
	}
	a.canvasSelectionShortcuts(c)
	if !a.canvasNotes && !a.reportOpen && !a.canvasTextInteraction && c.Shortcut(ui.Cmd, ui.KeyZ) {
		ed.Undo()
		a.inkChanged()
	}
	ui.Box(c).Fill().Clip().Label("无限画布工作区").Background(a.exactColor("#15161C", "#faf9f5")).Children(func() {
		ui.Box(c).Absolute().Fill().Label("画布纸面导航").Draw(func(p *ui.Painter, r ui.Rect) { ed.DrawPaper(p, r) }).ContextMenu(func(m *ui.Menu) { a.canvasSelectionMenu(m, c.Services()) }).HandleInput(a.canvasPaperInput)
		ed.Canvas(c.Key("ink-surface-" + a.inkQuestion)).ZIndex(3).PenInputOnly().ContextMenu(func(m *ui.Menu) { a.canvasSelectionMenu(m, c.Services()) }).HandleInput(func(e ui.InputEvent) bool {
			if e.Kind == ui.InputCommand && a.canvasEditCommand(e.Text, c.Services()) {
				return true
			}
			return a.canvasInput(e)
		}).Draw(func(p *ui.Painter, r ui.Rect) {
			// Measure real paints, not build passes: native focus/layout passes may
			// rebuild the view without presenting the ink texture.
			now := time.Now()
			if a.inkRecoveryPending || !a.canvasPaintedAt.IsZero() && now.Sub(a.canvasPaintedAt) > 1800*time.Millisecond {
				ed.RecoverAfterIdle(p, r)
				a.inkRecoveryPending = false
			}
			ed.Draw(p, r)
			if ed.PreviewingZoom() {
				p.After(200 * time.Millisecond)
			}
			a.canvasPaintedAt = time.Now()
		})
		// A world-space card shares the ink camera; only floating controls are
		// screen-fixed. Lay the card out at the scaled sizes rather than scaling
		// a 100% layer: MyGo's transform magnifies the glyph texture, which
		// blurs text as soon as the canvas zooms in.
		camera := ed.Camera()
		scale := camera.Zoom
		qx, qy := float32(360)*scale+camera.Offset[0], float32(110)*scale+camera.Offset[1]
		cardW := float32(720) * scale
		ui.Column(c).Absolute().Left(qx).Top(qy).Width(cardW).Shrink(0).Padding(28*scale, 32*scale).Gap(20*scale).Radius(20*scale).Background(a.reviewSurface(c)).Border(max(1, scale), c.Theme().Border).ZIndex(2).Shadow(0, 12*scale, 40*scale, 0, a.canvasShadow(.4)).Label("画布浮动题目").Draw(func(p *ui.Painter, r ui.Rect) {
			// Layout changes also change the card's shadow/occlusion region.
			// Recompose the complete ink base on the next frame; never leave
			// a partial damage repair as the only copy of surrounding ink.
			height := r.H / scale
			if a.canvasCardHeight > 0 && a.canvasCardZoom == scale && absCanvasHeight(height-a.canvasCardHeight) > 4 {
				// Generation tracking carries this request through an active
				// zoom preview or rebake; it must not be dropped on the floor.
				ed.RequestRecompose()
				p.After(time.Millisecond)
			}
			a.canvasCardHeight = height
			a.canvasCardZoom = scale
		}).ObserveInput(func(e ui.InputEvent) {
			if e.Kind == ui.InputPointerDown && e.Pointer != ui.PointerPen {
				a.canvasTextInteraction = true
			}
		}).HandleInput(func(e ui.InputEvent) bool { return a.canvasQuestionInput(e, ui.Point{X: qx, Y: qy}) }).Children(func() {
			if len(a.queue) == 0 {
				a.empty(c)
				return
			}
			a.inCanvasQuestion = true
			a.canvasScale = scale
			a.Rich.Scale = scale
			a.question(c)
			a.Rich.Scale = 0
			a.canvasScale = 0
			a.inCanvasQuestion = false
		})
		a.canvasFloat(c).Left(24).Top(24).Padding(6).Children(func() {
			if a.canvasButton(c, "退出草稿").Clicked() {
				a.closeCanvas()
				if !a.inkDirty {
					a.tool = ""
				}
			}
			label := fmt.Sprintf("%d / %d", slices.Index(a.queue, a.M.Round.CurrentID)+1, len(a.queue))
			if a.exactButton(c, label).Label("打开进度面板").Height(32).Padding(0, 10).Radius(8).FontSize(13).Background(c.Theme().Background).Tooltip("查看本次练习进度").Clicked() {
				a.canvasProgress = !a.canvasProgress
			}
		})
		a.canvasFloat(c).Right(24).Top(24).Padding(6).Children(func() {
			b := a.canvasButton(c, "本题笔记").Tooltip("打开或收起本题笔记卡")
			if a.canvasNotes {
				b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48"))
			}
			if b.Clicked() {
				a.canvasNotes = !a.canvasNotes
				a.canvasSettings = false
			}
		})
		a.canvasFloat(c).Left(24).Bottom(28).Gap(4).Children(func() {
			if a.canvasButton(c, "回到题目").Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48")).Clicked() {
				ed.Home()
				a.inkChanged()
			}
			ui.Box(c).Width(1).Height(24).Background(a.canvasBorder()).Margin(0, 6)
			if a.canvasButton(c, "−").Label("缩小画布").Disabled(ed.Zoom() <= canvasMinimumZoom).Width(44).Padding(0).FontSize(20).Clicked() {
				ed.ZoomBy(1 / 1.1)
				a.inkChanged()
			}
			if a.exactButton(c, fmt.Sprintf("%.0f%%", ed.Zoom()*100)).Label("恢复100%缩放").Padding(0).Width(44).FontWeight(400).Clicked() {
				ed.ZoomBy(1 / ed.Zoom())
				a.inkChanged()
			}
			if a.canvasButton(c, "+").Label("放大画布").Disabled(ed.Zoom() >= canvasMaximumZoom).Width(44).Padding(0).FontSize(20).Clicked() {
				ed.ZoomBy(1.1)
				a.inkChanged()
			}
		})
		a.canvasTools(c, w)
		a.canvasFloat(c).Right(24).Bottom(28).Children(func() {
			i := slices.Index(a.queue, a.M.Round.CurrentID)
			if a.canvasButton(c, "上一题").Border(1, a.canvasBorder()).AccessDisabled(i <= 0).Clicked() {
				a.navigate(-1)
			}
			if a.canvasButton(c, "下一题").Border(1, a.canvasBorder()).Background(c.Theme().Border).AccessDisabled(i < 0 || i >= len(a.queue)-1).Clicked() {
				a.navigate(1)
			}
			label := "提交答案"
			if a.M.Current().IsSubjective() && !a.answered && a.M.AIConfig.Configured() {
				label = "AI 批改"
				if a.aiGrading {
					label = "AI 批改中…"
				}
			} else if a.answered {
				label = "继续下一题"
			}
			if a.canvasButton(c, label).Padding(0, 24).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).FontWeight(700).Disabled(len(a.queue) == 0 || (!a.answered && a.aiGrading)).Clicked() {
				if a.answered {
					a.navigate(1)
				} else {
					a.submit(false)
				}
			}
		})
		if ed.Tool() == "lasso" {
			bottom := float32(100)
			if w < 1300 {
				bottom = 176
			}
			a.canvasFloat(c).Left(24).Bottom(bottom).ZIndex(6).Children(func() {
				if a.exactButton(c, "复制").Tooltip("复制笔迹 ⌘C").Disabled(!ed.HasSelection()).Clicked() {
					a.copyInk(c.Services())
				}
				if a.exactButton(c, "粘贴").Tooltip("粘贴笔迹 ⌘V").Disabled(ed.Busy()).Clicked() {
					a.pasteInk(c.Services())
				}
				if a.exactButton(c, "删除").Tooltip("删除笔迹 Delete / Backspace").Disabled(!ed.HasSelection()).Clicked() {
					a.deleteInk()
				}
				if a.exactButton(c, "存入本题笔记").Disabled(!ed.HasSelection()).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).Clicked() {
					a.saveInkSelection()
				}
			})
		}
		if a.canvasSettings {
			a.canvasToolSettings(c, w, h)
		}
		if a.canvasProgress {
			a.canvasProgressPanel(c, h)
		}
		if a.canvasNotes {
			a.canvasNoteCard(c, w, h)
		}
	})
	if ed.PreviewingZoom() {
		c.After(200 * time.Millisecond)
	}
}
func (a *App) canvasTools(c *ui.Context, w float32) {
	ed := a.inkEditor
	// Extra demo tools live under Settings to preserve the reference toolbar size.
	width := float32(474)
	bottom := float32(28)
	if w < 1300 {
		bottom = 98
	}
	a.canvasFloat(c).Left((w - width) / 2).Bottom(bottom).Width(width).Gap(4).Children(func() {
		for _, t := range []struct{ key, label string }{{"pen", "画笔"}, {"highlighter", "荧光笔"}, {"eraser", "橡皮擦"}, {"lasso", "套索"}} {
			if a.canvasIcon(c, t.label, ed.Tool() == t.key).Clicked() {
				ed.SetTool(t.key)
			}
		}
		ui.Box(c).Width(1).Height(24).Background(a.canvasBorder()).Margin(0, 6)
		for i := 0; i < 3; i++ {
			color := a.exactColor("#ffffff", "#000000")
			if i == 1 {
				color = ui.Hex("#F0B429")
			}
			if i == 2 {
				color = ui.Hex("#C4B5FD")
			}
			b := ui.ButtonBase(c.Key(fmt.Sprint("ink-color", i))).Label([]string{"自动墨色", "金黄色", "淡紫色"}[i]).Tooltip([]string{"随主题自动切换黑白", "金黄色", "淡紫色"}[i]).Size(26, 26).Radius(13).Background(color).Border(2, a.canvasBorder())
			if ed.ColorIndex() == i {
				b.Border(2, c.Theme().Accent)
			}
			if b.Clicked() {
				ed.SetColor(i)
			}
		}
		ui.Box(c).Width(1).Height(24).Background(a.canvasBorder()).Margin(0, 6)
		if a.canvasIcon(c, "撤销", false).Disabled(!ed.CanUndo()).Clicked() {
			ed.Undo()
			a.inkChanged()
		}
		if a.canvasIcon(c, "重做", false).Disabled(!ed.CanRedo()).Clicked() {
			ed.Redo()
			a.inkChanged()
		}
		if a.canvasIcon(c, "设置", a.canvasSettings).Clicked() {
			a.canvasSettings = !a.canvasSettings
		}
	})
}
func (a *App) canvasToolSettings(c *ui.Context, w, h float32) {
	ed := a.inkEditor
	ui.Column(c).Absolute().Left(w/2-190).Bottom(102).Width(380).Padding(18).Gap(14).Radius(18).Background(a.canvasSurface()).Border(1, a.canvasBorder()).ZIndex(9).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			for _, v := range []struct{ key, label string }{{"pen", "钢笔"}, {"ball", "圆珠笔"}, {"pencil", "铅笔"}, {"hand", "移动画布"}} {
				selected := ed.Tool() == v.key
				if v.key == "pen" {
					selected = ed.Tool() == "pen" && !ed.Ballpoint()
				}
				if v.key == "ball" {
					selected = ed.Tool() == "pen" && ed.Ballpoint()
				}
				if a.boardChip(c, v.label, selected).Height(36).Padding(0, 12).Clicked() {
					if v.key == "ball" || v.key == "pen" {
						ed.SetBallpoint(v.key == "ball")
					} else {
						ed.SetTool(v.key)
					}
				}
			}
		})
		ui.Textf(c, "粗细 · %.1f", ed.Width()).FontSize(13).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			for _, width := range []float32{2, 4, 8, 16, 24} {
				if a.boardChip(c, fmt.Sprintf("%.0f", width), ed.Width() == width).Height(32).Padding(0, 12).Clicked() {
					ed.SetWidth(width)
				}
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			for _, region := range []bool{false, true} {
				label := "整笔擦除"
				if region {
					label = "区域擦除"
				}
				if a.boardChip(c, label, ed.RegionEraser() == region).Height(36).Clicked() {
					ed.SetTool("eraser")
					ed.SetEraserRegion(region)
				}
			}
		})
		if a.exactButton(c, "复制画布诊断").Clicked() {
			data, err := json.MarshalIndent(ed.InputDiagnostics(), "", "  ")
			if err == nil {
				c.WriteClipboard(string(data))
				a.message = "画布诊断已复制"
			}
		}
		ui.Row(c).FillWidth().Children(func() {
			if a.exactButton(c, "切换明暗").Clicked() {
				if a.M.Theme == "dark" {
					a.M.Theme = "light"
				} else {
					a.M.Theme = "dark"
				}
				a.save()
			}
			ui.Spacer(c)
			if a.exactButton(c, "本题笔记").Clicked() {
				a.canvasNotes = !a.canvasNotes
				a.canvasSettings = false
			}
			if a.exactButton(c, "收起").Clicked() {
				a.canvasSettings = false
			}
		})
	})
}
func (a *App) canvasNoteCard(c *ui.Context, w, h float32) {
	r := a.M.Records[a.M.Round.CurrentID]
	ui.Column(c).Absolute().Right(24).Top(92).Width(332).MaxHeight(h-232).Padding(18).Gap(14).Radius(18).Background(a.canvasSurface()).Border(1, a.canvasBorder()).ZIndex(8).Shadow(0, 12, 36, 0, a.canvasShadow(.4)).Label("本题笔记卡").Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "本题笔记").FontSize(15).FontWeight(700).Grow(1)
			status := "已保存"
			if a.M.SaveError != "" {
				status = "保存失败"
			} else if a.savePending || a.saveWorker != nil {
				status = "保存中…"
			}
			ui.Text(c, status).FontSize(12).TextColor(c.Theme().TextMuted)
			if a.exactButton(c, "›").Label("收起笔记卡").Width(32).Height(32).Padding(0).FontSize(20).Clicked() {
				a.canvasNotes = false
			}
		})
		ui.Row(c).Gap(6).Wrap().Children(func() {
			for _, tag := range []string{"概念不清", "计算失误", "粗心"} {
				selected := slices.Contains(r.NoteTags, tag)
				if a.boardChip(c, tag, selected).Height(30).Padding(0, 10).FontSize(13).Clicked() {
					if selected {
						r.NoteTags = slices.DeleteFunc(r.NoteTags, func(v string) bool { return v == tag })
					} else {
						r.NoteTags = append(r.NoteTags, tag)
					}
					a.save()
				}
			}
		})
		if a.noteMode(c, a.M.Round.CurrentID) {
			ui.Scroll(c.Key("canvas-note-preview")).Height(180).FillWidth().Children(func() { a.Rich.View(c, r.Notes, 15).Label("笔记Markdown预览") })
		} else {
			a.noteEditor(c, "canvas-note-"+a.M.Round.CurrentID, a.M.Round.CurrentID, "本题笔记内容").Height(140).FontSize(15).Padding(0).Border(0, ui.Transparent).Background(ui.Transparent).Placeholder("错因：\n\n下次：")
		}
		if len(r.InkClips) > 0 {
			ui.Scroll(c.Key("note-clips")).Height(min(float32(150), h-550)).Gap(8).Children(func() {
				for _, clip := range r.InkClips {
					a.inkClipView(c, clip, 90)
				}
			})
		} else {
			ui.Text(c, "用套索选择笔迹，再点“存入本题笔记”。").FontSize(12).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).FillWidth().Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "同步到「我的笔记」").FontSize(13).TextColor(c.Theme().TextMuted).Grow(1)
			b := ui.ButtonBase(c).Label("同步到我的笔记").Tooltip("关闭仅隐藏笔记列表条目，内容仍保留").Size(36, 20).Radius(10).Background(c.Theme().Border).AlignItems(ui.Center).Padding(2)
			if !r.NotePrivate {
				b.Background(c.Theme().Accent).Justify(ui.End)
			}
			b.Children(func() { ui.Box(c).Size(16, 16).Radius(8).Background(c.Theme().AccentText) })
			if b.Clicked() {
				r.NotePrivate = !r.NotePrivate
				a.save()
			}
		})
	})
}
func (a *App) inkClipView(c *ui.Context, clip InkClip, height float32) {
	ui.Column(c.Key(clip.ID)).FillWidth().Padding(10).Gap(4).Radius(12).Background(c.Theme().Background).Border(1, c.Theme().Border).Children(func() {
		ui.Box(c).FillWidth().Height(height).Clip().Label("保存的草稿片段").Draw(func(p *ui.Painter, rect ui.Rect) {
			// A saved clip is a read-only preview. Never assign this renderer to
			// a.inkEditor: doing so made ensureInk skip ConfigureZoom and the next
			// scratch visit opened at the editor's default 100% instead of 80%.
			ed := a.inkPreview
			if ed == nil {
				var err error
				ed, err = newInkPreview()
				if err != nil {
					return
				}
				a.inkPreview = ed
			}
			ed.Appearance(a.M.Theme == "dark", max(1, a.DisplayRatio))
			ed.DrawSnapshot(p, rect, clip.Strokes, clip.Bounds, a.M.Theme == "dark")
		})
		ui.Text(c, "来自草稿").FontSize(12).TextColor(c.Theme().TextMuted)
	})
}
func (a *App) canvasProgressPanel(c *ui.Context, h float32) {
	done, right := 0, 0
	for _, id := range a.queue {
		state, _ := a.cardState(id)
		if state != "未作答" {
			done++
			if state == "答对" {
				right++
			}
		}
	}
	total := len(a.queue)
	remaining := max(0, total-done)
	ui.Box(c).Absolute().Left(0).Top(0).FillWidth().Height(4).Background(c.Theme().Border).ZIndex(10).Children(func() {
		if total > 0 {
			ui.Box(c).Height(4).WidthPercent(float32(done) * 100 / float32(total)).Background(c.Theme().Accent)
		}
	})
	ui.Column(c).Absolute().Left(24).Top(84).Width(380).MaxHeight(h-190).Padding(20).Gap(16).Radius(18).Border(1, a.canvasBorder()).Background(a.canvasSurface()).ZIndex(9).Shadow(0, 16, 40, 0, a.canvasShadow(.45)).Label("练习进度面板").Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, "今日练习进度").FontSize(15).FontWeight(700)
			ui.Textf(c, "已完成 %d · 剩余 %d", done, remaining).FontSize(13).TextColor(c.Theme().TextMuted)
		})
		ui.Box(c).Height(8).Radius(99).Background(a.canvasBorder()).Children(func() {
			if total > 0 {
				ui.Box(c).WidthPercent(float32(done) * 100 / float32(total)).Height(8).Radius(99).Background(c.Theme().Accent)
			}
		})
		elapsed := a.canvasElapsed
		if !a.canvasEntered.IsZero() {
			elapsed += time.Since(a.canvasEntered)
		}
		accuracy := "—"
		if done > 0 {
			accuracy = fmt.Sprintf("%.0f%%", float64(right)*100/float64(done))
		}
		ui.Row(c).Gap(8).Children(func() {
			for _, metric := range []struct{ value, label string }{{accuracy, "今日正确率"}, {fmt.Sprintf("%d 分", int(elapsed.Minutes())), "草稿用时"}, {fmt.Sprintf("约 %d 分", remaining*3/2), "预计还需"}} {
				ui.Column(c).Width(0).Grow(1).Padding(10, 12).Gap(2).Radius(12).Background(c.Theme().Background).Tooltip("预计用时按每题1.5分钟估算").Children(func() {
					ui.Text(c, metric.value).FontSize(17).FontWeight(700)
					ui.Text(c, metric.label).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
		})
		var ids []string
		for _, id := range a.queue {
			state, _ := a.cardState(id)
			if !a.canvasWrongOnly || state == "答错" || state == "不会" || state == "不确定" {
				ids = append(ids, id)
			}
		}
		ui.Scroll(c.Key("progress-grid")).Height(min(float32(190), h-470)).Gap(6).Children(func() {
			startOffset := 0
			limit := len(ids)
			if !a.canvasAllCards {
				current := slices.Index(ids, a.M.Round.CurrentID)
				if current >= 40 {
					startOffset = current / 40 * 40
				}
				limit = min(startOffset+40, limit)
			}
			for start := startOffset; start < limit; start += 10 {
				ui.Row(c.Key(start)).Gap(6).Children(func() {
					for col := 0; col < 10; col++ {
						idx := start + col
						if idx >= limit {
							ui.Box(c).Width(0).Grow(1)
							continue
						}
						id := ids[idx]
						num := slices.Index(a.queue, id) + 1
						state, _ := a.cardState(id)
						b := ui.ButtonBase(c.Key(id)).Label(fmt.Sprintf("进度 第%d题", num)).Tooltip(state).Height(30).Width(0).Grow(1).Radius(8).AlignItems(ui.Center).Justify(ui.Center).FontSize(13).Background(c.Theme().Surface).TextColor(c.Theme().TextMuted)
						if state == "答对" {
							b.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
						} else if state != "未作答" {
							b.Border(1, a.exactColor("#F2A29B", "#ab554f")).TextColor(a.exactColor("#F2A29B", "#ab554f")).Background(a.exactColor("#2A1F25", "#f5e8e3"))
						}
						if id == a.M.Round.CurrentID {
							b.Border(2, c.Theme().Text).Background(c.Theme().Background).TextColor(c.Theme().Text).FontWeight(700)
						}
						b.Children(func() { ui.Textf(c, "%d", num) })
						if b.Clicked() {
							a.open(id)
						}
					}
				})
			}
			if limit == 0 {
				ui.Text(c, "当前筛选没有题目").FontSize(13).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Row(c).Gap(14).Children(func() {
			for _, label := range []string{"答对", "答错", "当前", "未做"} {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					color := c.Theme().Border
					if label == "答对" {
						color = c.Theme().Accent
					} else if label == "答错" {
						color = a.exactColor("#F2A29B", "#ab554f")
					} else if label == "当前" {
						color = c.Theme().Text
					}
					ui.Box(c).Size(12, 12).Radius(4).Border(1, color).Background(color.Alpha(.2))
					ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
		})
		ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
			label := "只看答错的"
			if a.canvasWrongOnly {
				label = "显示所有状态"
			}
			if a.exactButton(c, label).FontSize(14).Border(1, a.canvasBorder()).Clicked() {
				a.canvasWrongOnly = !a.canvasWrongOnly
			}
			if a.exactButton(c, fmt.Sprintf("查看全部 %d 题", total)).Padding(0, 4).FontSize(14).TextColor(c.Theme().Accent).Clicked() {
				a.canvasAllCards = true
				a.canvasWrongOnly = false
			}
		})
	})
}

func (a *App) canvasShadow(alpha float32) ui.Color {
	if a.M.Theme != "dark" {
		alpha *= .3
	}
	return ui.RGBA(0, 0, 0, alpha)
}

func absCanvasHeight(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
func (a *App) syncCanvasCardSurface() {
	if a.canvasCardID != a.inkQuestion || a.canvasCardAnswered != a.answered || a.canvasCardDetails != a.details {
		a.canvasCardID = a.inkQuestion
		a.canvasCardAnswered, a.canvasCardDetails = a.answered, a.details
		a.canvasCardHeight = 0
		a.inkEditor.RequestRecompose()
	}
}
