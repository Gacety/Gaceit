package nativeapp

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"study408/internal/inkcanvas"
	"study408/internal/store"
)

type App struct {
	canvasTextInteraction                                                        bool
	inkRecoveryPending                                                           bool
	cardList                                                                     ui.ListState
	canvasCardZoom                                                               float32
	notePreviews                                                                 map[string]bool
	noteImagePastes                                                              map[string]string
	noteImageCarets                                                              map[string]int
	noteImageError                                                               string
	canvasCardID                                                                 string
	canvasCardAnswered, canvasCardDetails                                        bool
	canvasCardHeight                                                             float32
	inkSamplingRestore                                                           func()
	inkSessions                                                                  map[string]inkcanvas.Document
	inkPasteSerial                                                               int
	saveWorker                                                                   chan saveResult
	inkHomePending                                                               bool
	inkEditor                                                                    *inkcanvas.Editor
	inkPreview                                                                   *inkcanvas.Editor
	inkQuestion                                                                  string
	inkDirty                                                                     bool
	inkChangedAt, canvasEntered                                                  time.Time
	canvasElapsed                                                                time.Duration
	canvasNotes, canvasProgress, canvasWrongOnly, canvasAllCards, canvasSettings bool
	inCanvasQuestion                                                             bool
	DisplayRatio                                                                 float32
	studyDetail                                                                  bool
	chapterSubject, noteSearch, noteFilter, selectedNote                         string
	noteEditing, randomCustom                                                    bool
	passStats, passCreate                                                        bool
	passSelect, passName, passSubject, passChapter                               string
	reviewScrollID                                                               string
	cardScroll                                                                   ui.ScrollState
	M                                                                            *Model
	Rich                                                                         *Rich
	Window                                                                       *mygo.Window
	subject, chapter, search, filter, tool, quality                              string
	directory, details                                                           bool
	choices                                                                      []string
	answered, correct, unknown                                                   bool
	message                                                                      string
	queue                                                                        []string
	penTool                                                                      string
	live                                                                         *Stroke
	undo                                                                         [][]Stroke
	canvasW, canvasH                                                             float32
	canvasScale                                                                  float32
	savePending                                                                  bool
	saveSince, saveDue                                                           time.Time
	canvasPaintedAt                                                              time.Time
	pressKey, pressQID, pressLetter                                              string
	pressBlank                                                                   int
	pressAt                                                                      time.Time
	pressX, pressY                                                               float32
	pressFired                                                                   bool
	suppressClickKey                                                             string
	subjectiveText                                                               string
	aiGrading, aiShowConfig                                                      bool
	aiGradeCh                                                                    chan aiGradeResponse
	lastGrade                                                                    *AIGradeResult
	reportOpen                                                                   bool
	reportCategories                                                             []string
	reportComment, reportSuggested, reportStatus                                 string
}

func (a *App) subjects() []string { return a.M.SubjectList() }
func (a *App) subjectChoices() []string {
	subs := a.subjects()
	if len(subs) <= 1 {
		return nil
	}
	return subs[1:]
}
func (a *App) bankDisplayTitle() string {
	if strings.TrimSpace(a.M.BankTitle) != "" {
		return a.M.BankTitle
	}
	return "408 研习"
}
func (a *App) bankSummaryText() string {
	if strings.TrimSpace(a.M.BankTitle) == "" && len(a.M.Questions) == 1576 {
		return "relax 1000题 · 27版\n1,576 题 · 30 条勘误"
	}
	errata := 0
	for _, q := range a.M.Questions {
		if q.Errata != "" {
			errata++
		}
	}
	return fmt.Sprintf("%s\n%d 题 · %d 条勘误", a.bankDisplayTitle(), len(a.M.Questions), errata)
}

const (
	// Keep input responsive: painting and typing update memory at once, while
	// the full snapshot reaches disk shortly after the user pauses.
	saveDelay    = 600 * time.Millisecond
	saveMaxDelay = 3 * time.Second
)

