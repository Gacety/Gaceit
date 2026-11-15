package nativeapp

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

func cleanPassQuestions(m *Model, p *StudyPass, n int) []string {
	var ids []string
	for _, id := range p.IDs {
		r := m.Records[id]
		if len(r.Attempts) == 0 && !r.Wrong && !slices.Contains(p.PriorWrong, id) {
			ids = append(ids, id)
			if len(ids) == n {
				break
			}
		}
	}
	return ids
}

func submitAt(m *Model, id string, choices []string, quality string, unknown bool, now time.Time) error {
	m.Round.CurrentID = id
	_, err := m.Submit(choices, quality, unknown, now)
	return err
}

func TestStudyPassFirstAnswerProgress(t *testing.T) {
	a := testApp(t)
	m := a.M
	now := time.Now()
	if err := m.CreatePass("408 二刷", "全部学科", "全部章节", now); err != nil {
		t.Fatal(err)
	}
	p := m.ActivePass()
	if p == nil || len(p.IDs) != len(m.Questions) || m.Study.Active != p.ID {
		t.Fatal("pass not created or activated", p)
	}
	if m.Round.Settings.Range != "pass-unfinished" {
		t.Fatal("active pass should default to pass-unfinished", m.Round.Settings.Range)
	}
	ids := cleanPassQuestions(m, p, 3)
	if len(ids) < 3 {
		t.Fatal("not enough clean questions", len(ids))
	}
	// First valid submission completes the question in this round.
	q := m.Questions[m.Find(ids[0])]
	m.Round.Mode = "practice"
	if err := submitAt(m, q.ID, slices.Clone(q.AnswerChoices), "independent", false, now); err != nil {
		t.Fatal(err)
	}
	if last := m.Records[q.ID].Attempts[len(m.Records[q.ID].Attempts)-1]; last["passId"] != p.ID || last["mode"] != "practice" {
		t.Fatal("practice attempt not tagged with the active pass", last)
	}
	s := m.PassStats("全部学科", "全部章节")
	if s.Total != len(p.IDs) || s.Done != 1 || s.Right != 1 || s.Scored != 1 {
		t.Fatal("first answer did not define round progress", s)
	}
	// Answering again (unknown) must not rewrite the first-answer statistic.
	if err := submitAt(m, q.ID, nil, "independent", true, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if s = m.PassStats("全部学科", "全部章节"); s.Done != 1 || s.Right != 1 {
		t.Fatal("reinforcement attempt rewrote first-answer stats", s)
	}
	if first, _ := m.passAttempts(q.ID, p.ID); first["outcome"] != "right" {
		t.Fatal("first attempt lost", first)
	}
	// Review-mode (wrong) answers never complete round progress.
	m.Round.Mode = "wrong"
	m.Round.WrongScope = "all"
	if err := submitAt(m, ids[1], nil, "independent", true, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if last := m.Records[ids[1]].Attempts[len(m.Records[ids[1]].Attempts)-1]; last["passId"] != "" {
		t.Fatal("review attempt leaked into round progress", last)
	}
	if !m.passMatches(ids[1], "pass-unfinished") {
		t.Fatal("review attempt should not complete the round question")
	}
	// A first wrong answer in the round exposes the round filters. Unknown
	// counts as completed for the round while still marking first-wrong.
	m.Round.Mode = "practice"
	if err := submitAt(m, ids[2], nil, "independent", true, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, filter := range []string{"pass-first-wrong", "pass-new"} {
		if !m.passMatches(ids[2], filter) {
			t.Fatal("wrong first answer missing from filter", filter)
		}
	}
	if m.passMatches(ids[2], "pass-unfinished") {
		t.Fatal("unknown submission should complete the round question")
	}
	if m.passMatches(ids[0], "pass-unfinished") || m.passMatches(ids[0], "pass-first-wrong") {
		t.Fatal("completed question still matches unfinished/first-wrong")
	}
	// Wrong-book queue respects the round filter and keeps existing order.
	m.Round.Mode = "wrong"
	m.Round.PassFilter = "pass-new"
	m.Round.WrongIDs = nil
	queue := m.Queue("全部学科", "全部章节", now.Add(3*time.Minute))
	if !slices.Contains(queue, ids[2]) || slices.Contains(queue, ids[1]) {
		t.Fatal("pass-new filter wrong on wrong-book queue", queue)
	}
	m.Round.PassFilter = "pass-unfinished"
	m.Round.WrongIDs = nil
	queue = m.Queue("全部学科", "全部章节", now.Add(3*time.Minute))
	if !slices.Contains(queue, ids[1]) || slices.Contains(queue, ids[0]) {
		t.Fatal("pass-unfinished filter wrong on wrong-book queue", queue)
	}
	m.Round.PassFilter = ""
}

func TestStudyPassUndoRestoresRoundProgress(t *testing.T) {
	a := testApp(t)
	m := a.M
	now := time.Now()
	if err := m.CreatePass("408 二刷", "全部学科", "全部章节", now); err != nil {
		t.Fatal(err)
	}
	p := m.ActivePass()
	ids := cleanPassQuestions(m, p, 1)
	m.Round.Mode = "practice"
	if err := submitAt(m, ids[0], nil, "independent", true, now); err != nil {
		t.Fatal(err)
	}
	if !m.Records[ids[0]].Wrong {
		t.Fatal("wrong answer should join the wrong book")
	}
	if err := m.Undo(); err != nil {
		t.Fatal(err)
	}
	r := m.Records[ids[0]]
	if len(r.Attempts) != 0 || r.Wrong {
		t.Fatal("undo did not restore the record", r)
	}
	if !m.passMatches(ids[0], "pass-unfinished") {
		t.Fatal("undo did not roll back round progress")
	}
	if s := m.PassStats("全部学科", "全部章节"); s.Done != 0 {
		t.Fatal("undo left round stats dirty", s)
	}
}

func TestRoundWrongEntersSharedWrongBook(t *testing.T) {
	a := testApp(t)
	m := a.M
	now := time.Now()
	if err := m.CreatePass("408 二刷", "数据结构", "全部章节", now); err != nil {
		t.Fatal(err)
	}
	p := m.ActivePass()
	ids := cleanPassQuestions(m, p, 1)
	m.Round.Mode = "practice"
	if err := submitAt(m, ids[0], nil, "independent", true, now); err != nil {
		t.Fatal(err)
	}
	if !m.Records[ids[0]].Wrong {
		t.Fatal("round wrong answer did not join the shared wrong book")
	}
	// The 10-minute relearning delay is the existing HLR policy: not yet due.
	if slices.Contains(m.IDs("全部学科", "due", now), ids[0]) {
		t.Fatal("relearning delay bypassed")
	}
	if !slices.Contains(m.IDs("全部学科", "due", now.Add(11*time.Minute)), ids[0]) {
		t.Fatal("not scheduled for review after the relearning delay")
	}
	// Visible in the 全部 scope immediately, independent of the active round.
	if !slices.Contains(m.IDs("全部学科", "wrong", now), ids[0]) {
		t.Fatal("missing from the 全部 wrong scope")
	}
	m.Round.Mode = "wrong"
	m.Round.WrongScope = "all"
	m.Round.PassFilter = ""
	m.Round.WrongIDs = nil
	if !slices.Contains(m.Queue("全部学科", "全部章节", now), ids[0]) {
		t.Fatal("wrong-mode queue missing the round-wrong question")
	}
	// Round switching and other rounds never remove shared wrong-book membership.
	if err := m.SelectPass(""); err != nil {
		t.Fatal(err)
	}
	m.Round.Mode = "wrong"
	m.Round.WrongScope = "all"
	m.Round.PassFilter = ""
	m.Round.WrongIDs = nil
	if !slices.Contains(m.Queue("全部学科", "全部章节", now), ids[0]) {
		t.Fatal("round switch removed the question from the shared wrong book")
	}
	for _, filter := range []string{"pass-new", "pass-again", "pass-improved", "pass-first-wrong"} {
		m.Round.PassFilter = filter
		m.Round.WrongIDs = nil
		_ = m.Queue("全部学科", "全部章节", now)
		if !slices.Contains(m.IDs("全部学科", "wrong", now), ids[0]) {
			t.Fatal("filter changed wrong-book membership", filter)
		}
	}
	m.Round.PassFilter = ""
}

func TestSelectPassResetsFiltersAndPersists(t *testing.T) {
	a := testApp(t)
	m := a.M
	now := time.Now()
	if err := m.CreatePass("408 二刷", "全部学科", "全部章节", now); err != nil {
		t.Fatal(err)
	}
	p := m.ActivePass()
	m.Round.PassFilter = "pass-new"
	if err := m.SelectPass(""); err != nil {
		t.Fatal(err)
	}
	if m.Round.PassFilter != "" || m.Round.Settings.Range != "all" || m.ActivePass() != nil {
		t.Fatal("history selection did not reset pass-scoped state")
	}
	if err := m.SelectPass(p.ID); err != nil {
		t.Fatal(err)
	}
	if m.Round.PassFilter != "" || m.Round.Settings.Range != "pass-unfinished" {
		t.Fatal("round selection did not reset pass filter or default range")
	}
	if err := m.SelectPass("missing"); err == nil {
		t.Fatal("missing pass accepted")
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	m2, err := NewModel(assetsForTest(), m.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if m2.ActivePass() == nil || m2.ActivePass().ID != p.ID || len(m2.Study.Passes) != 1 {
		t.Fatal("passes not restored after reload")
	}
	if m2.Round.Settings.Range != "pass-unfinished" {
		t.Fatal("round default range not restored", m2.Round.Settings.Range)
	}
	// Unknown document metadata and source IDs survive saves and reloads.
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(m2.snapshot[studyPassKey]), &top); err != nil {
		t.Fatal(err)
	}
	top["futureFlag"] = json.RawMessage(`true`)
	b, _ := json.Marshal(top)
	m2.snapshot[studyPassKey] = string(b)
	if err := m2.Save(); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(m2.snapshot[studyPassKey]), &top); err != nil || string(top["futureFlag"]) != "true" {
		t.Fatal("unknown pass metadata dropped on save")
	}
}

func TestRandomDrawSharesRoundProgress(t *testing.T) {
	a := testApp(t)
	m := a.M
	now := time.Now()
	if err := m.CreatePass("408 二刷", "数据结构", "全部章节", now); err != nil {
		t.Fatal(err)
	}
	p := m.ActivePass()
	m.Round.Mode = "random"
	m.Round.Settings.Subjects = []string{"数据结构"}
	m.Round.Settings.Range = "pass-unfinished"
	if err := m.Draw(3, now); err != nil {
		t.Fatal(err)
	}
	if m.Round.RandomPassID != p.ID {
		t.Fatal("random batch not bound to the active pass")
	}
	drawn := m.Round.RandomIDs
	if len(drawn) != 3 {
		t.Fatal(len(drawn))
	}
	q := m.Questions[m.Find(drawn[0])]
	if err := submitAt(m, q.ID, slices.Clone(q.AnswerChoices), "independent", false, now); err != nil {
		t.Fatal(err)
	}
	if last := m.Records[q.ID].Attempts[len(m.Records[q.ID].Attempts)-1]; last["passId"] != p.ID || last["batchId"] != m.Round.BatchID {
		t.Fatal("random attempt missing pass/batch provenance", last)
	}
	// Random completion counts toward the same round; a redraw excludes it.
	if err := m.Draw(3, now); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(m.Round.RandomIDs, q.ID) {
		t.Fatal("completed question redrawn under pass-unfinished")
	}
	// Chapter practice sees the same completion: no clean question left in that
	// chapter beyond the drawn set.
	if first, _ := m.passAttempts(q.ID, p.ID); first == nil {
		t.Fatal("random completion not shared with round progress")
	}
}
