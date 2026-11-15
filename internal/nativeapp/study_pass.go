package nativeapp

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

const studyPassKey = "408-study-passes-v1"

type StudyPass struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CreatedAt  string   `json:"createdAt"`
	IDs        []string `json:"ids"`
	PriorWrong []string `json:"priorWrong"`
	members    map[string]bool
}
type PassWorkspace struct {
	PracticeID      string   `json:"practiceId"`
	RandomIDs       []string `json:"randomIds"`
	RandomCurrentID string   `json:"randomCurrentId"`
	BatchID         string   `json:"batchId"`
}
type StudyPasses struct {
	Version    int                      `json:"version"`
	Active     string                   `json:"active"`
	Passes     []StudyPass              `json:"passes"`
	Workspaces map[string]PassWorkspace `json:"workspaces"`
}

func (m *Model) Pass(id string) *StudyPass {
	for i := range m.Study.Passes {
		if m.Study.Passes[i].ID == id {
			return &m.Study.Passes[i]
		}
	}
	return nil
}
func (m *Model) ActivePass() *StudyPass { return m.Pass(m.Study.Active) }
func (m *Model) inPass(id string) bool  { p := m.ActivePass(); return p == nil || p.members[id] }
func (m *Model) restorePasses() error {
	m.Study = StudyPasses{Version: 1, Workspaces: map[string]PassWorkspace{}}
	raw := m.snapshot[studyPassKey]
	if raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &m.Study); err != nil {
		return fmt.Errorf("刷题轮次记录损坏：%w", err)
	}
	if m.Study.Version != 1 || m.Study.Workspaces == nil {
		return fmt.Errorf("刷题轮次版本或工作区无效")
	}
	seen := map[string]bool{}
	for i := range m.Study.Passes {
		p := &m.Study.Passes[i]
		if p.ID == "" || seen[p.ID] || strings.TrimSpace(p.Name) == "" || len(p.IDs) == 0 {
			return fmt.Errorf("刷题轮次内容无效")
		}
		if _, err := time.Parse(time.RFC3339Nano, p.CreatedAt); err != nil {
			return fmt.Errorf("刷题轮次日期无效")
		}
		seen[p.ID] = true
		p.members = map[string]bool{}
		for _, id := range p.IDs {
			if id == "" || p.members[id] {
				return fmt.Errorf("刷题轮次题目重复或为空")
			}
			p.members[id] = true
		}
	}
	if m.Study.Active != "" && m.ActivePass() == nil {
		return fmt.Errorf("当前刷题轮次不存在")
	}
	return nil
}

