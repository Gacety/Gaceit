package nativeapp

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
)

// The user-supplied HTML is the visual authority for this page.
// These constants mirror its inline CSS, rather than the legacy app styles.
var reviewBG = ui.Hex("#13141A")
var reviewSide = ui.Hex("#181A21")
var reviewCard = ui.Hex("#1D1F28")
var reviewBorder = ui.Hex("#2B2E3A")
var reviewMuted = ui.Hex("#A5A9B8")
var reviewFaint = ui.Hex("#8D92A3")
var reviewAccent = ui.Hex("#A78BFA")

func (a *App) reviewTheme() *ui.Theme {
	t := *a.theme()
	t.Font = "Review Noto"
	t.FontSize = 14
	t.Radius = 10
	if a.M.Theme == "dark" {
		t.Background = reviewBG
		t.Surface = ui.Hex("#232632")
		t.Border = reviewBorder
		t.Text = ui.Hex("#ECEDF3")
		t.TextMuted = reviewMuted
		t.Accent = reviewAccent
		t.AccentText = ui.Hex("#1A1230")
		t.SurfaceHover = ui.Hex("#292c38")
		t.Selection = ui.Hex("#2A2545")
		t.InactiveAccent = t.Accent
		t.AccentHover = t.Accent
		t.AccentPressed = t.Accent
	}
	return &t
}
func (a *App) exactColor(dark, light string) ui.Color {
	if a.M.Theme == "dark" {
		return ui.Hex(dark)
	}
	return ui.Hex(light)
}
func (a *App) exactButton(c *ui.Context, label string) ui.Element {
	return ui.ButtonBase(c).Label(label).Height(36).Padding(0, 12).Gap(6).Radius(10).AlignItems(ui.Center).Justify(ui.Center).FontSize(14).FontWeight(500).TextColor(c.Theme().TextMuted).Children(func() { ui.Text(c, label).SingleLine() })
}
func (a *App) reviewPage(c *ui.Context) {
	c.SetTheme(a.reviewTheme())
	t := c.Theme()
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontWeight(500).Children(func() {
		a.exactReviewSidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Height(56).Shrink(0).Padding(0, 40).Gap(16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				q := a.M.Current()
				ui.RichText(c, ui.Span{Text: a.readerName() + " / " + q.Subject + " / ", Color: a.exactColor("#8D92A3", "#777b72"), Size: 14, Weight: 400}, ui.Span{Text: q.Chapter, Color: t.Text, Size: 14, Weight: 400}).Grow(1).MinWidth(0).SingleLine()
				status := "已保存到本机"
				if a.M.SaveError != "" {
					status = "保存失败"
				} else if a.savePending {
					status = "保存中…"
				}
				ui.Text(c, status).FontSize(14).FontWeight(400).TextColor(a.exactColor("#8D92A3", "#777b72"))
				if a.exactButton(c, "题目目录").Border(1, t.Border).TextColor(t.Text).Clicked() {
					a.directory = !a.directory
				}
			})
			ui.Row(c).Grow(1).MinHeight(0).Padding(32, 40, 24, 40).Gap(32).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(16).Children(func() {
					ui.Scroll(c.Key("review-reading-" + a.M.Round.CurrentID)).Grow(1).MinHeight(0).Gap(16).Children(func() {
						if a.directory {
							a.directoryView(c)
						}
						if len(a.queue) == 0 {
							a.empty(c)
							return
						}
						ui.Column(c).Label(a.readerAreaLabel()).FillWidth().Gap(20).Padding(28, 32).Background(a.reviewSurface(c)).Border(1, t.Border).Radius(20).ContextMenu(a.reviewRecallMenu).Children(func() { a.question(c) })
					})
					a.exactReviewFooter(c)
				})
				if a.tool != "" {
					a.writing(c)
				} else {
					a.exactReviewRail(c)
				}
			})
		})
	})
}
func (a *App) exactReviewSidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(240).Shrink(0).FillHeight().Padding(24, 16).Gap(4).Background(a.exactColor("#181A21", "#f0efea")).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Padding(0, 14, 20, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, reviewIcons["brand"]).Size(24, 24).TextColor(t.Accent)
			ui.Text(c, a.bankDisplayTitle()).FontSize(20).FontWeight(700)
		})
		for _, item := range []struct{ key, label, icon string }{{"practice", "章节练习", "book"}, {"wrong", "错题复习", "clock"}, {"random", "随机练习", "shuffle"}, {"notes", "我的笔记", "note"}} {
			b := ui.ButtonBase(c.Key(item.key)).Label(item.label).FillWidth().Height(44).Padding(0, 14).Gap(12).Radius(12).AlignItems(ui.Center).TextColor(t.TextMuted).FontSize(15).FontWeight(500)
			if a.M.Round.Mode == item.key {
				b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48"))
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.Children(func() {
				ui.Icon(c, reviewIcons[item.icon]).Size(18, 18)
				ui.Text(c, item.label).Grow(1)
				if item.key == "wrong" {
					ui.Textf(c, "%d", a.dueCount()).FontSize(12).TextColor(a.exactColor("#C4B5FD", "#465b48"))
				}
			})
			if b.Clicked() {
				a.mode(item.key)
			}
		}
		ui.Text(c, "题库").FontSize(12).FontWeight(500).TextColor(a.exactColor("#8D92A3", "#777b72")).LetterSpacing(1).Padding(24, 14, 6, 14)
		for _, subject := range a.subjects() {
			b := ui.ButtonBase(c.Key(subject)).Height(36).FillWidth().Padding(0, 14).AlignItems(ui.Center).FontSize(14).FontWeight(400).TextColor(t.TextMuted).Label(subject)
			if a.subject == subject {
				b.TextColor(a.exactColor("#C4B5FD", "#465b48"))
			}
			b.Children(func() {
				ui.Text(c, subject).Grow(1)
				if subject == "全部学科" {
					ui.Textf(c, "%d", len(a.M.Questions)).FontSize(12).TextColor(a.exactColor("#8D92A3", "#777b72"))
				}
			})
			if b.Clicked() {
				a.subject = subject
				a.chapter = "全部章节"
				a.resetAnswer()
			}
		}
		ui.Spacer(c)
		ui.Column(c).Padding(0, 14).Gap(12).Children(func() {
			ui.Text(c, a.bankSummaryText()).FontSize(12).LineHeight(1.6).FontWeight(400).TextColor(a.exactColor("#8D92A3", "#777b72"))
			ui.Row(c).Gap(8).Wrap().Children(func() {
				theme := "浅色"
				if a.M.Theme != "dark" {
					theme = "深色"
				}
				if a.exactButton(c, theme).FontSize(13).Padding(0, 10).Border(1, t.Border).Clicked() {
					if a.M.Theme == "dark" {
						a.M.Theme = "light"
					} else {
						a.M.Theme = "dark"
					}
					a.save()
				}
				if a.exactButton(c, "导出").FontSize(13).Padding(0, 10).Border(1, t.Border).Clicked() {
					a.exportBackup()
				}
				if a.exactButton(c, "导入").FontSize(13).Padding(0, 10).Border(1, t.Border).Clicked() {
					a.importBackup()
				}
				if isWebPlatform() {
					label := "全屏"
					if a.isFullscreen() {
						label = "退出全屏"
					}
					if a.exactButton(c, label).FontSize(13).Padding(0, 10).Border(1, t.Border).Clicked() {
						a.toggleFullscreen()
					}
				}
			})
		})
	})
}
func (a *App) exactReviewFooter(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Shrink(0).Height(48).Gap(12).AlignItems(ui.Center).Children(func() {
		i := slices.Index(a.queue, a.M.Round.CurrentID)
		if a.exactButton(c, "上一题").Height(48).Padding(0, 20).Radius(12).Border(1, t.Border).FontSize(15).AccessDisabled(i <= 0).Clicked() {
			a.navigate(-1)
		}
		hint := "快捷键：Enter 提交"
		if a.M.Current().IsSubjective() {
			hint = "填写作答后可点击 AI 批改或自评提交"
		} else if len(a.choices) == 1 {
			hint = "快捷键：1–4 选择 · Enter 提交"
		}
		ui.Text(c, hint).FontSize(13).FontWeight(400).TextColor(a.exactColor("#8D92A3", "#777b72"))
		ui.Spacer(c)
		if a.exactButton(c, "下一题").Height(48).Padding(0, 20).Radius(12).Border(1, a.exactColor("#3A3E4C", "#c8cbbf")).Background(t.Surface).TextColor(t.Text).FontSize(15).AccessDisabled(i < 0 || i >= len(a.queue)-1).Clicked() {
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
		if a.exactButton(c, label).Height(48).Padding(0, 32).Radius(12).Background(t.Accent).TextColor(t.AccentText).FontWeight(700).FontSize(15).Disabled(len(a.queue) == 0 || (!a.answered && a.aiGrading)).Clicked() {
			if a.answered {
				if i < len(a.queue)-1 {
					a.navigate(1)
				} else {
					a.resetAnswer()
				}
			} else {
				a.submit(false)
			}
		}
	})
}
func (a *App) exactReviewRail(c *ui.Context) {
	a.followAnswerCard(a.queue)
	t := c.Theme()
	ui.Column(c.Key("review-rail")).Width(300).Shrink(0).Gap(16).Label(a.readerRailLabel()).Children(func() {
		ui.Column(c).Padding(20).Gap(14).Radius(20).Background(a.reviewSurface(c)).Border(1, t.Border).Children(func() {
			if a.M.Round.Mode == "wrong" {
				ui.Row(c).Padding(4).Background(t.Background).Radius(12).Children(func() {
					for _, item := range []struct {
						scope, label string
						count        int
					}{{"due", "今日到期", a.dueCount()}, {"all", "全部", len(a.M.IDs("全部学科", "wrong", time.Now()))}} {
						b := ui.ButtonBase(c.Key(item.scope)).Grow(1).Height(36).Radius(9).AlignItems(ui.Center).Justify(ui.Center).FontSize(14).FontWeight(500).TextColor(t.TextMuted).Label(item.label)
						if item.scope == a.M.Round.WrongScope {
							b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48")).FontWeight(700)
						}
						b.Children(func() { ui.Textf(c, "%s %d", item.label, item.count) })
						if b.Clicked() {
							a.switchReviewScope(item.scope)
						}
					}
				})
				if p := a.M.ActivePass(); p != nil {
					// Round filters view the shared wrong book through this round's
					// first/latest outcomes; they never move questions in or out.
					ui.Column(c).Gap(6).Children(func() {
						ui.Textf(c, "%s · 轮次筛选", p.Name).FontSize(12).FontWeight(500).TextColor(a.exactColor("#8D92A3", "#777b72"))
						ui.Row(c).Gap(6).Wrap().Children(func() {
							for _, item := range []struct{ key, label string }{
								{"", "全部错题"},
								{"pass-new", "新增错题"},
								{"pass-again", "再次做错"},
								{"pass-improved", "已答对旧错题"},
								{"pass-first-wrong", "首次做错"},
							} {
								b := ui.ButtonBase(c.Key("pass-filter-"+item.key)).Height(28).Padding(0, 10).Radius(8).AlignItems(ui.Center).Justify(ui.Center).FontSize(12).FontWeight(500).TextColor(t.TextMuted).Background(t.Background).Label(item.label)
								if a.M.Round.PassFilter == item.key {
									b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48"))
								}
								b.Children(func() { ui.Text(c, item.label).FontSize(12) })
								if b.Clicked() {
									a.M.Round.PassFilter = item.key
									a.M.Round.ReviewDeck = nil
									a.resetAnswer()
									a.save()
								}
							}
						})
					})
				}
			} else {
				ui.Row(c).Height(44).Padding(4).Gap(8).Background(t.Background).Radius(12).AlignItems(ui.Center).Children(func() {
					title, detail := "本章练习", "顺序作答"
					if a.chapter == "全部章节" {
						title = "章节练习"
						detail = "全部章节"
					}
					if a.M.Round.Mode == "random" {
						title, detail = "本轮随机练习", "固定题序"
					}
					ui.Text(c, title).Height(36).Padding(7, 12).FontSize(14).FontWeight(700).Radius(9).Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48"))
					ui.Spacer(c)
					ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted).Padding(0, 4)
					ui.Text(c, "错题本全局共享；轮次筛选只影响显示，不改变错题归属。做错后约 10 分钟到期重学。").FontSize(11).LineHeight(1.5).TextColor(a.exactColor("#8D92A3", "#777b72"))
				})
			}
			pos := slices.Index(a.queue, a.M.Round.CurrentID) + 1
			if len(a.queue) == 0 {
				pos = 0
			}
			ui.Row(c).Height(46).Gap(6).AlignItems(ui.End).Children(func() {
				ui.Textf(c, "%d", pos).FontSize(32).FontWeight(700)
				ui.Textf(c, "/ %d", len(a.queue)).FontSize(16).FontWeight(400).TextColor(a.exactColor("#8D92A3", "#777b72")).Padding(0, 0, 3, 0)
			})
			ui.Box(c).Height(6).Radius(99).Background(t.Border).Children(func() {
				if len(a.queue) > 0 {
					ui.Box(c).WidthPercent(float32(pos) * 100 / float32(len(a.queue))).Height(6).Radius(99).Background(t.Accent)
				}
			})
		})
		ui.Column(c).Padding(20).Gap(14).Radius(20).Background(a.reviewSurface(c)).Border(1, t.Border).Children(func() {
			ui.Row(c).Height(32).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "答题卡").FontSize(15).FontWeight(700).Grow(1)
				label, accessLabel := "重新随机", "重新随机排序"
				if a.M.Round.Mode == "practice" {
					label, accessLabel = "返回章节", "返回章节列表"
				}
				if a.M.Round.Mode == "random" {
					label, accessLabel = "调整范围", "返回随机设置"
				}
				b := a.exactButton(c, label).Label(accessLabel).Height(32)
				if b.Clicked() {
					if a.M.Round.Mode == "wrong" {
						a.shuffleReview()
					} else {
						a.mode(a.M.Round.Mode)
					}
				}
			})
			ids := a.queue
			if len(ids) == 0 {
				ui.Scroll(c.Key("review-cards")).Height(192).Children(func() { ui.Text(c, "当前范围没有题目").FontSize(13).TextColor(t.TextMuted) })
			} else {
				ui.List(c.Key("review-cards"), &a.cardList, (len(ids)+5)/6).TrackScroll(&a.cardScroll).Height(192).Gap(8).Rows(func(row ui.ListRow) {
					c := row.Context
					start := row.Index * 6
					ui.Row(c.Key(start)).Gap(8).Children(func() {
						for col := 0; col < 6; col++ {
							i := start + col
							if i >= len(ids) {
								ui.Box(c).Width(0).Grow(1)
								continue
							}
							id := ids[i]
							state, color := a.cardState(id)
							current := id == a.M.Round.CurrentID
							b := ui.ButtonBase(c.Key(id)).Width(0).Grow(1).Height(32).Radius(8).Justify(ui.Center).AlignItems(ui.Center).FontSize(13).FontWeight(400).Label(fmt.Sprintf("答题卡 第%d题", i+1)).Description(state).Tooltip(a.M.Questions[a.M.Find(id)].Title + " · " + state)
							bg := t.Surface
							fg := t.TextMuted
							if color != ui.Transparent {
								bg = color.Alpha(.16)
								fg = color
							}
							if current {
								bg = t.Accent
								fg = t.AccentText
								b.FontWeight(700)
							}
							b.Background(bg).TextColor(fg).Children(func() { ui.Textf(c, "%d", i+1) })
							if b.Clicked() {
								a.open(id)
							}
						}
					})
				})
			}
		})
		help := "先筛选到期题，再按固定随机轮次复习。标答存疑题保留在全部错题中核对。"
		if a.M.Round.Mode == "practice" {
			help = "按章节顺序练习，点击答题卡可跳转。卡片标记今日作答状态，原表历史独立保留。"
		}
		if a.M.Round.Mode == "random" {
			help = "本轮题序与进度自动保存，卡片标记今日作答状态。点击题号跳转，不会重新抽题。"
		}
		ui.Text(c, help).Padding(0, 4).FontSize(12).FontWeight(400).LineHeight(1.6).TextColor(a.exactColor("#8D92A3", "#777b72"))
	})
}