func New(assets fs.FS, disk *store.Store) (*App, error) {
	m, err := NewModel(assets, disk)
	if err != nil {
		return nil, err
	}
	a := &App{M: m, Rich: NewRich(assets), subject: "全部学科", chapter: "全部章节", filter: "全部记录", quality: "independent", penTool: "pen"}
	if m.Round.Mode == "wrong" && m.Round.ReviewDeck != nil {
		d := m.Round.ReviewDeck
		if slices.Contains(m.SubjectList(), d.Subject) {
			a.subject = d.Subject
		}
		if d.Chapter == "全部章节" || slices.ContainsFunc(m.Questions, func(q Question) bool {
			return q.Chapter == d.Chapter && (a.subject == "全部学科" || a.subject == q.Subject)
		}) {
			a.chapter = d.Chapter
		}
	}
	a.Rich.NoteImage = a.loadNoteImage
	a.resetAnswer()
	return a, nil
}
func (a *App) resetAnswer() {
	// Exclusions are aids for this visit only. Re-entering a question starts
	// with all options visible, including after loading an older saved record.
	if r := a.M.Records[a.M.Current().ID]; r != nil && len(r.Excluded) > 0 {
		r.Excluded = nil
		a.saveSoon()
	}
	a.choices = make([]string, len(a.M.Current().AnswerChoices))
	a.answered = false
	a.unknown = false
	a.quality = "independent"
	a.message = ""
	a.undo = nil
	a.live = nil
	a.cancelLongPress()
	a.suppressClickKey = ""
	a.subjectiveText = ""
	a.aiGrading = false
	a.aiGradeCh = nil
	a.lastGrade = nil
	a.reportOpen = false
	a.loadReportDraft(a.M.Current().ID)
}
func (a *App) save() {
	a.collectSave(true)
	a.savePending = false
	if err := a.M.Save(); err != nil {
		a.message = "保存失败，请导出备份：" + err.Error()
	}
}

// saveSoon coalesces edits into one snapshot write. The first pause writes
// after saveDelay; continuous input still commits at least every saveMaxDelay.
func (a *App) saveSoon() {
	now := time.Now()
	if !a.savePending {
		a.savePending = true
		a.saveSince = now
		a.saveDue = now.Add(saveDelay)
		return
	}
	next := now.Add(saveDelay)
	if next.Before(a.saveSince.Add(saveMaxDelay)) {
		a.saveDue = next
	}
}
func (a *App) flushSave() {
	a.collectSave(true)
	if a.savePending {
		a.save()
	}
}
func (a *App) scheduleSave(c *ui.Context) {
	if !a.savePending {
		return
	}
	wait := time.Until(a.saveDue)
	if wait < 50*time.Millisecond {
		wait = 50 * time.Millisecond
	}
	c.After(wait)
}
func (a *App) serviceSave(c *ui.Context) {
	a.collectSave(false)
	if a.saveWorker != nil {
		return
	}
	if !a.savePending {
		return
	}
	if time.Now().Before(a.saveDue) {
		return
	}
	a.startSave()
}

const longPressDelay = 450 * time.Millisecond

func optionExKey(blank int, letter string) string { return fmt.Sprintf("%d:%s", blank, letter) }

func optionNeedsMarkdown(s string) bool {
	return strings.Contains(s, `\(`) || strings.Contains(s, `\[`) || strings.Contains(s, "`") || strings.Contains(s, "**") || strings.Contains(s, "![")
}

func (a *App) optionExcluded(qID string, blank int, letter string) bool {
	r := a.M.Records[qID]
	return r != nil && slices.Contains(r.Excluded, optionExKey(blank, letter))
}

func (a *App) toggleOptionExclusion(qID string, blank int, letter string) {
	if a.answered {
		return
	}
	r := a.M.Records[qID]
	if r == nil {
		return
	}
	key := optionExKey(blank, letter)
	if i := slices.Index(r.Excluded, key); i >= 0 {
		r.Excluded = slices.Delete(r.Excluded, i, i+1)
	} else {
		if blank < len(a.choices) && a.choices[blank] == letter {
			a.choices[blank] = ""
		}
		r.Excluded = append(r.Excluded, key)
	}
	a.saveSoon()
}

func (a *App) cancelLongPress() {
	a.pressKey, a.pressQID, a.pressLetter = "", "", ""
	a.pressBlank, a.pressFired = 0, false
}

