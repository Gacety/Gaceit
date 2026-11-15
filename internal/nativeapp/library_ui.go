package nativeapp

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Native implementations of the three 1440x900 boards in the supplied HTML.
// The mock's counts and sample notes are replaced by actual local learning data.
func (a *App) libraryPage(c *ui.Context) {
	c.SetTheme(a.reviewTheme())
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontWeight(400).Children(func() {
		a.exactReviewSidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).FillHeight().Padding(32, 40).Gap(24).Children(func() {
			switch a.M.Round.Mode {
			case "practice":
				a.chapterPage(c)
			case "random":
				a.randomPage(c)
			case "notes":
				a.libraryNotesPage(c)
			}
		})
	})
}
func (a *App) boardTitle(c *ui.Context, title, subtitle string) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, title).FontSize(28).FontWeight(700)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(14).TextColor(c.Theme().TextMuted)
		}
	})
}
func (a *App) boardCard(c *ui.Context) ui.Element {
	return ui.Column(c).Background(a.reviewSurface(c)).Border(1, c.Theme().Border).Radius(18)
}
func (a *App) boardAction(c *ui.Context, label string, primary bool) ui.Element {
	b := a.exactButton(c, label).Height(44).Padding(0, 24).Radius(12).FontSize(15)
	if primary {
		b.Background(c.Theme().Accent).TextColor(c.Theme().AccentText).FontWeight(700)
	} else {
		b.Background(c.Theme().Surface).TextColor(c.Theme().Text).Border(1, a.exactColor("#3A3E4C", "#c8cbbf"))
	}
	return b
}
func (a *App) boardChip(c *ui.Context, label string, selected bool) ui.Element {
	b := a.exactButton(c, label).Height(40).Padding(0, 16).Radius(10).Background(c.Theme().Background).Border(1, a.exactColor("#363A48", "#c8cbbf"))
	if selected {
		b.Background(a.exactColor("#2A2545", "#e4e9df")).Border(1, c.Theme().Accent).TextColor(a.exactColor("#E4DAFF", "#465b48"))
	}
	return b
}

type chapterSummary struct {
	name               string
	ids                []string
	done, right, wrong int
}

func (a *App) chapterSummaries(subject string) []chapterSummary {
	p := a.M.ActivePass()
	var chapters []chapterSummary
	for _, q := range a.M.Questions {
		if q.Subject != subject {
			continue
		}
		if p != nil && !p.members[q.ID] {
			continue
		}
		i := slices.IndexFunc(chapters, func(ch chapterSummary) bool { return ch.name == q.Chapter })
		if i < 0 {
			chapters = append(chapters, chapterSummary{name: q.Chapter})
			i = len(chapters) - 1
		}
		ch := &chapters[i]
		ch.ids = append(ch.ids, q.ID)
		r := a.M.Records[q.ID]
		// With an active round, progress counts only that round's first
		// submissions; source history never fabricates round completion.
		if p != nil {
			if first, _ := a.M.passAttempts(q.ID, p.ID); first != nil {
				ch.done++
				if first["outcome"] == "right" {
					ch.right++
				}
			}
		} else {
			// Source snapshots remain source history, never fabricated local attempts.
			if len(r.Attempts) > 0 {
				ch.done++
				if r.Attempts[len(r.Attempts)-1]["correct"] == true {
					ch.right++
				}
			} else if q.History.Result != "" {
				ch.done++
				if q.History.Result == "正确" {
					ch.right++
				}
			}
		}
		if r.Wrong {
			ch.wrong++
		}
	}
	return chapters
}
func questionMetaLabel(q Question) string {
	if q.Source.Number > 0 {
		return fmt.Sprintf("%s · %s · 第 %d 题", q.Subject, q.Chapter, q.Source.Number)
	}
	if strings.TrimSpace(q.Title) != "" {
		return fmt.Sprintf("%s · %s", q.Subject, q.Title)
	}
	return fmt.Sprintf("%s · %s · %s", q.Subject, q.Chapter, q.ID)
}