var reviewIcons = map[string]*ui.SVG{}

func init() {
	paths := map[string]string{
		"brand": `<path d="M4 5a2 2 0 012-2h13v16H6a2 2 0 00-2 2zM4 19V5"/>`, "book": `<path d="M4 5a2 2 0 012-2h13v16H6a2 2 0 00-2 2z"/>`, "clock": `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`, "shuffle": `<path d="M16 3h5v5M4 20L21 3M21 16v5h-5M15 15l6 6M4 4l5 5"/>`, "note": `<path d="M6 3h9l4 4v14H6zM14 3v5h5"/>`, "pen": `<path d="m16 3 5 5L8 21H3v-5zM14 5l5 5"/>`}
	for name, path := range paths {
		reviewIcons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
}

func (a *App) reviewRecallMenu(m *ui.Menu) {
	if m.Item("独立作答").Radio(a.quality == "independent").Chosen() {
		a.quality = "independent"
	}
	if m.Item("不确定 / 用过提示").Radio(a.quality == "uncertain").Chosen() {
		a.quality = "uncertain"
	}
	if m.Item("不会，先看解析").Disabled(a.answered).Chosen() {
		a.submit(true)
	}
	m.Separator()
	m.EditItems()
}

func (a *App) readerName() string {
	switch a.M.Round.Mode {
	case "practice":
		return "章节练习"
	case "random":
		return "随机练习"
	default:
		return "错题复习"
	}
}
func (a *App) readerAreaLabel() string {
	if a.M.Round.Mode == "wrong" {
		return "错题题目区域"
	}
	return a.readerName() + "题目区域"
}
func (a *App) readerRailLabel() string {
	if a.M.Round.Mode == "wrong" {
		return "复习答题卡侧栏"
	}
	return a.readerName() + "答题卡侧栏"
}
