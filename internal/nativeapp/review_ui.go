package nativeapp

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
	"time"
)

func (a *App) reviewSurface(c *ui.Context) ui.Color {
	if a.M.Theme == "dark" {
		return ui.Hex("#1D1F28")
	}
	return ui.Hex("#ffffff")
}
func (a *App) reviewQuestionMeta(c *ui.Context) {
	q := a.M.Current()
	r := a.M.Records[q.ID]
	t := c.Theme()
	ui.Row(c).Gap(a.qs(8)).Wrap().AlignItems(ui.Center).Children(func() {
		label := "待建档 · 今日复习"
		if !r.Wrong {
			label = "未纳入错题复习"
		} else if strings.Contains(q.Errata, "标答存疑") {
			label = "标答存疑 · 不自动排期"
		} else if at := NextReview(*q, *r); !at.IsZero() {
			if Due(*q, *r, time.Now()) {
				label = "已到期 · 今日复习"
			} else {
				label = "下次 " + at.In(zone).Format("01/02 15:04")
			}
		}
		if a.M.Round.Mode != "wrong" {
			label = "章节练习"
			if a.M.Round.Mode == "random" {
				label = "本轮随机练习"
			}
			if strings.Contains(q.Errata, "标答存疑") {
				label = "标答存疑 · 不自动排期"
			}
		}
		ui.Text(c, label).FontSize(a.qs(13)).FontWeight(500).Padding(a.qs(4), a.qs(10)).Radius(a.qs(8)).Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#C4B5FD", "#465b48"))
		if p := a.M.ActivePass(); p != nil && p.members[q.ID] {
			passLabel := p.Name + " · 本轮未完成"
			hint := "在章节或随机练习中首次提交后，才会计入该轮次进度。"
			if first, _ := a.M.passAttempts(q.ID, p.ID); first != nil {
				passLabel = p.Name + " · 本轮首答" + attemptOutcomeCompact(first)
				hint = "该题在本轮次的首次提交结果；后续作答保留在历史中。"
			}
			if a.M.Round.Mode == "wrong" {
				passLabel = p.Name + " · 复习不计入轮次"
				hint = "在错题复习中作答只更新历史与复习安排，不会完成「" + p.Name + "」的轮次进度；错题本全局共享。"
			}
			ui.Text(c, passLabel).FontSize(a.qs(13)).FontWeight(500).Description("刷题轮次进度状态："+hint).Tooltip(hint).Padding(a.qs(4), a.qs(10)).Radius(a.qs(8)).Background(t.Surface).TextColor(t.TextMuted)
		}
		sourceTag := fmt.Sprintf("原书 p%d · 第 %d 题", q.Source.Page, q.Source.Number)
		if q.Source.Page == 0 && q.Source.Number == 0 {
			sourceTag = q.KindLabel
			if sourceTag == "" {
				sourceTag = q.ID
			}
		}
		ui.Text(c, sourceTag).FontSize(a.qs(13)).FontWeight(500).Description(q.ID+" · "+q.Source.Workbook).Tooltip(q.Title+" · "+q.ID).Padding(a.qs(4), a.qs(10)).Background(t.Surface).TextColor(t.TextMuted).Radius(a.qs(8))
		a.reportButton(c, q)
		if a.inCanvasQuestion {
			return
		}
		ui.Spacer(c)
		ui.Row(c).Gap(4).Children(func() {
			a.exactToolButton(c, "scratch", "草稿纸")
			a.exactToolButton(c, "notes", "本题笔记")
			label = "加入错题本"
			if r.Wrong {
				label = "移出错题本"
			}
			if a.exactButton(c, label).Clicked() {
				r.Wrong = !r.Wrong
				a.save()
			}
		})
	})
}
func (a *App) exactToolButton(c *ui.Context, key, label string) {
	b := ui.ButtonBase(c).Label(label).Height(36).Padding(0, 12).Gap(6).Radius(10).AlignItems(ui.Center).TextColor(c.Theme().TextMuted).Children(func() { ui.Text(c, label).FontSize(14).FontWeight(500) })
	if b.Clicked() {
		if a.tool == key {
			a.closeCanvas()
			if a.inkDirty {
				return
			}
			a.tool = ""
		} else {
			a.closeCanvas()
			if a.inkDirty {
				return
			}
			a.tool = key
		}
	}
}
func (a *App) switchReviewScope(scope string) {
	if scope == a.M.Round.WrongScope {
		return
	}
	a.M.Round.WrongScope = scope
	a.M.Round.ReviewDeck = nil
	a.resetAnswer()
	a.save()
}
func (a *App) shuffleReview() {
	a.M.Round.ReviewDeck = nil
	pool := "due"
	if a.M.Round.WrongScope == "all" {
		pool = "wrong"
	}
	a.M.Round.WrongIDs = shuffled(a.M.IDs("全部学科", pool, time.Now()))
	a.resetAnswer()
	ids := a.M.Queue(a.subject, a.chapter, time.Now())
	if len(ids) > 0 {
		a.M.Round.CurrentID = ids[0]
		a.resetAnswer()
	}
	a.save()
}
func (a *App) cardState(id string) (label string, color ui.Color) {
	r := a.M.Records[id]
	if r == nil || len(r.Attempts) == 0 {
		return "未作答", ui.Transparent
	}
	last := r.Attempts[len(r.Attempts)-1]
	stamp, _ := last["time"].(string)
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil || at.In(zone).Format("2006-01-02") != time.Now().In(zone).Format("2006-01-02") {
		return "未作答", ui.Transparent
	}
	if last["outcome"] == "uncertain" {
		return "不确定", ui.Hex("#ffb86c")
	}
	if last["outcome"] == "unknown" {
		return "不会", ui.Hex("#e09888")
	}
	if last["correct"] == true {
		return "答对", ui.Hex("#5ba879")
	}
	return "答错", ui.Hex("#e09888")
}

