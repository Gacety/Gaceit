package nativeapp

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"study408/internal/ink"
	"study408/internal/inkcanvas"
	"study408/internal/store"
)

const progressKey = "408-study-glass-v1"
const roundKey = "408-study-round-v1"

var Subjects = []string{"全部学科", "数据结构", "计算机组成原理", "操作系统", "计算机网络"}

type Question struct {
	ID, Subject, Chapter, Title, Kind, KindLabel, Text, Answer, Rubric, Explanation, Errata string
	Options, AnswerChoices, OCRNotes                                                        []string
	History                                                                                 struct {
		Chosen, Result string
		InWrongBook    bool
	}
	Source struct {
		Workbook          string
		Row, Page, Number int
	}
}

func (q Question) IsSubjective() bool {
	return strings.EqualFold(q.Kind, "subjective") || (len(q.Options) == 0 && len(q.AnswerChoices) == 0)
}

func deriveSubjects(questions []Question) []string {
	out := []string{"全部学科"}
	for _, q := range questions {
		s := strings.TrimSpace(q.Subject)
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	if len(questions) == 0 {
		return append([]string(nil), Subjects...)
	}
	return out
}

type Stroke struct {
	Tool, Color string       `json:",omitempty"`
	Points      [][2]float32 `json:"points"`
}

// Retain the browser's lower-case wire format for existing drawings.
func (s Stroke) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tool   string       `json:"tool"`
		Color  string       `json:"color"`
		Points [][2]float32 `json:"points"`
	}{s.Tool, s.Color, s.Points})
}

type InkClip struct {
	ID        string       `json:"id"`
	CreatedAt string       `json:"createdAt"`
	Strokes   []ink.Stroke `json:"strokes"`
	Bounds    [4]float32   `json:"bounds"`
}
type Record struct {
	NoteImages  map[string]string   `json:"noteImages,omitempty"`
	Ink         *inkcanvas.Document `json:"ink,omitempty"`
	InkClips    []InkClip           `json:"inkClips,omitempty"`
	NoteTags    []string            `json:"noteTags,omitempty"`
	NotePrivate bool                `json:"notePrivate,omitempty"`
	Excluded    []string            `json:"excluded,omitempty"`
	Notes       string              `json:"notes"`
	Strokes     []Stroke            `json:"strokes"`
	Attempts    []map[string]any    `json:"attempts"`
	Wrong       bool                `json:"wrong"`
	Review      *Review             `json:"review,omitempty"`
}
type DrawSettings struct {
	Subjects []string `json:"subjects"`
	Subject  string   `json:"subject"`
	Range    string   `json:"range"`
	Count    string   `json:"count"`
}
type ReviewDeck struct {
	IDs     []string `json:"ids"`
	Scope   string   `json:"scope"`
	Subject string   `json:"subject"`
	Chapter string   `json:"chapter"`
	Day     string   `json:"day"`
	Filter  string   `json:"filter,omitempty"`
}
type Round struct {
	RandomPassID    string       `json:"randomPassId"`
	BatchID         string       `json:"batchId"`
	PassFilter      string       `json:"passFilter"`
	ReviewDeck      *ReviewDeck  `json:"reviewDeck,omitempty"`
	Mode            string       `json:"mode"`
	CurrentID       string       `json:"currentId"`
	WrongIDs        []string     `json:"wrongIds"`
	WrongScope      string       `json:"wrongScope"`
	RandomIDs       []string     `json:"randomIds"`
	RandomCurrentID string       `json:"randomCurrentId"`
	Settings        DrawSettings `json:"settings"`
}
type Model struct {
	Study         StudyPasses
	Subjects      []string
	BankTitle     string
	ReviewConfig  ReviewConfig
	AIConfig      AIConfig
	Reports       map[string]QuestionReport
	index         map[string]int
	Questions     []Question
	Records       map[string]*Record
	rawRecords    map[string]map[string]json.RawMessage
	snapshot      store.Snapshot
	Disk          *store.Store
	Round         Round
	Theme         string
	SaveError     string
	Before        *Record
	LastAttemptID string
}

