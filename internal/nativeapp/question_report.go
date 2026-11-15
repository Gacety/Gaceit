package nativeapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	questionReportsKey      = "408-question-reports-v1"
	questionReportsFilename = "question-reports.json"
)

var defaultReportCategories = []string{
	"题干有误",
	"选项/配图有误",
	"答案有误",
	"解析有误",
	"评分细则有误",
}

// QuestionReport records user-reported issues on a specific question so users
// or external AI tools can inspect and patch the question bank accurately.
type QuestionReport struct {
	QuestionID      string   `json:"questionId"`
	Categories      []string `json:"categories"`
	Comment         string   `json:"comment"`
	SuggestedAnswer string   `json:"suggestedAnswer,omitempty"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// QuestionReportSnapshot captures the current source fields of a reported
// question so external AI tools can read the report standalone without reloading
// or parsing the full bank.
type QuestionReportSnapshot struct {
	ID            string   `json:"id"`
	Subject       string   `json:"subject"`
	Chapter       string   `json:"chapter"`
	Title         string   `json:"title"`
	Kind          string   `json:"kind,omitempty"`
	KindLabel     string   `json:"kindLabel,omitempty"`
	Text          string   `json:"text"`
	Options       []string `json:"options,omitempty"`
	Answer        string   `json:"answer"`
	AnswerChoices []string `json:"answerChoices,omitempty"`
	Rubric        string   `json:"rubric,omitempty"`
	Explanation   string   `json:"explanation,omitempty"`
	Errata        string   `json:"errata,omitempty"`
}

type QuestionReportEntry struct {
	QuestionReport
	Question *QuestionReportSnapshot `json:"question,omitempty"`
}

type QuestionReportArchive struct {
	Format      string                `json:"format"`
	Version     int                   `json:"version"`
	GeneratedAt string                `json:"generatedAt"`
	BankTitle   string                `json:"bankTitle,omitempty"`
	Total       int                   `json:"total"`
	OpenCount   int                   `json:"openCount"`
	Reports     []QuestionReportEntry `json:"reports"`
}

func cloneQuestionReports(src map[string]QuestionReport) map[string]QuestionReport {
	out := maps.Clone(src)
	for id, rep := range out {
		rep.Categories = slices.Clone(rep.Categories)
		out[id] = rep
	}
	return out
}

func snapshotForQuestion(q Question) *QuestionReportSnapshot {
	return &QuestionReportSnapshot{
		ID:            q.ID,
		Subject:       q.Subject,
		Chapter:       q.Chapter,
		Title:         q.Title,
		Kind:          q.Kind,
		KindLabel:     q.KindLabel,
		Text:          q.Text,
		Options:       slices.Clone(q.Options),
		Answer:        q.Answer,
		AnswerChoices: slices.Clone(q.AnswerChoices),
		Rubric:        q.Rubric,
		Explanation:   q.Explanation,
		Errata:        q.Errata,
	}
}

func (m *Model) QuestionReport(qID string) *QuestionReport {
	if m == nil || len(m.Reports) == 0 {
		return nil
	}
	rep, ok := m.Reports[qID]
	if !ok {
		return nil
	}
	cp := rep
	cp.Categories = slices.Clone(rep.Categories)
	return &cp
}

func normalizeReportCategories(categories []string) []string {
	var out []string
	for _, cat := range categories {
		trimmed := strings.TrimSpace(cat)
		if trimmed != "" && !slices.Contains(out, trimmed) {
			out = append(out, trimmed)
		}
	}
	return out
}

func (m *Model) SaveQuestionReport(qID string, categories []string, comment, suggestedAnswer, status string, now time.Time) error {
	if m.Find(qID) < 0 {
		return fmt.Errorf("未找到题目 %s", qID)
	}
	cats := normalizeReportCategories(categories)
	trimmedComment := strings.TrimSpace(comment)
	trimmedSuggested := strings.TrimSpace(suggestedAnswer)
	if trimmedComment == "" && trimmedSuggested == "" && len(cats) == 0 {
		return fmt.Errorf("请至少选择错误类型或填写报错说明")
	}
	if len(cats) == 0 {
		cats = []string{"题干有误"}
	}
	if status != "resolved" {
		status = "open"
	}
	if now.IsZero() {
		now = time.Now()
	}
	created := stamp(now)
	if prev, ok := m.Reports[qID]; ok && prev.CreatedAt != "" {
		created = prev.CreatedAt
	}
	if m.Reports == nil {
		m.Reports = map[string]QuestionReport{}
	}
	m.Reports[qID] = QuestionReport{
		QuestionID:      qID,
		Categories:      cats,
		Comment:         trimmedComment,
		SuggestedAnswer: trimmedSuggested,
		Status:          status,
		CreatedAt:       created,
		UpdatedAt:       stamp(now),
	}
	return m.Save()
}

func (m *Model) DeleteQuestionReport(qID string) error {
	if m.Reports == nil {
		return nil
	}
	if _, ok := m.Reports[qID]; !ok {
		return nil
	}
	delete(m.Reports, qID)
	return m.Save()
}

func (m *Model) BuildQuestionReportArchive(now time.Time) QuestionReportArchive {
	if now.IsZero() {
		now = time.Now()
	}
	entries := make([]QuestionReportEntry, 0, len(m.Reports))
	seen := make(map[string]bool, len(m.Reports))
	openCount := 0
	for _, q := range m.Questions {
		rep, ok := m.Reports[q.ID]
		if !ok {
			continue
		}
		seen[q.ID] = true
		rep.Categories = slices.Clone(rep.Categories)
		if rep.Status != "resolved" {
			openCount++
		}
		entries = append(entries, QuestionReportEntry{
			QuestionReport: rep,
			Question:       snapshotForQuestion(q),
		})
	}
	var extraIDs []string
	for id := range m.Reports {
		if !seen[id] {
			extraIDs = append(extraIDs, id)
		}
	}
	sort.Strings(extraIDs)
	for _, id := range extraIDs {
		rep := m.Reports[id]
		rep.Categories = slices.Clone(rep.Categories)
		if rep.Status != "resolved" {
			openCount++
		}
		entries = append(entries, QuestionReportEntry{
			QuestionReport: rep,
		})
	}
	return QuestionReportArchive{
		Format:      "408-question-reports",
		Version:     1,
		GeneratedAt: stamp(now),
		BankTitle:   m.BankTitle,
		Total:       len(entries),
		OpenCount:   openCount,
		Reports:     entries,
	}
}

func (m *Model) ExportQuestionReportsJSON() ([]byte, error) {
	archive := m.BuildQuestionReportArchive(time.Now())
	return json.MarshalIndent(archive, "", "  ")
}

func (m *Model) QuestionReportsFilePath() string {
	if m == nil || m.Disk == nil || strings.TrimSpace(m.Disk.Path) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(m.Disk.Path), questionReportsFilename)
}

func (m *Model) syncQuestionReportsFile() error {
	path := m.QuestionReportsFilePath()
	if path == "" {
		return nil
	}
	if len(m.Reports) == 0 {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}
	data, err := m.ExportQuestionReportsJSON()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".question-reports-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (m *Model) FormatQuestionReportPrompt(qID string, draft *QuestionReport) string {
	idx := m.Find(qID)
	if idx < 0 {
		return ""
	}
	q := m.Questions[idx]
	rep := m.QuestionReport(qID)
	if draft != nil {
		cp := *draft
		cp.Categories = normalizeReportCategories(draft.Categories)
		cp.Comment = strings.TrimSpace(draft.Comment)
		cp.SuggestedAnswer = strings.TrimSpace(draft.SuggestedAnswer)
		if cp.QuestionID == "" {
			cp.QuestionID = qID
		}
		if cp.Status == "" {
			cp.Status = "open"
		}
		rep = &cp
	}
	qJSON, _ := json.MarshalIndent(snapshotForQuestion(q), "", "  ")
	var b strings.Builder
	fmt.Fprintf(&b, "请根据我提交的题目报错信息，检查并给出修正后的题库 JSON（题目 ID: `%s`）。\n\n", q.ID)
	if path := m.QuestionReportsFilePath(); path != "" {
		fmt.Fprintf(&b, "完整报错存档路径：`%s`\n\n", path)
	}
	b.WriteString("## 我的报错信息\n")
	if rep != nil {
		if len(rep.Categories) > 0 {
			fmt.Fprintf(&b, "- 错误类型：%s\n", strings.Join(rep.Categories, "、"))
		}
		fmt.Fprintf(&b, "- 处理状态：%s\n", rep.Status)
		if rep.Comment != "" {
			fmt.Fprintf(&b, "- 报错说明：%s\n", rep.Comment)
		}
		if rep.SuggestedAnswer != "" {
			fmt.Fprintf(&b, "- 建议修正答案：%s\n", rep.SuggestedAnswer)
		}
	} else {
		b.WriteString("- 尚未填写报错详情，请核对题干、选项、答案与解析是否一致。\n")
	}
	b.WriteString("\n## 当前题目原始 JSON\n```json\n")
	b.Write(qJSON)
	b.WriteString("\n```\n\n请只修改有误字段并输出更新后的完整题目 JSON 对象。")
	return b.String()
}