func (a *App) chapterPage(c *ui.Context) {
	subs := a.subjectChoices()
	if len(subs) > 0 && !slices.Contains(subs, a.chapterSubject) {
		a.chapterSubject = a.M.Current().Subject
		if !slices.Contains(subs, a.chapterSubject) {
			a.chapterSubject = subs[0]
		}
	}
	if a.subject != "全部学科" {
		a.chapterSubject = a.subject
	}
	chapters := a.chapterSummaries(a.chapterSubject)
	done, total := 0, 0
	for _, ch := range chapters {
		done += ch.done
		total += len(ch.ids)
	}
	pass := a.M.ActivePass()
	summary := fmt.Sprintf("%s · 共 %d 章 · 已做 %d / %d 题", a.chapterSubject, len(chapters), done, total)
	if pass != nil {
		summary = fmt.Sprintf("%s · %s · 本轮已做 %d / %d 题", a.chapterSubject, pass.Name, done, total)
	}
	ui.Row(c).AlignItems(ui.End).Gap(12).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "章节练习").FontSize(28).FontWeight(700)
			ui.Row(c).Gap(18).AlignItems(ui.Center).Children(func() {
				ui.Text(c, summary).FontSize(14).TextColor(c.Theme().TextMuted)
				for _, v := range []struct {
					label string
					color ui.Color
				}{{"答对", c.Theme().Accent}, {"答错", a.exactColor("#F2A29B", "#ab554f")}} {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(10, 10).Radius(3).Background(v.color)
						ui.Text(c, v.label).FontSize(14).TextColor(c.Theme().TextMuted)
					})
				}
			})
		})
		ui.Row(c).Padding(4).Gap(2).Radius(12).Background(a.reviewSurface(c)).Border(1, c.Theme().Border).Children(func() {
			for _, subject := range a.subjectChoices() {
				b := a.exactButton(c, subject).Padding(0, 14).Radius(9)
				if subject == a.chapterSubject {
					b.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48")).FontWeight(700)
				}
				if b.Clicked() {
					a.chapterSubject = subject
					a.subject = "全部学科"
				}
			}
		})
	})
	current := a.M.Current()
	ui.Row(c).Padding(18, 24).Gap(16).AlignItems(ui.Center).Radius(18).Background(a.exactColor("#26213F", "#e4e9df")).Border(1, a.exactColor("#4B4470", "#b7c4b0")).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "继续上次").FontSize(13).FontWeight(500).TextColor(a.exactColor("#C4B5FD", "#465b48"))
			ui.Text(c, questionMetaLabel(*current)).FontSize(17).FontWeight(700)
		})
		if a.boardAction(c, "继续练习", true).Clicked() {
			a.subject = current.Subject
			a.chapter = current.Chapter
			a.open(current.ID)
		}
	})
	ui.Scroll(c.Key("chapter-grid-" + a.chapterSubject)).Grow(1).MinHeight(0).Gap(16).Children(func() {
		for start := 0; start < len(chapters); start += 3 {
			ui.Row(c.Key(start)).Gap(16).AlignItems(ui.Stretch).Children(func() {
				for col := 0; col < 3; col++ {
					i := start + col
					if i >= len(chapters) {
						ui.Box(c).Width(0).Grow(1)
						continue
					}
					ch := chapters[i]
					active := current.Subject == a.chapterSubject && current.Chapter == ch.name
					card := a.boardCard(c.Key(ch.name)).Width(0).Grow(1).MinWidth(0).Padding(20).Gap(14)
					if active {
						card.Border(2, c.Theme().Accent)
					}
					card.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Textf(c, "第 %02d 章", i+1).FontSize(13).FontWeight(700).TextColor(a.exactColor("#8D92A3", "#777b72"))
							if active {
								ui.Text(c, "进行中").FontSize(12).FontWeight(500).Padding(3, 8).Radius(7).Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#C4B5FD", "#465b48"))
							} else if ch.done == len(ch.ids) {
								ui.Text(c, "已完成").FontSize(12).TextColor(c.Theme().TextMuted)
							}
						})
						ui.Text(c, ch.name).FontSize(18).FontWeight(700).LineHeight(1.4).MinHeight(50)
						ui.Column(c).Gap(8).Children(func() {
							ui.Row(c).Height(6).Radius(99).Clip().Background(c.Theme().Border).Children(func() {
								if ch.right > 0 {
									ui.Box(c).WidthPercent(float32(ch.right) * 100 / float32(len(ch.ids))).Height(6).Background(c.Theme().Accent)
								}
								if ch.done > ch.right {
									ui.Box(c).WidthPercent(float32(ch.done-ch.right) * 100 / float32(len(ch.ids))).Height(6).Background(a.exactColor("#F2A29B", "#ab554f"))
								}
							})
							ui.Row(c).Justify(ui.SpaceBetween).FontSize(13).TextColor(c.Theme().TextMuted).Children(func() {
								label := "尚未开始"
								if ch.done > 0 {
									label = fmt.Sprintf("已做 %d · 正确率 %.0f%%", ch.done, float64(ch.right)*100/float64(ch.done))
								}
								ui.Text(c, label)
								ui.Textf(c, "%d / %d", ch.done, len(ch.ids))
							})
						})
						ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
							badge := ui.Textf(c, "错题 %d", ch.wrong).FontSize(13).TextColor(a.exactColor("#8D92A3", "#777b72"))
							if ch.wrong > 0 {
								badge.FontWeight(700).Padding(4, 10).Radius(8).Background(a.exactColor("#2A1F25", "#f5e8e3")).TextColor(a.exactColor("#F2A29B", "#ab554f")).Border(1, a.exactColor("#F2A29B", "#ab554f"))
							}
							label := "复习"
							if active {
								label = "继续"
							} else if ch.done == 0 {
								label = "开始"
							}
							if a.boardAction(c, label, active).Label("章节 "+ch.name).Height(36).Padding(0, 16).FontSize(14).Clicked() {
								a.subject = a.chapterSubject
								a.chapter = ch.name
								id := ch.ids[0]
								if active {
									id = current.ID
								} else if p := a.M.ActivePass(); p != nil {
									// Round progress: continue at the first uncompleted question.
									for _, qid := range ch.ids {
										if first, _ := a.M.passAttempts(qid, p.ID); first == nil {
											id = qid
											break
										}
									}
								} else {
									for _, qid := range ch.ids {
										if len(a.M.Records[qid].Attempts) == 0 && a.M.Questions[a.M.Find(qid)].History.Result == "" {
											id = qid
											break
										}
									}
								}
								a.open(id)
							}
						})
					})
				}
			})
		}
	})
}