// Once a hold fires, its click remains suppressed until a NEW gesture starts.
// Neither pointer jitter nor extra render passes can turn it back into a tap.
func (a *App) optionInput(e ui.InputEvent, key, qID string, blank int, letter string) bool {
	if a.answered || a.inCanvasQuestion && a.inkEditor != nil && a.inkEditor.PanToolActive() {
		return false
	}
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 {
			return false
		}
		a.suppressClickKey = ""
		a.pressKey, a.pressQID, a.pressLetter = key, qID, letter
		a.pressBlank, a.pressAt, a.pressFired = blank, time.Now(), false
		a.pressX, a.pressY = e.X, e.Y
	case ui.InputPointerMove:
		if a.pressKey != key {
			return false
		}
		dx, dy := e.X-a.pressX, e.Y-a.pressY
		if !a.pressFired && dx*dx+dy*dy > 64 {
			a.cancelLongPress()
		}
	case ui.InputPointerUp:
		if a.pressKey != key {
			return false
		}
		// The release can arrive before the timer's scheduled build.
		if !e.Canceled && !a.pressFired && time.Since(a.pressAt) >= longPressDelay {
			a.fireOptionHold()
		}
		fired := a.pressFired
		a.cancelLongPress()
		return fired
	case ui.InputKeyDown:
		if e.Key == ui.KeySpace || e.Key == ui.KeyEnter {
			a.suppressClickKey = ""
		}
	case ui.InputScroll, ui.InputZoom:
		if a.pressKey != "" {
			a.cancelLongPress()
		}
	}
	return false
}

// An excluded option cannot become an answer through a shortcut or stale input.
func (a *App) reconcileExcludedChoices() {
	for blank, letter := range a.choices {
		if a.optionExcluded(a.M.Round.CurrentID, blank, letter) {
			a.choices[blank] = ""
		}
	}
}

func (a *App) longPressPoll(c *ui.Context) {
	if a.pressKey == "" || a.answered {
		return
	}
	if a.M.Round.CurrentID != a.pressQID {
		a.cancelLongPress()
		return
	}
	if a.pressFired {
		return
	}
	if wait := longPressDelay - time.Since(a.pressAt); wait > 0 {
		c.After(wait)
		return
	}
	a.fireOptionHold()
}

func (a *App) fireOptionHold() {
	a.pressFired = true
	a.suppressClickKey = a.pressKey
	a.toggleOptionExclusion(a.pressQID, a.pressBlank, a.pressLetter)
}

func (a *App) clickedOption(key string) bool {
	return a.suppressClickKey == key
}