func (m *Model) SubjectList() []string {
	if len(m.Subjects) > 0 {
		return m.Subjects
	}
	return Subjects
}

func NewModel(assets fs.FS, disk *store.Store) (*Model, error) {
	data, err := fs.ReadFile(assets, "prototype/src/relax-data.json")
	if err != nil {
		data, err = fs.ReadFile(assets, "questions.json")
		if err != nil {
			return nil, err
		}
	}
	m := &Model{Disk: disk, Records: map[string]*Record{}, Reports: map[string]QuestionReport{}, rawRecords: map[string]map[string]json.RawMessage{}, snapshot: store.Snapshot{}, ReviewConfig: DefaultReviewConfig()}
	if err = json.Unmarshal(data, &m.Questions); err != nil {
		return nil, err
	}
	if len(m.Questions) == 0 {
		return nil, fmt.Errorf("题库文件不包含任何题目")
	}
	sanitizeInitialQuestions(m.Questions)
	m.Subjects = deriveSubjects(m.Questions)
	Subjects = slices.Clone(m.Subjects)
	if disk != nil {
		m.snapshot, err = disk.Load()
		if err != nil {
			return nil, err
		}
	}
	if err = m.restore(); err != nil {
		return nil, err
	}
	return m, nil
}
func (m *Model) restore() error {
	cfg := DefaultReviewConfig()
	if raw := m.snapshot[reviewConfigKey]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return fmt.Errorf("复习参数格式不正确，保留原文件：%w", err)
		}
	}
	m.ReviewConfig = SetReviewConfig(cfg)
	m.AIConfig = AIConfig{}
	if raw := m.snapshot[aiConfigKey]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &m.AIConfig); err != nil {
			return fmt.Errorf("AI 配置格式不正确，保留原文件：%w", err)
		}
	}
	m.Reports = map[string]QuestionReport{}
	if raw := m.snapshot[questionReportsKey]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &m.Reports); err != nil {
			return fmt.Errorf("题目报错存档格式不正确，保留原文件：%w", err)
		}
		if m.Reports == nil {
			m.Reports = map[string]QuestionReport{}
		}
	}
	if source := m.snapshot[progressKey]; source != "" {
		var envelope struct {
			Version int                                   `json:"version"`
			Records map[string]map[string]json.RawMessage `json:"records"`
		}
		if err := json.Unmarshal([]byte(source), &envelope); err != nil || envelope.Version != 1 || envelope.Records == nil {
			return fmt.Errorf("学习记录格式不正确，保留原文件")
		}
		m.rawRecords = envelope.Records
	}
	m.Records = map[string]*Record{}
	for _, q := range m.Questions {
		r := &Record{Wrong: q.History.InWrongBook, Strokes: []Stroke{}, Attempts: []map[string]any{}}
		if raw, ok := m.rawRecords[q.ID]; ok {
			b, _ := json.Marshal(raw)
			if err := json.Unmarshal(b, r); err != nil {
				return fmt.Errorf("题目 %s 的记录损坏: %w", q.ID, err)
			}
		}
		if err := validateNoteImages(r); err != nil {
			return fmt.Errorf("题目 %s 的笔记图片损坏，保留原文件: %w", q.ID, err)
		}
		if err := validateInkRecord(r); err != nil {
			return fmt.Errorf("题目 %s 的笔迹损坏，保留原文件: %w", q.ID, err)
		}
		m.Records[q.ID] = r
	}
	m.Round = Round{Mode: "practice", WrongScope: "due", Settings: DrawSettings{Subject: "全部学科", Range: "all", Count: "20"}}
	if raw := m.snapshot[roundKey]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &m.Round); err != nil {
			return err
		}
	}
	if !slices.Contains([]string{"practice", "wrong", "random", "notes"}, m.Round.Mode) {
		m.Round.Mode = "practice"
	}
	if m.Round.Settings.Subject == "" {
		m.Round.Settings = DrawSettings{Subject: "全部学科", Range: "all", Count: "20"}
	}
	if m.Round.WrongScope != "all" {
		m.Round.WrongScope = "due"
	}
	if m.Find(m.Round.CurrentID) < 0 {
		m.Round.CurrentID = m.snapshot["408-study-location-v1"]
	}
	if m.Find(m.Round.CurrentID) < 0 {
		m.Round.CurrentID = m.Questions[0].ID
	}
	if err := m.restorePasses(); err != nil {
		return err
	}
	m.Theme = m.snapshot["408-theme-v1"]
	if m.Theme != "dark" {
		m.Theme = "light"
	}
	return nil
}
func (m *Model) Find(id string) int {
	if m.index == nil {
		m.index = map[string]int{}
		for i, q := range m.Questions {
			m.index[q.ID] = i
		}
	}
	if i, ok := m.index[id]; ok {
		return i
	}
	return -1
}
func (m *Model) Current() *Question { return &m.Questions[max(0, m.Find(m.Round.CurrentID))] }
func (m *Model) Save() error {
	m.capturePassWorkspace()
	m.savePasses()
	for id, r := range m.Records {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(b, &fields)
		original := m.rawRecords[id]
		if original == nil {
			original = map[string]json.RawMessage{}
		}
		previousInk := original["ink"]
		for k, v := range fields {
			original[k] = v
		}
		for _, name := range []string{"ink", "inkClips", "noteTags", "notePrivate", "excluded", "noteImages"} {
			if _, ok := fields[name]; !ok {
				delete(original, name)
			}
		}
		// Retain unrecognized document metadata while replacing known ink fields.
		if current, ok := fields["ink"]; ok {
			var prior, updated map[string]json.RawMessage
			_ = json.Unmarshal(previousInk, &prior)
			_ = json.Unmarshal(current, &updated)
			if prior != nil {
				for key, value := range updated {
					prior[key] = value
				}
				merged, _ := json.Marshal(prior)
				original["ink"] = merged
			}
		}
		if r.Review == nil {
			delete(original, "review")
		}
		m.rawRecords[id] = original
	}
	b, err := json.Marshal(map[string]any{"version": 1, "records": m.rawRecords})
	if err != nil {
		return err
	}
	m.snapshot[progressKey] = string(b)
	b, err = json.Marshal(m.Round)
	if err != nil {
		return err
	}
	m.snapshot[roundKey] = string(b)
	m.ReviewConfig = SetReviewConfig(m.ReviewConfig)
	cfgBytes, err := json.Marshal(m.ReviewConfig)
	if err != nil {
		return err
	}
	m.snapshot[reviewConfigKey] = string(cfgBytes)
	if m.AIConfig.hasCustomValues() || m.snapshot[aiConfigKey] != "" {
		aiBytes, err := json.Marshal(m.AIConfig)
		if err != nil {
			return err
		}
		m.snapshot[aiConfigKey] = string(aiBytes)
	}
	if len(m.Reports) > 0 {
		repBytes, err := json.Marshal(m.Reports)
		if err != nil {
			return err
		}
		m.snapshot[questionReportsKey] = string(repBytes)
	} else {
		delete(m.snapshot, questionReportsKey)
	}
	m.snapshot["408-theme-v1"] = m.Theme
	m.snapshot["408-study-location-v1"] = m.Round.CurrentID
	if m.Disk != nil {
		if err = m.Disk.Save(m.snapshot); err != nil {
			m.SaveError = err.Error()
			return err
		}
		if err = m.syncQuestionReportsFile(); err != nil {
			m.SaveError = err.Error()
			return err
		}
	}
	m.SaveError = ""
	return nil
}
func (m *Model) IDs(subject, pool string, now time.Time) []string {
	var ids []string
	for _, q := range m.Questions {
		r := m.Records[q.ID]
		if subject != "全部学科" && subject != "" && q.Subject != subject {
			continue
		}
		if pool == "wrong" && !r.Wrong || pool == "due" && !Due(q, *r, now) || pool == "unpracticed" && len(r.Attempts) > 0 || pool == "notes" && (r.NotePrivate || strings.TrimSpace(r.Notes) == "" && len(r.InkClips) == 0) {
			continue
		}
		ids = append(ids, q.ID)
	}
	return ids
}
func shuffled(ids []string) []string {
	out := slices.Clone(ids)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
func (m *Model) Queue(subject, chapter string, now time.Time) []string {
	matches := func(q Question) bool {
		return (subject == "全部学科" || q.Subject == subject) && (chapter == "全部章节" || q.Chapter == chapter)
	}
	// The immutable source library already has unique IDs. Avoid constructing
	// an all-ID list plus a second deduplication map for every pointer frame.
	if m.Round.Mode != "random" && m.Round.Mode != "notes" && m.Round.Mode != "wrong" {
		out := make([]string, 0, len(m.Questions))
		for _, q := range m.Questions {
			if matches(q) && m.inPass(q.ID) {
				out = append(out, q.ID)
			}
		}
		return out
	}
	var ids []string
	switch m.Round.Mode {
	case "random":
		ids = m.Round.RandomIDs
	case "notes":
		ids = m.IDs("全部学科", "notes", now)
	case "wrong":
		pool := "due"
		if m.Round.WrongScope == "all" {
			pool = "wrong"
		}
		eligible := m.IDs("全部学科", pool, now)
		if m.Round.PassFilter != "" && m.Round.PassFilter != "all" {
			eligible = slices.DeleteFunc(eligible, func(id string) bool { return !m.passMatches(id, m.Round.PassFilter) })
		}
		available := make(map[string]bool, len(eligible))
		existing := make(map[string]bool, len(m.Round.WrongIDs))
		for _, id := range m.Round.WrongIDs {
			existing[id] = true
		}
		var arrivals []string
		for _, id := range eligible {
			available[id] = true
			if !existing[id] {
				arrivals = append(arrivals, id)
			}
		}
		// Only genuinely new arrivals need randomization. Existing round order
		// remains stable, with no repeated shuffling/searches on ordinary frames.
		if len(arrivals) > 0 {
			m.Round.WrongIDs = append(m.Round.WrongIDs, shuffled(arrivals)...)
		}
		ids = make([]string, 0, len(m.Round.WrongIDs))
		for _, id := range m.Round.WrongIDs {
			if available[id] {
				ids = append(ids, id)
			}
		}
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		idx := m.Find(id)
		if idx < 0 || seen[id] {
			continue
		}
		if matches(m.Questions[idx]) {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
func (m *Model) DrawPool(now time.Time) []string {
	settings := m.Round.Settings
	if m.ActivePass() != nil {
		ids := m.IDs("全部学科", settings.Range, now)
		return slices.DeleteFunc(ids, func(id string) bool {
			q := m.Questions[m.Find(id)]
			return !m.inPass(id) || !m.passMatches(id, settings.Range) || (settings.Subjects != nil && !slices.Contains(settings.Subjects, q.Subject)) || (settings.Subjects == nil && settings.Subject != "全部学科" && settings.Subject != q.Subject)
		})
	}
	if settings.Subjects == nil {
		return m.IDs(settings.Subject, settings.Range, now)
	}
	ids := m.IDs("全部学科", settings.Range, now)
	return slices.DeleteFunc(ids, func(id string) bool { return !slices.Contains(settings.Subjects, m.Questions[m.Find(id)].Subject) })
}
func (m *Model) Draw(count int, now time.Time) error {
	pool := m.DrawPool(now)
	if count < 1 || count > len(pool) {
		return fmt.Errorf("请填写 1～%d 之间的整数题量", len(pool))
	}
	m.Round.RandomPassID = m.Study.Active
	m.Round.BatchID = fmt.Sprintf("batch-%d", now.UnixNano())
	m.Round.RandomIDs = shuffled(pool)[:count]
	m.Round.CurrentID = m.Round.RandomIDs[0]
	m.Round.RandomCurrentID = m.Round.CurrentID
	return m.Save()
}
func cloneRecord(r *Record) *Record {
	b, _ := json.Marshal(r)
	var out Record
	_ = json.Unmarshal(b, &out)
	return &out
}
func (m *Model) applyAttemptReview(q Question, r *Record, a map[string]any, outcome, quality string, now time.Time) {
	if r.Wrong || outcome != "right" {
		state, meta := ApplyReview(q, *r, outcome, now)
		if strings.Contains(q.Errata, "标答存疑") {
			state = ReviewState(q, *r)
			meta = map[string]any{"blocked": true, "qualified": false, "reason": "标答存疑，仅记录作答，不自动排期。"}
		}
		meta["quality"] = quality
		meta["before"] = scheduleSnapshot(r.Review)
		meta["after"] = scheduleSnapshot(state)
		a["review"] = meta
		r.Review = state
	}
	r.Wrong = r.Wrong || outcome != "right"
	r.Attempts = append(r.Attempts, a)
	if meta, ok := a["review"].(map[string]any); ok {
		at := NextReview(q, *r)
		if !at.IsZero() {
			meta["readyAt"] = stamp(at)
		}
	}
}

func (m *Model) Submit(choices []string, quality string, unknown bool, now time.Time) (bool, error) {
	q := *m.Current()
	r := m.Records[q.ID]
	if q.IsSubjective() {
		return false, fmt.Errorf("主观题请使用主观作答提交")
	}
	correct := slices.Equal(choices, q.AnswerChoices)
	if !unknown && (len(choices) != len(q.AnswerChoices) || slices.Contains(choices, "")) {
		return false, fmt.Errorf("请完成所有选项后提交")
	}
	m.Before = cloneRecord(r)
	id := fmt.Sprintf("native-%d", now.UnixNano())
	m.LastAttemptID = id
	outcome := "wrong"
	if unknown {
		outcome = "unknown"
	} else if correct {
		outcome = "right"
		if quality != "independent" {
			outcome = "uncertain"
		}
	}
	a := map[string]any{"passId": m.attemptPassID(), "mode": m.Round.Mode, "batchId": m.Round.BatchID, "disputed": strings.Contains(q.Errata, "标答存疑"), "id": id, "choices": choices, "correct": correct && !unknown, "quality": quality, "outcome": outcome, "time": stamp(now)}
	m.applyAttemptReview(q, r, a, outcome, quality, now)
	return correct && !unknown, m.Save()
}

func normalizeSubjectiveOutcome(outcome string) string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "right", "correct", "independent":
		return "right"
	case "uncertain", "partial", "hint":
		return "uncertain"
	case "unknown":
		return "unknown"
	default:
		return "wrong"
	}
}

func toStringSlice(v any) []string {
	switch xs := v.(type) {
	case []string:
		return slices.Clone(xs)
	case []any:
		out := make([]string, 0, len(xs))
		for _, item := range xs {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func (m *Model) SubmitSubjective(answerText string, outcome string, grade *AIGradeResult, now time.Time) (bool, error) {
	return m.submitSubjective(answerText, outcome, grade, false, now)
}

func (m *Model) submitSubjective(answerText string, outcome string, grade *AIGradeResult, allowEmpty bool, now time.Time) (bool, error) {
	q := *m.Current()
	r := m.Records[q.ID]
	outcome = normalizeSubjectiveOutcome(outcome)
	unknown := outcome == "unknown"
	if !unknown && !allowEmpty && strings.TrimSpace(answerText) == "" {
		return false, fmt.Errorf("请填写作答内容后再提交")
	}
	m.Before = cloneRecord(r)
	id := fmt.Sprintf("native-%d", now.UnixNano())
	m.LastAttemptID = id
	quality := "independent"
	switch outcome {
	case "uncertain":
		quality = "uncertain"
	case "wrong":
		quality = "wrong"
	case "unknown":
		quality = "unknown"
	}
	correct := outcome == "right"
	a := map[string]any{
		"passId":     m.attemptPassID(),
		"mode":       m.Round.Mode,
		"batchId":    m.Round.BatchID,
		"disputed":   strings.Contains(q.Errata, "标答存疑"),
		"id":         id,
		"answerText": answerText,
		"correct":    correct,
		"quality":    quality,
		"outcome":    outcome,
		"time":       stamp(now),
	}
	if grade != nil {
		a["score"] = grade.Score
		a["maxScore"] = grade.MaxScore
		a["feedback"] = grade.Feedback
		if len(grade.MatchedPoints) > 0 {
			a["matchedPoints"] = slices.Clone(grade.MatchedPoints)
		}
		if len(grade.MissedPoints) > 0 {
			a["missedPoints"] = slices.Clone(grade.MissedPoints)
		}
	}
	m.applyAttemptReview(q, r, a, outcome, quality, now)
	return correct, m.Save()
}

func (m *Model) OverrideLastSubjectiveOutcome(outcome string, now time.Time) error {
	q := m.Current()
	r := m.Records[q.ID]
	if m.Before == nil || len(r.Attempts) == 0 || r.Attempts[len(r.Attempts)-1]["id"] != m.LastAttemptID {
		return fmt.Errorf("当前没有可改判的作答记录")
	}
	last := r.Attempts[len(r.Attempts)-1]
	answerText, _ := last["answerText"].(string)
	var grade *AIGradeResult
	if _, hasScore := last["score"]; hasScore {
		g := &AIGradeResult{Outcome: normalizeSubjectiveOutcome(outcome)}
		if v, ok := last["score"].(float64); ok {
			g.Score = v
		}
		if v, ok := last["maxScore"].(float64); ok {
			g.MaxScore = v
		}
		g.Feedback, _ = last["feedback"].(string)
		g.MatchedPoints = toStringSlice(last["matchedPoints"])
		g.MissedPoints = toStringSlice(last["missedPoints"])
		grade = g
	}
	old := m.Before
	oldOutcome, _ := last["outcome"].(string)
	produced := old.Wrong || oldOutcome != "right"
	r.Attempts = old.Attempts
	r.Review = old.Review
	if r.Wrong == produced {
		r.Wrong = old.Wrong
	}
	m.Before = nil
	_, err := m.submitSubjective(answerText, outcome, grade, true, now)
	return err
}
func (m *Model) Undo() error {
	q := m.Current()
	r := m.Records[q.ID]
	if m.Before == nil || len(r.Attempts) == 0 || r.Attempts[len(r.Attempts)-1]["id"] != m.LastAttemptID {
		return nil
	}
	old := m.Before
	outcome, _ := r.Attempts[len(r.Attempts)-1]["outcome"].(string)
	produced := old.Wrong || outcome != "right"
	r.Attempts = old.Attempts
	r.Review = old.Review
	if r.Wrong == produced {
		r.Wrong = old.Wrong
	}
	m.Before = nil
	return m.Save()
}

func scheduleSnapshot(r *Review) any {
	if r == nil {
		return nil
	}
	return map[string]any{"stage": r.Stage, "dueAt": r.DueAt, "intervalDays": r.IntervalDays, "halfLifeDays": r.HalfLifeDays, "right": r.Right, "wrong": r.Wrong, "unknown": r.Unknown, "uncertain": r.Uncertain, "streak": r.Streak}
}