// Merge known fields into the old JSON; future metadata and unknown source IDs
// survive saves and portable backups. No existing attempt is relabelled.
func mergePassJSON(old json.RawMessage, value any) json.RawMessage {
	out := map[string]json.RawMessage{}
	_ = json.Unmarshal(old, &out)
	if out == nil {
		out = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(value)
	fields := map[string]json.RawMessage{}
	_ = json.Unmarshal(b, &fields)
	for k, v := range fields {
		out[k] = v
	}
	b, _ = json.Marshal(out)
	return b
}
func (m *Model) savePasses() {
	var old map[string]json.RawMessage
	_ = json.Unmarshal([]byte(m.snapshot[studyPassKey]), &old)
	var prior []json.RawMessage
	_ = json.Unmarshal(old["passes"], &prior)
	byID := map[string]json.RawMessage{}
	for _, raw := range prior {
		var p StudyPass
		_ = json.Unmarshal(raw, &p)
		byID[p.ID] = raw
	}
	items := make([]json.RawMessage, 0, len(m.Study.Passes))
	for _, p := range m.Study.Passes {
		items = append(items, mergePassJSON(byID[p.ID], p))
	}
	work := map[string]json.RawMessage{}
	_ = json.Unmarshal(old["workspaces"], &work)
	if work == nil {
		work = map[string]json.RawMessage{}
	}
	for id, w := range m.Study.Workspaces {
		work[id] = mergePassJSON(work[id], w)
	}
	m.snapshot[studyPassKey] = string(mergePassJSON([]byte(m.snapshot[studyPassKey]), map[string]any{"version": 1, "active": m.Study.Active, "passes": items, "workspaces": work}))
}
func (m *Model) capturePassWorkspace() {
	if m.Study.Workspaces == nil {
		m.Study.Workspaces = map[string]PassWorkspace{}
	}
	w := m.Study.Workspaces[m.Study.Active]
	if m.Round.Mode == "practice" {
		w.PracticeID = m.Round.CurrentID
	}
	w.RandomIDs = slices.Clone(m.Round.RandomIDs)
	w.RandomCurrentID = m.Round.RandomCurrentID
	w.BatchID = m.Round.BatchID
	m.Study.Workspaces[m.Study.Active] = w
}
func (m *Model) SelectPass(id string) error {
	if id != "" && m.Pass(id) == nil {
		return fmt.Errorf("刷题轮次不存在")
	}
	if id == m.Study.Active {
		return nil
	}
	m.capturePassWorkspace()
	m.Study.Active = id
	w := m.Study.Workspaces[id]
	m.Round.RandomIDs = slices.Clone(w.RandomIDs)
	m.Round.RandomCurrentID = w.RandomCurrentID
	m.Round.BatchID = w.BatchID
	m.Round.RandomPassID = id
	m.Round.ReviewDeck = nil
	m.Round.WrongIDs = nil
	// Pass filters are pass-relative; switching rounds must not keep the old
	// round's 本轮新增/再次做错 selection applied to a different round.
	m.Round.PassFilter = ""
	m.Round.CurrentID = w.PracticeID
	if m.Round.Mode == "random" {
		m.Round.CurrentID = w.RandomCurrentID
	}
	if m.Find(m.Round.CurrentID) < 0 || !m.inPass(m.Round.CurrentID) {
		for _, q := range m.Questions {
			if m.inPass(q.ID) {
				m.Round.CurrentID = q.ID
				break
			}
		}
	}
	m.Before = nil
	if id != "" {
		m.Round.Settings.Range = "pass-unfinished"
	} else if strings.HasPrefix(m.Round.Settings.Range, "pass-") {
		m.Round.Settings.Range = "all"
	}
	return m.Save()
}
func (m *Model) CreatePass(name, subject, chapter string, now time.Time) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 40 {
		return fmt.Errorf("请输入 1～40 字的轮次名称")
	}
	if name == "历史练习" {
		return fmt.Errorf("请使用其他轮次名称")
	}
	for _, p := range m.Study.Passes {
		if p.Name == name {
			return fmt.Errorf("已有同名轮次，请换一个名称")
		}
	}
	p := StudyPass{ID: fmt.Sprintf("pass-%d", now.UnixNano()), Name: name, CreatedAt: stamp(now), members: map[string]bool{}}
	if m.Pass(p.ID) != nil {
		return fmt.Errorf("请稍后重试创建轮次")
	}
	for _, q := range m.Questions {
		if subject != "全部学科" && q.Subject != subject || chapter != "全部章节" && q.Chapter != chapter {
			continue
		}
		p.IDs = append(p.IDs, q.ID)
		p.members[q.ID] = true
		wrong := m.Records[q.ID].Wrong || q.History.InWrongBook
		for _, a := range m.Records[q.ID].Attempts {
			if a["outcome"] != "right" && a["correct"] == false {
				wrong = true
			}
		}
		if wrong {
			p.PriorWrong = append(p.PriorWrong, q.ID)
		}
	}
	if len(p.IDs) == 0 {
		return fmt.Errorf("选定范围没有题目")
	}
	m.Study.Passes = append(m.Study.Passes, p)
	return m.SelectPass(p.ID)
}
func (m *Model) passAttempts(id, passID string) (first, last map[string]any) {
	r := m.Records[id]
	if r == nil {
		return nil, nil
	}
	for _, a := range r.Attempts {
		if a["passId"] == passID && passID != "" {
			if first == nil {
				first = a
			}
			last = a
		}
	}
	return
}
func disputedAttempt(q Question, a map[string]any) bool {
	return strings.Contains(q.Errata, "标答存疑") || a["disputed"] == true
}
func (m *Model) passMatches(id, filter string) bool {
	p := m.ActivePass()
	if p == nil {
		return filter == "all"
	}
	if !p.members[id] {
		return false
	}
	first, last := m.passAttempts(id, p.ID)
	bad := func(a map[string]any) bool {
		return a != nil && !disputedAttempt(m.Questions[m.Find(id)], a) && a["outcome"] != "right"
	}
	switch filter {
	case "pass-unfinished":
		return first == nil
	case "pass-first-wrong":
		return first != nil && !disputedAttempt(m.Questions[m.Find(id)], first) && (first["outcome"] == "wrong" || first["outcome"] == "unknown")
	case "pass-new", "pass-again":
		anyBad := false
		for _, a := range m.Records[id].Attempts {
			if a["passId"] == p.ID && bad(a) {
				anyBad = true
			}
		}
		return anyBad && (slices.Contains(p.PriorWrong, id) == (filter == "pass-again"))
	case "pass-improved":
		return slices.Contains(p.PriorWrong, id) && last != nil && !disputedAttempt(m.Questions[m.Find(id)], last) && last["outcome"] == "right"
	}
	return true
}

type PassStats struct{ Total, Done, Right, Scored, Uncertain, Wrong, Disputed int }

func (m *Model) PassStats(subject, chapter string) PassStats {
	return m.passStatsFor(m.ActivePass(), subject, chapter)
}
func (m *Model) passStatsFor(p *StudyPass, subject, chapter string) PassStats {
	s := PassStats{}
	if p == nil {
		return s
	}
	for _, q := range m.Questions {
		if !p.members[q.ID] || subject != "全部学科" && q.Subject != subject || chapter != "全部章节" && q.Chapter != chapter {
			continue
		}
		s.Total++
		first, _ := m.passAttempts(q.ID, p.ID)
		if first == nil {
			continue
		}
		s.Done++
		if disputedAttempt(q, first) {
			s.Disputed++
			continue
		}
		s.Scored++
		switch first["outcome"] {
		case "right":
			s.Right++
		case "uncertain":
			s.Uncertain++
		default:
			s.Wrong++
		}
	}
	return s
}
func (m *Model) attemptPassID() string {
	if m.ActivePass() == nil || !m.inPass(m.Round.CurrentID) {
		return ""
	}
	if m.Round.Mode == "practice" {
		return m.Study.Active
	}
	if m.Round.Mode == "random" && m.Round.RandomPassID == m.Study.Active {
		return m.Study.Active
	}
	return ""
}