func (a *App) loadReportDraft(qID string) {
	if rep := a.M.QuestionReport(qID); rep != nil {
		a.reportCategories = slices.Clone(rep.Categories)
		a.reportComment = rep.Comment
		a.reportSuggested = rep.SuggestedAnswer
		a.reportStatus = rep.Status
		if a.reportStatus != "resolved" {
			a.reportStatus = "open"
		}
		return
	}
	a.reportCategories = []string{"题干有误"}
	a.reportComment = ""
	a.reportSuggested = ""
	a.reportStatus = "open"
}

func (a *App) toggleReportPanel() {
	a.reportOpen = !a.reportOpen
	if a.reportOpen {
		a.loadReportDraft(a.M.Current().ID)
	}
}

func (a *App) toggleReportCategory(cat string) {
	if i := slices.Index(a.reportCategories, cat); i >= 0 {
		a.reportCategories = slices.Delete(a.reportCategories, i, i+1)
	} else {
		a.reportCategories = append(a.reportCategories, cat)
	}
}

func (a *App) reportButton(c *ui.Context, q *Question) {
	t := c.Theme()
	rep := a.M.QuestionReport(q.ID)
	text := "题目报错"
	if rep != nil {
		if rep.Status == "resolved" {
			text = "报错已处理"
		} else {
			text = "已报错"
		}
	}
	b := ui.ButtonBase(c.Key("question-report-btn")).
		Label("题目报错").
		Tooltip("标记题干/答案/解析错误，保存到独立报错存档并生成 AI 修题指令").
		Padding(a.qs(4), a.qs(10)).
		Radius(a.qs(8)).
		AlignItems(ui.Center).
		Justify(ui.Center).
		FontSize(a.qs(13)).
		FontWeight(500).
		Background(t.Surface).
		TextColor(t.TextMuted)
	if a.reportOpen || rep != nil {
		bg := t.Warning.Alpha(.14)
		fg := t.Warning
		if rep != nil && rep.Status == "resolved" && !a.reportOpen {
			bg = t.Success.Alpha(.14)
			fg = t.Success
		}
		b.Background(bg).TextColor(fg)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, text).FontSize(a.qs(13)).FontWeight(500).SingleLine()
	})
	if b.Clicked() {
		a.toggleReportPanel()
	}
}

