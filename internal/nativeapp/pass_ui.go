package nativeapp

import (
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

// Shared 刷题轮次 selector for the chapter and random boards. A round is
// long-term progress; a random batch is just one practice batch inside it.
const historyPassLabel = "历史练习"

func attemptOutcomeLabel(a map[string]any) string {
	switch a["outcome"] {
	case "right":
		return "答对"
	case "uncertain":
		return "答对（不确定 / 借助提示）"
	case "unknown":
		return "不会"
	default:
		return "答错"
	}
}
func attemptOutcomeCompact(a map[string]any) string {
	switch a["outcome"] {
	case "right":
		return "答对"
	case "uncertain":
		return "答对（不确定）"
	case "unknown":
		return "不会"
	default:
		return "答错"
	}
}
func passRate(right, scored int) string {
	if scored <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", float64(right)*100/float64(scored))
}

func (a *App) chaptersFor(subject string) []string {
	chapters := []string{"全部章节"}
	for _, q := range a.M.Questions {
		if subject != "全部学科" && q.Subject != subject {
			continue
		}
		if !slices.Contains(chapters, q.Chapter) {
			chapters = append(chapters, q.Chapter)
		}
	}
	return chapters
}

// passBar renders the round selector plus optional stats and creation panels.
// Selecting another round switches progress scope only; attempts, notes, the
// wrong book and review schedules are never reset by a round switch.
func (a *App) passBar(c *ui.Context) {
	t := c.Theme()
	labels := []string{historyPassLabel}
	for _, p := range a.M.Study.Passes {
		labels = append(labels, p.Name)
	}
	current := historyPassLabel
	if p := a.M.ActivePass(); p != nil {
		current = p.Name
	}
	if a.passSelect == "" {
		a.passSelect = current
	}
	if a.passSelect != current {
		id := ""
		if a.passSelect != historyPassLabel {
			for i := range a.M.Study.Passes {
				if a.M.Study.Passes[i].Name == a.passSelect {
					id = a.M.Study.Passes[i].ID
				}
			}
		}
		a.collectSave(true)
		if err := a.M.SelectPass(id); err != nil {
			a.message = err.Error()
			a.passSelect = current
		} else {
			a.message = ""
			a.passSelect = ""
			a.resetAnswer()
		}
		if p := a.M.ActivePass(); p != nil {
			current = p.Name
		} else {
			current = historyPassLabel
		}
		if a.passSelect == "" {
			a.passSelect = current
		}
	}
	ui.Column(c.Key("pass-bar")).Gap(12).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				if p := a.M.ActivePass(); p != nil {
					s := a.M.PassStats("全部学科", "全部章节")
					ui.Textf(c, "当前轮次：%s · 已完成 %d / %d · 本轮首次答对率 %s", p.Name, s.Done, s.Total, passRate(s.Right, s.Scored)).FontSize(14).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "当前轮次：历史练习 · 汇总本机全部作答，不划分轮次").FontSize(14).TextColor(t.TextMuted)
				}
			})
			ui.Select(c, &a.passSelect, labels).Width(180).Label("当前刷题轮次")
			if a.boardAction(c, "查看统计", false).Disabled(len(a.M.Study.Passes) == 0).Clicked() {
				a.passStats = !a.passStats
				a.passCreate = false
			}
			if a.boardAction(c, "新建轮次", true).Clicked() {
				a.passCreate = !a.passCreate
				a.passStats = false
				if a.passCreate {
					if !slices.Contains(a.subjects(), a.passSubject) {
						a.passSubject = "全部学科"
					}
					if !slices.Contains(a.chaptersFor(a.passSubject), a.passChapter) {
						a.passChapter = "全部章节"
					}
				}
			}
		})
		if a.passStats {
			a.passStatsCard(c)
		}
		if a.passCreate {
			a.passCreateCard(c)
		}
		if a.message != "" {
			ui.Text(c, a.message).FontSize(13).TextColor(t.Danger)
		}
	})
}

func (a *App) passStatsCard(c *ui.Context) {
	t := c.Theme()
	a.boardCard(c.Key("pass-stats")).Padding(20, 24).Gap(12).Children(func() {
		ui.Text(c, "轮次统计 · 按本轮首次有效提交计算").FontSize(14).FontWeight(700)
		ui.Text(c, "后续巩固作答保留在历史中，不改写首答成绩；到期复习默认不计入轮次完成。").FontSize(13).TextColor(t.TextMuted)
		for _, p := range a.M.Study.Passes {
			s := a.M.passStatsFor(&p, "全部学科", "全部章节")
			active := p.ID == a.M.Study.Active
			row := a.boardCard(c.Key("pass-stat-"+p.ID)).Padding(12, 16).Gap(8)
			if active {
				row.Border(1, t.Accent)
			}
			row.Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.Name).FontSize(15).FontWeight(700)
					if active {
						ui.Text(c, "当前").FontSize(12).Padding(3, 8).Radius(7).Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#C4B5FD", "#465b48"))
					}
				})
				ui.Row(c).Gap(16).FontSize(13).TextColor(t.TextMuted).Children(func() {
					ui.Textf(c, "已完成 %d / %d", s.Done, s.Total)
					ui.Textf(c, "首次答对率 %s", passRate(s.Right, s.Scored))
					ui.Textf(c, "答对 %d · 不确定 %d · 答错/不会 %d · 存疑 %d", s.Right, s.Uncertain, s.Wrong, s.Disputed)
				})
			})
		}
	})
}

func (a *App) passCreateCard(c *ui.Context) {
	t := c.Theme()
	a.boardCard(c.Key("pass-create")).Padding(20, 24).Gap(12).Children(func() {
		ui.Text(c, "新建刷题轮次").FontSize(14).FontWeight(700)
		ui.Text(c, "新轮次内所有题目均为“未完成”；历史作答、笔记、错题本与复习安排全部保留。").FontSize(13).TextColor(t.TextMuted)
		if !slices.Contains(a.subjects(), a.passSubject) {
			a.passSubject = "全部学科"
		}
		chapters := a.chaptersFor(a.passSubject)
		if !slices.Contains(chapters, a.passChapter) {
			a.passChapter = "全部章节"
		}
		ui.Row(c).Gap(12).AlignItems(ui.End).Children(func() {
			ui.TextInput(c, &a.passName).Label("轮次名称").Placeholder("如：408 二刷").Grow(1)
			ui.Select(c, &a.passSubject, a.subjects()).Width(150).Label("学科范围")
			ui.Select(c, &a.passChapter, chapters).Width(170).Label("章节范围")
			if a.boardAction(c, "创建轮次", true).Clicked() {
				a.collectSave(true)
				if err := a.M.CreatePass(a.passName, a.passSubject, a.passChapter, time.Now()); err != nil {
					a.message = err.Error()
				} else {
					a.message = ""
					a.passName = ""
					a.passCreate = false
					a.passSelect = ""
				}
			}
		})
	})
}