func (a *App) open(id string) {
	a.flushInk()
	if a.inkDirty {
		return
	}
	if a.M.Find(id) < 0 {
		return
	}
	a.studyDetail = true
	a.M.Round.CurrentID = id
	if a.M.Round.Mode == "random" {
		a.M.Round.RandomCurrentID = id
	}
	a.resetAnswer()
	a.save()
}
func (a *App) mode(mode string) {
	a.closeCanvas()
	if a.inkDirty {
		return
	}
	a.studyDetail = false
	a.tool = ""
	if a.M.Round.Mode == mode {
		return
	}
	a.M.Round.Mode = mode
	a.M.Round.ReviewDeck = nil
	a.subject = "全部学科"
	a.chapter = "全部章节"
	a.directory = false
	a.search = ""
	a.filter = "全部记录"
	if mode == "wrong" {
		a.M.Round.WrongScope = "due"
		a.M.Round.WrongIDs = shuffled(a.M.IDs("全部学科", "due", time.Now()))
	}
	if mode == "random" && len(a.M.Round.RandomIDs) > 0 {
		a.M.Round.CurrentID = a.M.Round.RandomCurrentID
		if !slices.Contains(a.M.Round.RandomIDs, a.M.Round.CurrentID) {
			a.M.Round.CurrentID = a.M.Round.RandomIDs[0]
		}
	}
	a.resetAnswer()
	a.save()
}
func (a *App) submit(unknown bool) {
	if a.M.Current().IsSubjective() {
		if unknown {
			a.submitSubjectiveOutcome("unknown")
			return
		}
		if a.M.AIConfig.Configured() {
			a.startAIGrade()
			return
		}
		outcome := "right"
		if a.quality != "independent" {
			outcome = "uncertain"
		}
		a.submitSubjectiveOutcome(outcome)
		return
	}
	a.flushInk()
	if a.inkDirty {
		return
	}
	a.collectSave(true)
	correct, err := a.M.Submit(a.choices, a.quality, unknown, time.Now())
	if err != nil {
		a.message = err.Error()
		return
	}
	a.answered = true
	a.correct = correct
	a.unknown = unknown
	a.message = ""
}
func (a *App) navigate(delta int) {
	i := slices.Index(a.queue, a.M.Round.CurrentID)
	if i+delta >= 0 && i+delta < len(a.queue) {
		a.open(a.queue[i+delta])
	}
}
func (a *App) dueCount() int { return len(a.M.IDs("全部学科", "due", time.Now())) }
func (a *App) theme() *ui.Theme {
	t := *ui.LightTheme()
	t.Background = ui.Hex("#fbfaf7")
	t.Surface = ui.Hex("#f0efea")
	t.SurfaceHover = ui.Hex("#e8e7e0")
	t.Border = ui.Hex("#deddd5")
	t.Text = ui.Hex("#30352f")
	t.TextMuted = ui.Hex("#6d7369")
	t.Accent = ui.Hex("#4f6653")
	t.AccentText = ui.Hex("#ffffff")
	t.Success = ui.Hex("#39704e")
	t.Danger = ui.Hex("#aa4a3b")
	if a.M.Theme == "dark" {
		t = *ui.DarkTheme()
		t.Background = ui.Hex("#282a36")
		t.Surface = ui.Hex("#21222c")
		t.SurfaceHover = ui.Hex("#343746")
		t.Border = ui.Hex("#44475a")
		t.Text = ui.Hex("#f8f8f2")
		t.TextMuted = ui.Hex("#a5adce")
		t.Accent = ui.Hex("#bd93f9")
		t.AccentText = ui.Hex("#282a36")
		t.Success = ui.Hex("#50fa7b")
		t.Danger = ui.Hex("#ff5555")
	}
	t.InactiveAccent = t.Accent
	t.AccentHover = t.Accent
	t.AccentPressed = t.Accent
	t.Selection = t.Accent.Alpha(.2)
	t.Focus = t.Accent
	t.Font = "Study Noto"
	t.FontSize = 13
	t.Radius = 6
	t.Spacing = 4
	return &t
}
func quiet(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).Background(ui.Transparent).Border(0, ui.Transparent).Padding(8, 10).Radius(5)
}
func (a *App) View(c *ui.Context) {
	c.SetTheme(a.theme())
	a.serviceSave(c)
	a.longPressPoll(c)
	a.reconcileExcludedChoices()
	a.collectAIGrade(c)
	defer a.scheduleSave(c)
	if a.tool != "scratch" {
		c.After(30 * time.Second)
	}
	now := time.Now()
	a.queue = a.M.Queue(a.subject, a.chapter, now)
	if a.answered && a.M.Round.Mode == "wrong" && !slices.Contains(a.queue, a.M.Round.CurrentID) {
		a.queue = append(a.queue, a.M.Round.CurrentID)
	}
	if a.M.Round.Mode == "wrong" {
		a.queue = a.reviewCardQueue(now, a.queue)
	}
	if a.M.Round.Mode != "notes" && len(a.queue) > 0 && !slices.Contains(a.queue, a.M.Round.CurrentID) && !a.answered {
		a.M.Round.CurrentID = a.queue[0]
		a.resetAnswer()
	}
	if (a.M.Round.Mode == "wrong" || a.studyDetail) && a.M.Round.Mode != "notes" && a.tool != "notes" && !(a.tool == "scratch" && a.canvasNotes) && !a.reportOpen && len(a.queue) > 0 && !a.answered && !a.M.Current().IsSubjective() {
		if len(a.choices) == 1 {
			for i := 0; i < 4; i++ {
				if c.Shortcut(0, ui.Key1+ui.Key(i)) {
					letter := string(rune('A' + i))
					if !a.optionExcluded(a.M.Round.CurrentID, 0, letter) {
						a.choices[0] = letter
					}
				}
			}
		}
		if c.Shortcut(0, ui.KeyEnter) {
			a.submit(false)
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		a.closeCanvas()
		if !a.inkDirty {
			a.tool = ""
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		a.flushInk()
		a.save()
	}
	if a.M.Round.Mode == "notes" || ((a.M.Round.Mode == "practice" || a.M.Round.Mode == "random") && !a.studyDetail) {
		a.libraryPage(c)
		return
	}
	if a.tool == "scratch" {
		a.canvasPage(c)
		return
	}
	a.reviewPage(c)
}
func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(func() float32 {
		if a.M.Round.Mode == "wrong" {
			return 240
		}
		return 206
	}()).Shrink(0).FillHeight().Padding(26, 16).Gap(8).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(10).Padding(6, 10, 26, 10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["book"]).Size(23, 23).TextColor(t.Accent)
			ui.Text(c, a.bankDisplayTitle()).FontSize(20)
		})
		for _, item := range []struct{ key, label, icon string }{{"practice", "章节练习", "book"}, {"wrong", "错题复习", "clock"}, {"random", "随机练习", "shuffle"}, {"notes", "我的笔记", "note"}} {
			selected := a.M.Round.Mode == item.key
			b := ui.ButtonBase(c.Key("nav-"+item.key)).FillWidth().Padding(10, 12).Gap(10).Radius(6).Label(item.label)
			if selected {
				b.Background(t.Accent.Alpha(.12))
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.Children(func() {
				ui.Icon(c, icons[item.icon]).Size(17, 17).TextColor(t.TextMuted)
				ui.Text(c, item.label).Grow(1)
				if item.key == "wrong" {
					ui.Textf(c, "%d", a.dueCount()).FontSize(11).TextColor(t.TextMuted)
				}
			})
			if b.Clicked() {
				a.mode(item.key)
			}
		}
		ui.Text(c, "题库").FontSize(11).TextColor(t.TextMuted).Padding(24, 12, 8, 12)
		for _, subject := range a.subjects() {
			if quiet(c, subject).FillWidth().Justify(ui.Start).TextAlign(ui.Start).TextColor(func() ui.Color {
				if a.subject == subject {
					return t.Accent
				}
				return t.TextMuted
			}()).Clicked() {
				a.subject = subject
				a.chapter = "全部章节"
				if a.M.Round.Mode == "notes" {
					a.mode("practice")
					a.subject = subject
				}
				a.resetAnswer()
			}
		}
		ui.Spacer(c)
		summaryLines := strings.SplitN(a.bankSummaryText(), "\n", 2)
		ui.Text(c, summaryLines[0]).FontSize(11).TextColor(t.TextMuted).Padding(4, 12)
		if len(summaryLines) > 1 {
			ui.Text(c, summaryLines[1]).FontSize(10).TextColor(t.TextMuted).Padding(2, 12, 18, 12)
		}
		themeLabel := "切换 Dracula"
		if a.M.Theme == "dark" {
			themeLabel = "切换浅色主题"
		}
		if quiet(c, themeLabel).FillWidth().TextAlign(ui.Start).Clicked() {
			if a.M.Theme == "dark" {
				a.M.Theme = "light"
			} else {
				a.M.Theme = "dark"
			}
			a.save()
		}
		ui.Row(c).Gap(4).Children(func() {
			if quiet(c, "导出备份").FontSize(11).Clicked() {
				a.exportBackup()
			}
			if quiet(c, "导入备份").FontSize(11).Clicked() {
				a.importBackup()
			}
		})
	})
}