func (a *App) copyReportPrompt(c *ui.Context, qID string, useDraft bool) {
	var draft *QuestionReport
	if useDraft {
		draft = &QuestionReport{
			QuestionID:      qID,
			Categories:      a.reportCategories,
			Comment:         a.reportComment,
			SuggestedAnswer: a.reportSuggested,
			Status:          a.reportStatus,
		}
	}
	prompt := a.M.FormatQuestionReportPrompt(qID, draft)
	if prompt != "" {
		c.WriteClipboard(prompt)
	}
}

func (a *App) questionReportSection(c *ui.Context, q *Question) {
	t := c.Theme()
	rep := a.M.QuestionReport(q.ID)
	if !a.reportOpen && rep == nil {
		return
	}
	if !a.reportOpen && rep != nil {
		ui.Row(c.Key("report-summary-"+q.ID)).FillWidth().Padding(a.qs(10), a.qs(14)).Gap(a.qs(8)).Radius(a.qs(12)).Background(t.Warning.Alpha(.08)).Border(a.qs(1), t.Warning.Alpha(.35)).AlignItems(ui.Center).Wrap().Children(func() {
			statusText := "待修正"
			statusColor := t.Warning
			if rep.Status == "resolved" {
				statusText = "已处理"
				statusColor = t.Success
			}
			ui.Textf(c, "报错[%s] · %s", statusText, strings.Join(rep.Categories, " / ")).FontSize(a.qs(13)).FontWeight(700).TextColor(statusColor)
			if rep.Comment != "" {
				ui.Text(c, rep.Comment).FontSize(a.qs(13)).TextColor(t.Text).Grow(1).MinWidth(0)
			} else {
				ui.Spacer(c)
			}
			if a.exactButton(c, "复制 AI 修题指令").Height(a.qs(28)).Padding(0, a.qs(10)).Radius(a.qs(8)).FontSize(a.qs(12)).Border(a.qs(1), t.Border).Background(t.Surface).TextColor(t.Text).Clicked() {
				a.copyReportPrompt(c, q.ID, false)
			}
			if a.exactButton(c, "编辑报错").Height(a.qs(28)).Padding(0, a.qs(10)).Radius(a.qs(8)).FontSize(a.qs(12)).Border(a.qs(1), t.Border).Background(t.Surface).TextColor(t.Text).Clicked() {
				a.toggleReportPanel()
			}
		})
		return
	}
	ui.Column(c.Key("report-editor-"+q.ID)).Label("题目报错面板").FillWidth().Padding(a.qs(16)).Gap(a.qs(10)).Radius(a.qs(14)).Background(t.Background).Border(a.qs(1), t.Warning.Alpha(.45)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(a.qs(8)).Children(func() {
			ui.Text(c, "题目报错（题干 / 答案 / 解析错误）").FontSize(a.qs(14)).FontWeight(700).TextColor(t.Warning).Grow(1)
			for _, st := range []struct{ key, label string }{{"open", "待修改"}, {"resolved", "已处理"}} {
				sb := a.exactButton(c, st.label).Height(a.qs(26)).Padding(0, a.qs(10)).Radius(a.qs(8)).FontSize(a.qs(12)).Border(a.qs(1), t.Border)
				if a.reportStatus == st.key || (st.key == "open" && a.reportStatus != "resolved") {
					sb.Background(a.exactColor("#2A2545", "#e4e9df")).TextColor(a.exactColor("#E4DAFF", "#465b48")).Border(a.qs(1), t.Accent)
				}
				if sb.Clicked() {
					a.reportStatus = st.key
				}
			}
		})
		ui.Row(c).Gap(a.qs(6)).Wrap().Children(func() {
			for _, cat := range defaultReportCategories {
				active := slices.Contains(a.reportCategories, cat)
				cb := a.exactButton(c, cat).Height(a.qs(28)).Padding(0, a.qs(10)).Radius(a.qs(8)).FontSize(a.qs(12)).Border(a.qs(1), t.Border)
				if active {
					cb.Background(t.Warning.Alpha(.16)).TextColor(t.Warning).Border(a.qs(1), t.Warning.Alpha(.5))
				}
				if cb.Clicked() {
					a.toggleReportCategory(cat)
				}
			}
		})
		ui.TextArea(c.Key("report-comment-"+q.ID), &a.reportComment).
			Label("报错详情输入").
			Placeholder("描述哪一步有误（例如题干条件缺失、参考答案错误、公式写错或解析推导有误）…").
			FillWidth().
			MinHeight(a.qs(88)).
			Padding(a.qs(12)).
			FontSize(a.qs(14)).
			Radius(a.qs(10)).
			Background(t.Surface).
			Border(a.qs(1), t.Border)
		ui.TextInput(c.Key("report-suggested-"+q.ID), &a.reportSuggested).
			Label("建议修正答案").
			Placeholder("可选：填写建议修改后的答案或关键修正点…").
			FillWidth()
		ui.Row(c).Gap(a.qs(8)).Wrap().AlignItems(ui.Center).Children(func() {
			if a.exactButton(c, "保存报错").Height(a.qs(34)).Padding(0, a.qs(14)).Radius(a.qs(8)).Background(t.Accent).TextColor(t.AccentText).FontSize(a.qs(13)).FontWeight(700).Clicked() {
				a.collectSave(true)
				a.savePending = false
				if err := a.M.SaveQuestionReport(q.ID, a.reportCategories, a.reportComment, a.reportSuggested, a.reportStatus, time.Now()); err != nil {
					a.message = err.Error()
				} else {
					a.message = ""
					a.reportOpen = false
				}
			}
			if a.exactButton(c, "复制 AI 修题指令").Height(a.qs(34)).Padding(0, a.qs(12)).Radius(a.qs(8)).Border(a.qs(1), t.Border).Background(t.Surface).TextColor(t.Text).FontSize(a.qs(13)).Clicked() {
				a.copyReportPrompt(c, q.ID, true)
			}
			if rep != nil {
				if a.exactButton(c, "删除报错").Height(a.qs(34)).Padding(0, a.qs(12)).Radius(a.qs(8)).Border(a.qs(1), t.Danger.Alpha(.4)).TextColor(t.Danger).FontSize(a.qs(13)).Clicked() {
					a.collectSave(true)
					a.savePending = false
					if err := a.M.DeleteQuestionReport(q.ID); err != nil {
						a.message = err.Error()
					} else {
						a.message = ""
						a.reportOpen = false
						a.loadReportDraft(q.ID)
					}
				}
			}
			ui.Spacer(c)
			if a.exactButton(c, "收起").Height(a.qs(34)).Padding(0, a.qs(12)).Radius(a.qs(8)).Border(a.qs(1), t.Border).FontSize(a.qs(13)).Clicked() {
				a.reportOpen = false
			}
		})
		if archivePath := a.M.QuestionReportsFilePath(); archivePath != "" {
			ui.Textf(c, "AI 报错存档：%s（命令行可执行 -list-reports 读取）", archivePath).FontSize(a.qs(11)).TextColor(t.TextMuted)
		}
	})
}