func (a *App) selectedDrawSubjects() []string {
	if a.M.Round.Settings.Subjects != nil {
		return a.M.Round.Settings.Subjects
	}
	if a.M.Round.Settings.Subject == "全部学科" {
		return a.subjectChoices()
	}
	return []string{a.M.Round.Settings.Subject}
}
func (a *App) randomPage(c *ui.Context) {
	a.boardTitle(c, "随机练习", "设定范围和题量，系统随机抽题")
	ui.Scroll(c.Key("random-setup")).Grow(1).MinHeight(0).Gap(20).Children(func() {
		a.passBar(c)
		ui.Row(c).Gap(24).AlignItems(ui.Start).Children(func() {
			a.boardCard(c).Grow(1).MinWidth(0).Padding(28).Gap(24).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "范围").FontSize(14).FontWeight(700)
					ui.Row(c).Gap(8).Wrap().Children(func() {
						for _, subject := range a.subjectChoices() {
							selected := slices.Contains(a.selectedDrawSubjects(), subject)
							if a.boardChip(c, subject, selected).Clicked() {
								selectedSubjects := slices.Clone(a.selectedDrawSubjects())
								if selected {
									selectedSubjects = slices.DeleteFunc(selectedSubjects, func(v string) bool { return v == subject })
								} else {
									selectedSubjects = append(selectedSubjects, subject)
								}
								if selectedSubjects == nil {
									selectedSubjects = []string{}
								}
								a.M.Round.Settings.Subjects = selectedSubjects
								a.save()
							}
						}
					})
				})
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "题量").FontSize(14).FontWeight(700)
					ui.Row(c).Gap(8).Wrap().Children(func() {
						for _, count := range []string{"10", "20", "50", "自定义"} {
							selected := a.M.Round.Settings.Count == count && !a.randomCustom || count == "自定义" && a.randomCustom
							label := count
							if count != "自定义" {
								label += " 题"
							}
							if a.boardChip(c, label, selected).Clicked() {
								a.randomCustom = count == "自定义"
								if !a.randomCustom {
									a.M.Round.Settings.Count = count
									a.save()
								}
							}
						}
					})
					if a.randomCustom {
						if ui.TextInput(c, &a.M.Round.Settings.Count).Label("自定义题量").Placeholder("输入题数").Width(150).Height(40).Changed() {
							a.saveSoon()
						}
					}
				})
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "题目来源").FontSize(14).FontWeight(700)
					ui.Row(c).Gap(8).Wrap().Children(func() {
						items := []struct{ key, label string }{{"all", "全部题目"}, {"unpracticed", "从未作答"}, {"wrong", "当前错题本"}}
						if a.M.ActivePass() != nil {
							items = append(items, struct{ key, label string }{"pass-unfinished", "本轮未完成"}, struct{ key, label string }{"pass-first-wrong", "本轮首次做错"})
						}
						for _, item := range items {
							if a.boardChip(c, item.label, a.M.Round.Settings.Range == item.key).Clicked() {
								a.M.Round.Settings.Range = item.key
								a.save()
							}
						}
					})
					if a.M.Round.Settings.Range == "due" {
						ui.Text(c, "当前范围：今日到期错题").FontSize(13).TextColor(c.Theme().TextMuted)
					}
					if a.M.Round.Settings.Range == "pass-unfinished" && a.M.ActivePass() != nil {
						ui.Textf(c, "只抽 %s 中尚未首次提交的题；章节练习和随机抽题共用这一进度。", a.M.ActivePass().Name).FontSize(13).TextColor(c.Theme().TextMuted)
					}
				})
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "模式").FontSize(14).FontWeight(700)
					ui.Row(c).Gap(12).Children(func() {
						for _, exam := range []bool{false, true} {
							title, desc := "练习模式", "每题提交后立刻看答案和解析"
							if exam {
								title, desc = "模拟考试", "限时作答，交卷后统一看结果"
							}
							b := ui.ButtonBase(c).Label(title).Width(0).Grow(1).MinWidth(0).Padding(14, 16).Gap(4).Radius(12).Border(1, c.Theme().Border).Background(c.Theme().Background)
							if !exam {
								b.Border(1, c.Theme().Accent).Background(a.exactColor("#26213F", "#e4e9df"))
							} else {
								b.AccessDisabled(true).Tooltip("模拟考试尚未启用；当前可使用练习模式")
							}
							b.Children(func() {
								ui.Column(c).Gap(4).Children(func() {
									ui.Text(c, title).FontSize(15).FontWeight(700)
									ui.Text(c, desc).FontSize(13).TextColor(c.Theme().TextMuted)
								})
							})
						}
					})
				})
			})
			ui.Column(c).Width(360).Shrink(0).Gap(16).Children(func() { a.drawSummary(c); a.randomHistory(c) })
		})
	})
}
func (a *App) drawSummary(c *ui.Context) {
	pool := a.M.DrawPool(time.Now())
	n, err := strconv.Atoi(a.M.Round.Settings.Count)
	valid := err == nil && n > 0 && n <= len(pool)
	a.boardCard(c).Padding(24).Gap(16).Children(func() {
		ui.Text(c, "本次将抽取").FontSize(13).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Textf(c, "%d", max(0, n)).FontSize(44).FontWeight(700)
			ui.Textf(c, "题 · 约 %d 分钟", max(0, n)*3/2).FontSize(16).Padding(0, 0, 7, 0).TextColor(c.Theme().TextMuted)
		})
		for _, subject := range a.subjectChoices() {
			if !slices.Contains(a.selectedDrawSubjects(), subject) {
				continue
			}
			count := 0
			for _, id := range pool {
				if a.M.Questions[a.M.Find(id)].Subject == subject {
					count++
				}
			}
			ui.Row(c).Gap(12).AlignItems(ui.Center).FontSize(13).TextColor(c.Theme().TextMuted).Children(func() {
				ui.Text(c, subject).Width(96)
				ui.Box(c).Grow(1).Height(6).Radius(99).Background(c.Theme().Border).Children(func() {
					if len(pool) > 0 {
						ui.Box(c).WidthPercent(float32(count) * 100 / float32(len(pool))).Height(6).Radius(99).Background(c.Theme().Accent)
					}
				})
				ui.Textf(c, "%d", count).Width(28).TextAlign(ui.End)
			})
		}
		ui.Textf(c, "可抽 %d 题 · 各科条形为可用题量", len(pool)).FontSize(12).TextColor(c.Theme().TextMuted)
		if a.boardAction(c, "开始随机练习", true).Height(52).FontSize(16).FillWidth().Disabled(!valid).Clicked() {
			a.collectSave(true)
			if err := a.M.Draw(n, time.Now()); err != nil {
				a.message = err.Error()
			} else {
				a.subject = "全部学科"
				a.chapter = "全部章节"
				a.studyDetail = true
				a.resetAnswer()
			}
		}
		if !valid {
			ui.Text(c, "请选择学科，并填写可抽范围内的题量。").FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if a.message != "" {
			ui.Text(c, a.message).FontSize(12).TextColor(c.Theme().Danger)
		}
	})
}
func (a *App) randomHistory(c *ui.Context) {
	a.boardCard(c).Padding(20, 24).Gap(8).Children(func() {
		ui.Text(c, "最近的随机练习").FontSize(14).FontWeight(700)
		if len(a.M.Round.RandomIDs) == 0 {
			ui.Text(c, "暂无随机练习记录").Padding(10, 0).FontSize(14).TextColor(c.Theme().TextMuted)
			return
		}
		done, right := 0, 0
		for _, id := range a.M.Round.RandomIDs {
			if r := a.M.Records[id]; r != nil && len(r.Attempts) > 0 {
				done++
				if r.Attempts[len(r.Attempts)-1]["correct"] == true {
					right++
				}
			}
		}
		b := ui.ButtonBase(c).Label("继续随机轮次").FillWidth().Padding(10, 0).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border)
		b.Children(func() {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).FontSize(14).Children(func() {
				ui.Text(c, "当前轮次")
				ui.Textf(c, "%d 题", len(a.M.Round.RandomIDs)).TextColor(c.Theme().TextMuted)
				if done > 0 {
					ui.Textf(c, "%.0f%%", float64(right)*100/float64(done)).FontWeight(700)
				} else {
					ui.Text(c, "未作答").TextColor(c.Theme().TextMuted)
				}
			})
		})
		if b.Clicked() {
			a.studyDetail = true
			a.subject = "全部学科"
			a.chapter = "全部章节"
			a.open(a.M.Round.RandomCurrentID)
		}
	})
}