// Freeze this round's eligible IDs so grading cannot renumber the answer card.
// New arrivals can join at the end; only explicit scope/filter/day/shuffle resets it.
func (a *App) reviewCardQueue(now time.Time, eligible []string) []string {
	day := now.In(zone).Format("2006-01-02")
	deck := a.M.Round.ReviewDeck
	changed := false
	if deck == nil || deck.Scope != a.M.Round.WrongScope || deck.Subject != a.subject || deck.Chapter != a.chapter || deck.Day != day || deck.Filter != a.M.Round.PassFilter {
		deck = &ReviewDeck{Scope: a.M.Round.WrongScope, Subject: a.subject, Chapter: a.chapter, Day: day, Filter: a.M.Round.PassFilter}
		a.M.Round.ReviewDeck = deck
		a.cardScroll = ui.ScrollState{}
		a.cardList = ui.ListState{}
		a.reviewScrollID = ""
		changed = true
	}
	valid := []string{}
	seen := map[string]bool{}
	for _, id := range deck.IDs {
		idx := a.M.Find(id)
		if idx < 0 || seen[id] {
			changed = true
			continue
		}
		q := a.M.Questions[idx]
		if a.subject != "全部学科" && q.Subject != a.subject || a.chapter != "全部章节" && q.Chapter != a.chapter {
			changed = true
			continue
		}
		seen[id] = true
		valid = append(valid, id)
	}
	for _, id := range eligible {
		if !seen[id] {
			valid = append(valid, id)
			seen[id] = true
			changed = true
		}
	}
	deck.IDs = valid
	a.followAnswerCard(valid)
	if changed {
		a.save()
	}
	return slices.Clone(valid)
}

func (a *App) followAnswerCard(ids []string) {
	if a.reviewScrollID != a.M.Round.Mode+":"+a.M.Round.CurrentID {
		index := slices.Index(ids, a.M.Round.CurrentID)
		if index >= 0 {
			y := float32(index / 6 * 40)
			if y < a.cardScroll.Y {
				a.cardScroll.Y = y
			} else if y+32 > a.cardScroll.Y+192 {
				a.cardScroll.Y = y + 32 - 192
			}
		}
		a.reviewScrollID = a.M.Round.Mode + ":" + a.M.Round.CurrentID
	}
}