// qs scales the question reader inside the infinite canvas by the ink camera.
// A MyGo transform would magnify a texture of the 100% card and blur its text;
// laying the same content out at the scaled sizes keeps glyphs vector-crisp.
func (a *App) qs(v float32) float32 {
	if a.inCanvasQuestion && a.canvasScale > 0 {
		return v * a.canvasScale
	}
	return v
}

func (a *App) question(c *ui.Context) {
	q := a.M.Current()
	r := a.M.Records[q.ID]
	t := c.Theme()
	a.reviewQuestionMeta(c)
	a.questionReportSection(c, q)
	if q.Errata != "" || len(q.OCRNotes) > 0 {
		if quiet(c, "查看勘误与录入备注").FontSize(a.qs(13)).Padding(a.qs(8), a.qs(10)).TextColor(t.Warning).Clicked() {
			a.details = !a.details
		}
		if a.details {
			a.Rich.View(c, q.Errata+"\n\n"+strings.Join(q.OCRNotes, "\n\n"), a.qs(13))
		}
	}
	a.Rich.Stem(c, q.Text, a.qs(21)).Label("题干正文").FontWeight(500)
	if q.IsSubjective() {
		a.subjectiveQuestionSection(c, q, r)
	} else {
		for blank := range q.AnswerChoices {
			if len(q.AnswerChoices) > 1 {
				ui.Textf(c, "第 %d 空", blank+1).FontSize(a.qs(12)).TextColor(t.TextMuted)
			}
			group := ui.RadioGroup(c.Key(fmt.Sprint("answers", blank)), func() {
				for index, option := range q.Options {
					letter := string(rune('A' + index))
					selected := a.choices[blank] == letter
					excluded := a.optionExcluded(q.ID, blank, letter)
					pressKey := fmt.Sprintf("%s|%d|%s", q.ID, blank, letter)
					row := ui.ButtonBase(c.Key(fmt.Sprintf("%d-%d", blank, index))).Role(ui.RoleRadio).AccessSelected(selected).Label(fmt.Sprintf("第%d空 选项%s", blank+1, letter)).Description(option).FillWidth().Padding(a.qs(12), a.qs(14)).Gap(a.qs(14)).Radius(a.qs(7)).Disabled(a.answered).AlignItems(ui.Start).HandleInput(func(e ui.InputEvent) bool { return a.optionInput(e, pressKey, q.ID, blank, letter) })
					color := t.TextMuted
					fill := ui.Transparent
					if a.M.Round.Mode != "notes" {
						fill = a.reviewSurface(c)
						row.Border(a.qs(1), t.Border).Radius(a.qs(14)).Padding(0, a.qs(20)).MinHeight(a.qs(64)).Gap(a.qs(16)).AlignItems(ui.Center).FontWeight(500)
						if a.inCanvasQuestion {
							row.Radius(a.qs(12)).Padding(0, a.qs(18)).MinHeight(a.qs(56))
						}
					}
					if selected {
						fill = t.Accent.Alpha(.1)
						color = t.Accent
					}
					if a.answered && q.AnswerChoices[blank] == letter {
						fill = t.Success.Alpha(.12)
						color = t.Success
					}
					if a.answered && selected && q.AnswerChoices[blank] != letter {
						fill = t.Danger.Alpha(.12)
						color = t.Danger
					}
					if row.Hovered() && !a.answered {
						fill = t.Surface
					}
					if excluded && !a.answered {
						fill = a.reviewSurface(c).Alpha(.45)
						color = t.TextMuted.Alpha(.6)
					}
					row.Background(fill)
					if selected {
						row.Border(a.qs(1), t.Accent)
						if !a.answered {
							row.Background(a.exactColor("#26213F", "#e4e9df"))
						}
					}
					if excluded && !a.answered {
						row.Border(a.qs(1), t.Border.Alpha(.35))
						row.Background(a.reviewSurface(c).Alpha(.45))
					}
					row.Children(func() {
						if a.M.Round.Mode != "notes" {
							markBG := t.Border
							markFG := t.TextMuted
							if selected {
								markBG = t.Accent
								markFG = t.AccentText
							}
							if excluded && !a.answered {
								markBG = t.Border.Alpha(.35)
								markFG = t.TextMuted.Alpha(.6)
							}
							ui.Row(c).Size(a.qs(func() float32 {
								if a.inCanvasQuestion {
									return 30
								}
								return 32
							}()), a.qs(func() float32 {
								if a.inCanvasQuestion {
									return 30
								}
								return 32
							}())).Shrink(0).Radius(a.qs(9)).Background(markBG).AlignItems(ui.Center).Justify(ui.Center).Children(func() { ui.Text(c, letter).FontSize(a.qs(14)).FontWeight(700).TextColor(markFG) })
						} else {
							ui.Text(c, letter).Width(a.qs(24)).Height(a.qs(26)).TextAlign(ui.Center).FontSize(a.qs(13)).TextColor(color).Border(a.qs(1), color.Alpha(.5)).Radius(a.qs(func() float32 {
								if a.M.Round.Mode != "notes" {
									return 8
								}
								return 13
							}()))
						}
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							if option == "" {
								option = "见原题图片中的 " + letter + " 选项"
							}
							size := float32(17)
							if a.inCanvasQuestion {
								size = 16
							}
							if excluded && !optionNeedsMarkdown(option) {
								// Plain options get a real per-line strikethrough.
								ui.Text(c, option).FontSize(a.qs(size)).LineHeight(1.75).FontWeight(500).FillWidth().MinWidth(0).Strikethrough().Opacity(.5)
							} else {
								text := a.Rich.View(c, option, a.qs(size)).IgnoreInput(true)
								if excluded {
									text = text.Opacity(.5)
									ui.Box(c).Absolute().Fill().ZIndex(1).Draw(func(p *ui.Painter, rect ui.Rect) {
										line := max(float32(1), a.qs(1.4))
										color := c.Theme().TextMuted.Alpha(.7)
										lh := a.qs(size) * 1.75
										if lh <= 0 {
											lh = rect.H
										}
										for y := rect.Y + lh/2; y <= rect.Y+rect.H; y += lh {
											p.Line(rect.X, y, rect.X+rect.W, y, line, color)
										}
									})
								}
							}
						})
					})
					if row.Clicked() && !a.answered && !excluded && !a.clickedOption(pressKey) {
						a.choices[blank] = letter
					}
				}
			})
			if a.M.Round.Mode != "notes" {
				group.Gap(a.qs(12))
			}
		}
	}
	if a.answered {
		ui.Divider(c)
		if q.IsSubjective() {
			a.subjectiveAnsweredSection(c, q, r)
		} else {
			text := "与表内答案一致"
			color := t.Success
			if !a.correct {
				text = "表内答案：" + q.Answer
				color = t.Danger
			}
			if a.unknown {
				text = "本次标记不会 · 表内答案：" + q.Answer
			}
			ui.Text(c, text).FontSize(a.qs(17)).TextColor(color)
		}
		if p := a.M.ActivePass(); p != nil && p.members[q.ID] {
			// First submission defines this round's result; later reinforcement
			// stays in history and never rewrites the first-answer statistic.
			if first, _ := a.M.passAttempts(q.ID, p.ID); first != nil {
				ui.Row(c).Gap(a.qs(16)).Wrap().Children(func() {
					ui.Textf(c, "本轮首次：%s", attemptOutcomeLabel(first)).FontSize(a.qs(12)).TextColor(t.TextMuted)
					if len(r.Attempts) > 1 {
						prev := r.Attempts[len(r.Attempts)-2]
						ui.Textf(c, "上一次作答：%s · 历史共 %d 次", attemptOutcomeLabel(prev), len(r.Attempts)).FontSize(a.qs(12)).TextColor(t.TextMuted)
					} else {
						ui.Textf(c, "历史共 %d 次作答", len(r.Attempts)).FontSize(a.qs(12)).TextColor(t.TextMuted)
					}
					if r.Wrong {
						ui.Text(c, "在错题本中 · 所有轮次共享").FontSize(a.qs(12)).TextColor(t.TextMuted)
					}
				})
			}
		}
		if r.Review != nil && r.Review.DueAt != nil {
			ui.Text(c, "下次复习："+parse(r.Review.DueAt).In(zone).Format("01月02日 15:04")).FontSize(a.qs(12)).TextColor(t.TextMuted)
		}
		a.Rich.View(c, q.Explanation, a.qs(16)).Label("本题解析")
		if q.Source.Workbook != "" {
			ui.Textf(c, "原表作答 %s · 原表结果 %s\n来源 %s · 第 %d 行；原表无日期，不计为本机新作答。", q.History.Chosen, q.History.Result, q.Source.Workbook, q.Source.Row).FontSize(a.qs(11)).TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(a.qs(12)).Children(func() {
			if quiet(c, "重新作答").FontSize(a.qs(13)).Padding(a.qs(8), a.qs(10)).Clicked() {
				a.resetAnswer()
			}
			if quiet(c, "撤销本次作答与安排").FontSize(a.qs(13)).Padding(a.qs(8), a.qs(10)).Disabled(a.M.Before == nil).Clicked() {
				a.collectSave(true)
				_ = a.M.Undo()
				a.resetAnswer()
			}
		})
	}
	if a.message != "" {
		ui.Text(c, a.message).TextColor(t.Danger).FontSize(a.qs(12))
	}
}
func (a *App) toolButton(c *ui.Context, key, label, icon string) {
	b := quiet(c, "").Label(label).Tooltip(label).Gap(6).Children(func() { ui.Icon(c, icons[icon]).Size(16, 16); ui.Text(c, label).FontSize(12) })
	if a.tool == key {
		b.Background(c.Theme().Accent.Alpha(.12)).TextColor(c.Theme().Accent)
	}
	if b.Clicked() {
		if a.tool == key {
			a.tool = ""
		} else {
			a.tool = key
		}
	}
}
func (a *App) footer(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Shrink(0).Padding(0, 36, 20, 36).Children(func() {
		ui.Divider(c)
		ui.Row(c).Gap(12).Padding(16, 0, 0, 0).AlignItems(ui.Center).Children(func() {
			i := slices.Index(a.queue, a.M.Round.CurrentID)
			if quiet(c, "上一题").Disabled(i <= 0).Clicked() {
				a.navigate(-1)
			}
			if a.M.Round.Mode == "wrong" {
				hint := "Enter 提交"
				if len(a.M.Current().AnswerChoices) == 1 {
					hint = "1–4 选择 · Enter 提交"
				}
				ui.Text(c, hint).FontSize(10).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			if quiet(c, "下一题").Disabled(i < 0 || i >= len(a.queue)-1).Clicked() {
				a.navigate(1)
			}
			label := "提交答案"
			if a.answered {
				label = "继续下一题"
			}
			if ui.PrimaryButton(c, label).Padding(11, 24).Disabled(len(a.queue) == 0).Clicked() {
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
		ui.Text(c, "本机作答与原表历史分别保留").FontSize(10).TextColor(t.TextMuted).Padding(10, 0, 0, 0)
	})
}
func (a *App) empty(c *ui.Context) {
	ui.Column(c).Padding(60, 20).Gap(14).Children(func() {
		title := "当前范围没有题目"
		detail := "调整学科或章节筛选后继续。"
		if a.M.Round.Mode == "wrong" {
			title = "当前没有到期题"
			detail = "可切换全部错题浏览；答对不会自动移出错题本。"
		}
		if a.M.Round.Mode == "random" {
			title = "抽一组题，开始这一轮"
			detail = "选择学科、范围与题量后，点击抽取。"
		}
		ui.Text(c, title).FontSize(24)
		ui.Text(c, detail).TextColor(c.Theme().TextMuted)
	})
}
func (a *App) drawHeader(c *ui.Context) {
	title := "随机练习 · 当前轮次"
	if p := a.M.ActivePass(); p != nil {
		title = "随机练习 · " + p.Name
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(26).Grow(1)
		if quiet(c, "调整范围").Clicked() {
			a.studyDetail = false
		}
	})
	ui.Textf(c, "本轮 %d 题 · 每轮无放回 · 顺序与进度自动保存", len(a.M.Round.RandomIDs)).FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Divider(c)
}
func (a *App) directoryView(c *ui.Context) {
	ui.Column(c).Gap(12).Padding(16).Background(c.Theme().Surface).Radius(8).Children(func() {
		ui.TextInput(c, &a.search).Search().Placeholder("搜索题干、题号或 slug").Label("搜索题目").FillWidth()
		chapters := []string{"全部章节"}
		for _, q := range a.M.Questions {
			if a.subject != "全部学科" && a.subject != q.Subject {
				continue
			}
			if !slices.Contains(chapters, q.Chapter) {
				chapters = append(chapters, q.Chapter)
			}
		}
		ui.Row(c).Gap(10).Children(func() {
			ui.Select(c, &a.chapter, chapters).Grow(1).Label("章节")
			ui.Select(c, &a.filter, []string{"全部记录", "含勘误", "当前错题本", "原表错误", "原表不会"}).Grow(1).Label("记录筛选")
		})
		ui.Scroll(c).Height(280).Children(func() {
			count := 0
			for _, q := range a.M.Questions {
				r := a.M.Records[q.ID]
				if a.subject != "全部学科" && q.Subject != a.subject || a.chapter != "全部章节" && q.Chapter != a.chapter {
					continue
				}
				if a.filter == "含勘误" && q.Errata == "" || a.filter == "当前错题本" && !r.Wrong || a.filter == "原表错误" && q.History.Result != "错误" || a.filter == "原表不会" && q.History.Result != "不会" {
					continue
				}
				if term := strings.ToLower(strings.TrimSpace(a.search)); term != "" && !strings.Contains(strings.ToLower(q.ID+q.Text+fmt.Sprintf(" p%d", q.Source.Page)), term) {
					continue
				}
				if quiet(c, q.Subject+" · "+q.Title+"   "+q.ID).FillWidth().TextAlign(ui.Start).Label("目录 " + q.ID).Clicked() {
					a.open(q.ID)
					a.directory = false
				}
				count++
				if count >= 80 {
					ui.Text(c, "显示前 80 题，请用搜索缩小范围。").TextColor(c.Theme().TextMuted)
					break
				}
			}
			if count == 0 {
				ui.Text(c, "没有匹配的题目")
			}
		})
	})
}
func (a *App) notesPage(c *ui.Context) {
	ui.Scroll(c).Grow(1).Padding(40).Gap(18).Children(func() {
		ui.Text(c, "我的笔记").FontSize(28)
		ids := a.M.IDs("全部学科", "notes", time.Now())
		if len(ids) == 0 {
			ui.Text(c, "打开题目标题右侧的“本题笔记”，记录思路与疑问。").TextColor(c.Theme().TextMuted)
		}
		for _, id := range ids {
			q := a.M.Questions[a.M.Find(id)]
			if quiet(c, q.Subject+" / "+q.Title).FillWidth().TextAlign(ui.Start).Clicked() {
				a.mode("practice")
				a.open(id)
				a.tool = "notes"
			}
			ui.Text(c, a.M.Records[id].Notes).FontSize(13).TextColor(c.Theme().TextMuted)
			ui.Divider(c)
		}
	})
}

// ShowQuestion preserves the command-line direct-question entry without
// changing progress merely to render or inspect a source question.
func (a *App) ShowQuestion(id string) {
	if a.M.Find(id) < 0 {
		return
	}
	a.studyDetail = true
	a.M.Round.CurrentID = id
	a.resetAnswer()
}
func (a *App) SetTool(tool string) {
	a.closeCanvas()
	if a.inkDirty {
		return
	}
	a.studyDetail = true
	if tool == "scratch" || tool == "notes" {
		a.tool = tool
	} else {
		a.tool = ""
	}
}