func noteHasCause(text string) bool { return strings.Contains(text, "错因") }
func noteTags(text string) []string {
	var tags []string
	for _, tag := range []string{"概念不清", "计算失误", "粗心"} {
		if strings.Contains(text, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}
func (a *App) libraryNotesPage(c *ui.Context) {
	ids := a.M.IDs("全部学科", "notes", time.Now())
	cause, images := 0, 0
	for _, id := range ids {
		r := a.M.Records[id]
		if recordHasCause(r) {
			cause++
		}
		if recordHasImage(r) {
			images++
		}
	}
	a.boardTitle(c, "我的笔记", fmt.Sprintf("共 %d 条 · %d 条标了错因", len(ids), cause))
	var visible []string
	for _, id := range ids {
		q := a.M.Questions[a.M.Find(id)]
		r := a.M.Records[id]
		if a.subject != "全部学科" && q.Subject != a.subject {
			continue
		}
		if a.noteFilter == "cause" && !recordHasCause(r) || a.noteFilter == "images" && !recordHasImage(r) {
			continue
		}
		if term := strings.ToLower(strings.TrimSpace(a.noteSearch)); term != "" && !strings.Contains(strings.ToLower(q.Subject+q.Chapter+q.Title+r.Notes), term) {
			continue
		}
		visible = append(visible, id)
	}
	if !slices.Contains(visible, a.selectedNote) {
		a.selectedNote = ""
		a.noteEditing = false
		if len(visible) > 0 {
			a.selectedNote = visible[0]
		}
	}
	ui.Row(c).Grow(1).MinHeight(0).Gap(20).AlignItems(ui.Stretch).Children(func() {
		a.boardCard(c).Width(380).Shrink(0).Padding(16).Gap(8).Children(func() {
			ui.TextInput(c, &a.noteSearch).Search().Placeholder("搜索笔记").Label("搜索笔记").Height(44).FillWidth().Padding(0, 14).Radius(12).Background(c.Theme().Background).Border(1, a.exactColor("#363A48", "#c8cbbf")).FontSize(14)
			ui.Row(c).Gap(6).Padding(6, 0, 8, 0).Children(func() {
				for _, f := range []struct {
					key, label string
					count      int
				}{{"", "全部", len(ids)}, {"cause", "错因", cause}, {"images", "有图片", images}} {
					if a.boardChip(c, fmt.Sprintf("%s %d", f.label, f.count), a.noteFilter == f.key).Height(32).Padding(0, 12).FontSize(13).Clicked() {
						a.noteFilter = f.key
					}
				}
			})
			ui.Scroll(c.Key("note-list")).Grow(1).MinHeight(0).Gap(8).Children(func() {
				if len(visible) == 0 {
					ui.Text(c, "暂无匹配的笔记").Padding(14, 16).TextColor(c.Theme().TextMuted)
				}
				for _, id := range visible {
					q := a.M.Questions[a.M.Find(id)]
					r := a.M.Records[id]
					b := ui.ButtonBase(c.Key(id)).Label("笔记 "+id).FillWidth().Padding(14, 16).Radius(14).Border(1, ui.Transparent)
					if id == a.selectedNote {
						b.Background(a.exactColor("#26213F", "#e4e9df")).Border(1, a.exactColor("#4B4470", "#b7c4b0"))
					}
					b.Children(func() {
						ui.Column(c).Gap(6).Children(func() {
							ui.Text(c, questionMetaLabel(q)).FontSize(13).TextColor(c.Theme().TextMuted)
							preview := strings.ReplaceAll(r.Notes, "**", "")
							if strings.TrimSpace(preview) == "" && len(r.InkClips) > 0 {
								preview = fmt.Sprintf("草稿片段 · %d 张", len(r.InkClips))
							}
							ui.Text(c, preview).FontSize(15).LineHeight(1.55).MaxLines(3)
							tags := recordNoteTags(r)
							if recordHasImage(r) {
								tags = append(tags, "有图片")
							}
							if len(tags) > 0 {
								ui.Row(c).Gap(6).Wrap().Children(func() {
									for _, tag := range tags {
										ui.Text(c, tag).FontSize(12).FontWeight(500).Padding(3, 8).Radius(7).Background(c.Theme().Background).Border(1, a.exactColor("#363A48", "#c8cbbf")).TextColor(c.Theme().TextMuted)
									}
								})
							}
						})
					})
					if b.Clicked() {
						a.selectedNote = id
						a.noteEditing = false
					}
				}
			})
		})
		a.boardCard(c).Grow(1).MinWidth(0).Padding(28, 32).Gap(20).Children(func() {
			if a.selectedNote == "" {
				ui.Text(c, "选择一条笔记").FontSize(18).FontWeight(700)
				ui.Text(c, "在题目中打开“本题笔记”即可记录，保存后会显示在这里。").FontSize(15).TextColor(c.Theme().TextMuted)
				return
			}
			q := a.M.Questions[a.M.Find(a.selectedNote)]
			r := a.M.Records[q.ID]
			ui.Text(c, strings.ReplaceAll(questionMetaLabel(q), " · ", " / ")).FontSize(14).TextColor(c.Theme().TextMuted)
			ui.Scroll(c.Key("note-detail-" + q.ID)).Grow(1).MinHeight(0).Gap(20).Children(func() {
				ui.Column(c).Padding(16, 18).Radius(14).Border(1, c.Theme().Border).Background(c.Theme().Background).Gap(8).Children(func() {
					a.Rich.Stem(c.Key("note-source"), q.Text, 15)
					if a.exactButton(c, "查看原题").Padding(0).TextColor(a.exactColor("#C4B5FD", "#465b48")).Clicked() {
						a.mode("practice")
						a.subject = q.Subject
						a.chapter = q.Chapter
						a.open(q.ID)
					}
				})
				tags := recordNoteTags(r)
				if len(tags) > 0 {
					ui.Row(c).Gap(8).Children(func() {
						for _, tag := range tags {
							ui.Text(c, tag).FontSize(13).FontWeight(500).Padding(5, 10).Radius(8).Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48")).Border(1, c.Theme().Accent)
						}
					})
				}
				if a.noteEditing {
					a.noteEditor(c, "library-editor-"+q.ID, q.ID, "编辑笔记内容").MinHeight(220).Padding(14).FontSize(17).Background(c.Theme().Background).Border(1, c.Theme().Border).Radius(12)
					ui.Text(c, "Markdown · ⌘V 粘贴图片").FontSize(12).TextColor(c.Theme().TextMuted)
				} else {
					a.Rich.View(c.Key("note-body"), r.Notes, 17)
				}
				for _, clip := range r.InkClips {
					a.inkClipView(c, clip, 150)
				}
				if len(r.Strokes) > 0 {
					ui.Box(c).FillWidth().MaxWidth(340).Height(180).Radius(12).Clip().Padding(14).Background(c.Theme().Background).Border(1, c.Theme().Border).Draw(func(p *ui.Painter, rect ui.Rect) {
						a.drawScratch(p, ui.Rect{X: rect.X + 14, Y: rect.Y + 14, W: rect.W - 28, H: rect.H - 28}, r)
					})
					ui.Text(c, "来自草稿").FontSize(12).TextColor(a.exactColor("#8D92A3", "#777b72"))
				}
			})
			ui.Row(c).Gap(12).Children(func() {
				if a.boardAction(c, "回到这题复习", true).Clicked() {
					a.mode("wrong")
					a.M.Round.WrongScope = "all"
					a.subject = q.Subject
					a.chapter = q.Chapter
					a.open(q.ID)
					if !r.Wrong {
						a.M.Round.Mode = "practice"
						a.save()
					}
				}
				label := "编辑"
				if a.noteEditing {
					label = "完成"
				}
				if a.boardAction(c, label, false).Padding(0, 20).Clicked() {
					a.noteEditing = !a.noteEditing
					a.save()
				}
			})
		})
	})
}

func recordHasCause(r *Record) bool { return noteHasCause(r.Notes) || len(r.NoteTags) > 0 }
func recordHasImage(r *Record) bool { return len(r.Strokes) > 0 || len(r.InkClips) > 0 }
func recordNoteTags(r *Record) []string {
	tags := slices.Clone(r.NoteTags)
	for _, tag := range noteTags(r.Notes) {
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}
