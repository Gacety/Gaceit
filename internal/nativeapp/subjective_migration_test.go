package nativeapp

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/egoist/mygo/ui"
	"study408/internal/store"
)

func encodeTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func customBankAssets(t *testing.T) fs.FS {
	t.Helper()
	origSubjects := slices.Clone(Subjects)
	t.Cleanup(func() {
		Subjects = origSubjects
		SetReviewConfig(DefaultReviewConfig())
	})
	font, err := fs.ReadFile(assetsForTest(), "prototype/public/fonts/NotoSansSC-Bold.otf")
	if err != nil {
		t.Fatal(err)
	}
	_ = ui.RegisterFont(font, "Study Noto")
	reviewFont, err := fs.ReadFile(assetsForTest(), "native-assets/fonts/NotoSansSC-Variable.ttf")
	if err != nil {
		t.Fatal(err)
	}
	_ = ui.RegisterFont(reviewFont, "Review Noto")

	questionsJSON := `[
  {
    "id": "custom-os-03-001",
    "subject": "操作系统",
    "chapter": "内存管理",
    "title": "内存管理 · 综合第 1 题",
    "kind": "subjective",
    "kindLabel": "综合应用题",
    "text": "某系统采用二级页表分页存储管理……请计算逻辑地址 ` + "`0x12345678`" + ` 对应的页目录号、页号和页内偏移量，并说明快表（TLB）未命中时的访存次数。",
    "options": [],
    "answer": "1. 页目录号：0x48；页号：0x345；页内偏移：0x678。\n2. 访存次数为 3 次（一级页表 1 次、二级页表 1 次、物理内存单元 1 次）。",
    "answerChoices": [],
    "rubric": "共 10 分：① 正确拆解地址结构给 6 分（三个字段各 2 分）；② 答出 3 次访存给 2 分，说明三次分别访问什么给 2 分。",
    "explanation": "详细解析内容……",
    "errata": ""
  },
  {
    "id": "custom-db-01-001",
    "subject": "数据库系统",
    "chapter": "关系代数",
    "title": "关系代数 · 第 1 题",
    "kind": "choice",
    "kindLabel": "单项选择",
    "text": "关系模型如图：\n\n![图1](/assets/custom-db/fig.png)\n\n下列选项正确的是（）",
    "options": ["选项一", "选项二", "选项三", "选项四"],
    "answer": "A",
    "answerChoices": ["A"],
    "explanation": "参考 ![图2](/practice/collection-assets/custom-db/fig2.png)",
    "errata": ""
  }
]`
	pngBytes := encodeTestPNG(t)
	return fstest.MapFS{
		"questions.json":                              {Data: []byte(questionsJSON)},
		"assets/custom-db/fig.png":                    {Data: pngBytes},
		"prototype/public/assets/custom-db/fig2.png":  {Data: pngBytes},
		"native-assets/math/manifest.json":            {Data: []byte(`{}`)},
		"prototype/public/fonts/NotoSansSC-Bold.otf":  {Data: font},
		"native-assets/fonts/NotoSansSC-Variable.ttf": {Data: reviewFont},
	}
}

func TestDynamicSubjectsCustomBankAndGenericAssets(t *testing.T) {
	bank := customBankAssets(t)
	a, err := New(bank, &store.Store{Path: filepath.Join(t.TempDir(), "progress.json")})
	if err != nil {
		t.Fatal(err)
	}
	wantSubjects := []string{"全部学科", "操作系统", "数据库系统"}
	if !slices.Equal(a.M.SubjectList(), wantSubjects) {
		t.Fatalf("dynamic subjects mismatch: got %v want %v", a.M.SubjectList(), wantSubjects)
	}
	if !a.M.Questions[0].IsSubjective() || a.M.Questions[1].IsSubjective() {
		t.Fatal("question subjective classification mismatch")
	}
	if !strings.Contains(a.M.Questions[0].Rubric, "共 10 分") {
		t.Fatalf("rubric not parsed: %q", a.M.Questions[0].Rubric)
	}
	if _, err := a.Rich.image("/assets/custom-db/fig.png"); err != nil {
		t.Fatalf("generic /assets/ load failed: %v", err)
	}
	if _, err := a.Rich.image("/practice/collection-assets/custom-db/fig2.png"); err != nil {
		t.Fatalf("collection-assets fallback load failed: %v", err)
	}
	if _, err := a.Rich.image("/assets/../questions.json"); err == nil {
		t.Fatal("path traversal was not blocked")
	}
	if _, err := a.Rich.image("/etc/passwd"); err == nil {
		t.Fatal("disallowed prefix was not blocked")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestSubjectiveAIGradingScheduleAndOverride(t *testing.T) {
	bank := customBankAssets(t)
	a, err := New(bank, &store.Store{Path: filepath.Join(t.TempDir(), "progress.json")})
	if err != nil {
		t.Fatal(err)
	}
	a.studyDetail = true
	if _, err := a.M.SubmitSubjective("   ", "right", nil, time.Now()); err == nil {
		t.Fatal("empty subjective answer should be rejected")
	}

	var gotAuth, gotURL string
	origClient := aiHTTPClient
	t.Cleanup(func() {
		aiHTTPClient = origClient
	})
	aiHTTPClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotAuth = r.Header.Get("Authorization")
			gotURL = r.URL.String()
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rec).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"content": "```json\n{\"score\":7.5,\"maxScore\":10,\"outcome\":\"uncertain\",\"matchedPoints\":[\"正确拆解地址\"],\"missedPoints\":[\"未说明三次访存具体对象\"],\"feedback\":\"思路正确，补充三次访存细节即可满分。\"}\n```",
						},
					},
				},
			})
			return rec.Result(), nil
		}),
	}

	a.M.AIConfig = AIConfig{
		Endpoint: "https://mock.ai.local/v1",
		APIKey:   "test-key-123",
		Model:    "gpt-test",
	}
	tt := ui.NewTester(a.View, 1440, 960)
	if _, ok := tt.Find("主观题作答输入"); !ok {
		t.Fatal("subjective answer input not rendered")
	}
	a.subjectiveText = "页目录号 0x48，页号 0x345，页内偏移 0x678，一共访存 3 次。"
	if err := tt.Click("AI 批改"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !a.answered && time.Now().Before(deadline) {
		time.Sleep(15 * time.Millisecond)
		tt.Advance(20 * time.Millisecond)
	}
	if !a.answered {
		t.Fatalf("AI grading did not complete; message=%q", a.message)
	}
	if gotURL != "https://mock.ai.local/v1/chat/completions" {
		t.Fatalf("unexpected request URL: %q", gotURL)
	}
	if gotAuth != "Bearer test-key-123" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	q := a.M.Current()
	r := a.M.Records[q.ID]
	if !r.Wrong || r.Review == nil || r.Review.IntervalDays != 1 {
		t.Fatalf("uncertain AI outcome did not schedule 24h review: wrong=%v review=%+v", r.Wrong, r.Review)
	}
	if a.lastGrade == nil || a.lastGrade.Score != 7.5 || !tt.HasText("AI 评分：7.5 / 10.0") {
		t.Fatalf("AI feedback not shown: %+v", a.lastGrade)
	}

	// Override first-try uncertain -> right -> keeps question out of wrong book.
	now := time.Now().Add(time.Minute)
	if err := a.M.OverrideLastSubjectiveOutcome("right", now); err != nil {
		t.Fatal(err)
	}
	if r.Wrong || r.Review != nil || r.Attempts[len(r.Attempts)-1]["outcome"] != "right" {
		t.Fatalf("override to right failed: wrong=%v review=%+v attempt=%+v", r.Wrong, r.Review, r.Attempts[len(r.Attempts)-1])
	}

	// Override to wrong -> 10 minute relearning (IntervalDays == 0).
	if err := a.M.OverrideLastSubjectiveOutcome("wrong", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if r.Review == nil || r.Review.IntervalDays != 0 || !r.Wrong {
		t.Fatalf("override to wrong failed: wrong=%v review=%+v", r.Wrong, r.Review)
	}

	// Undo removes the attempt and restores prior clean state.
	if err := a.M.Undo(); err != nil {
		t.Fatal(err)
	}
	if len(r.Attempts) != 0 || r.Wrong {
		t.Fatalf("undo failed: attempts=%d wrong=%v", len(r.Attempts), r.Wrong)
	}

	// Unknown shortcut does not require answerText.
	correct, err := a.M.SubmitSubjective("", "unknown", nil, now)
	if err != nil || correct || !r.Wrong || r.Review == nil || r.Review.IntervalDays != 0 {
		t.Fatalf("unknown subjective submission mismatch: correct=%v err=%v review=%+v", correct, err, r.Review)
	}

	// Subsequent independent recall on an existing wrong-book item advances to 3-day ladder.
	correct, err = a.M.SubmitSubjective("完整解答", "right", nil, now.Add(24*time.Hour))
	if err != nil || !correct || !r.Wrong || r.Review == nil || r.Review.IntervalDays != 3 {
		t.Fatalf("qualified recall on wrong-book subjective item mismatch: correct=%v err=%v wrong=%v review=%+v", correct, err, r.Wrong, r.Review)
	}
}

func TestBackupIncludesAndRestoresReviewParameters(t *testing.T) {
	bank := customBankAssets(t)
	m, err := NewModel(bank, &store.Store{Path: filepath.Join(t.TempDir(), "progress.json")})
	if err != nil {
		t.Fatal(err)
	}
	m.ReviewConfig = ReviewConfig{
		Algorithm:             "hlr-local-v1",
		SchedulePolicy:        "custom-ladder-v1",
		LadderDays:            []float64{2, 5, 12},
		RelearnDelayMinutes:   15,
		UncertainIntervalDays: 2,
		CooldownHours:         6,
		PriorWeights:          [3]float64{1.1, 1.8, -1.1},
		WeightLimits:          [3][2]float64{{-4, 8}, {.25, 3}, {-4, -.25}},
		HalfLifeMinDays:       15. / 1440,
		HalfLifeMaxDays:       1800,
		LearningRate:          0.25,
		L2Regularization:      0.02,
		Timezone:              "Asia/Taipei",
	}
	SetReviewConfig(m.ReviewConfig)
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, zone)
	if _, err := m.SubmitSubjective("测试解答", "uncertain", nil, now); err != nil {
		t.Fatal(err)
	}

	raw, err := m.ExportBackupData()
	if err != nil {
		t.Fatal(err)
	}
	var env BackupEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.ReviewConfig == nil || !slices.Equal(env.ReviewConfig.LadderDays, []float64{2, 5, 12}) || env.ReviewConfig.RelearnDelayMinutes != 15 {
		t.Fatalf("exported reviewConfig missing or mismatched: %+v", env.ReviewConfig)
	}
	st, ok := env.ReviewStates["custom-os-03-001"]
	if !ok || len(st.Weights) != 3 || st.HalfLifeDays == nil || st.IntervalDays != 2 || st.NextReviewAt == nil {
		t.Fatalf("exported reviewStates missing computation params: ok=%v state=%+v", ok, st)
	}

	// Modify top-level reviewConfig in exported JSON and import into a fresh model.
	env.ReviewConfig.LadderDays = []float64{4, 9, 21}
	modified, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := NewModel(bank, &store.Store{Path: filepath.Join(t.TempDir(), "fresh.json")})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := fresh.ImportBackupData(modified)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(restored.ReviewConfig.LadderDays, []float64{4, 9, 21}) {
		t.Fatalf("imported reviewConfig not applied: %+v", restored.ReviewConfig)
	}
	nextState, _ := ApplyReview(restored.Questions[0], *restored.Records["custom-os-03-001"], "right", now.Add(3*24*time.Hour))
	if nextState.IntervalDays != 4 {
		t.Fatalf("restored reviewConfig ladder did not govern scheduling: got %v want 4", nextState.IntervalDays)
	}

	// Legacy v1 backup without top-level reviewConfig / reviewStates still imports cleanly.
	legacyBytes, err := json.Marshal(map[string]any{
		"format":  "408-study-backup",
		"version": 1,
		"data":    m.snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.ImportBackupData(legacyBytes); err != nil {
		t.Fatalf("legacy backup import failed: %v", err)
	}
}

func TestQuestionErrorReportWorkflowAndArchive(t *testing.T) {
	bank := customBankAssets(t)
	dir := t.TempDir()
	storePath := filepath.Join(dir, "progress.json")
	a, err := New(bank, &store.Store{Path: storePath})
	if err != nil {
		t.Fatal(err)
	}
	a.M.BankTitle = "自定义测试题库"
	a.studyDetail = true

	tt := ui.NewTester(a.View, 1440, 960)
	if _, ok := tt.Find("题目报错"); !ok {
		t.Fatal("report button not found in reader header")
	}
	if err := tt.Click("题目报错"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("题目报错面板"); !ok {
		t.Fatal("report panel did not open")
	}
	if err := tt.Click("答案有误"); err != nil {
		t.Fatal(err)
	}
	a.reportComment = "参考答案里的页目录号与页号位数标注和32位二级页表划分不一致"
	a.reportSuggested = "请核对 0x12345678 的高 10 位应为 0x048（二进制前 10 位）"

	if err := tt.Click("复制 AI 修题指令"); err != nil {
		t.Fatal(err)
	}
	prompt := tt.Clipboard()
	if !strings.Contains(prompt, "custom-os-03-001") ||
		!strings.Contains(prompt, "答案有误") ||
		!strings.Contains(prompt, a.reportComment) ||
		!strings.Contains(prompt, a.reportSuggested) ||
		!strings.Contains(prompt, `"rubric"`) {
		t.Fatalf("clipboard AI prompt missing expected fields:\n%s", prompt)
	}

	if err := tt.Click("保存报错"); err != nil {
		t.Fatal(err)
	}
	if a.reportOpen {
		t.Fatal("expected report panel to collapse after save")
	}
	rep := a.M.QuestionReport("custom-os-03-001")
	if rep == nil || !slices.Equal(rep.Categories, []string{"题干有误", "答案有误"}) || rep.Status != "open" {
		t.Fatalf("saved report mismatch: %+v", rep)
	}

	archivePath := filepath.Join(dir, questionReportsFilename)
	rawArchive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("standalone AI report archive not written to %s: %v", archivePath, err)
	}
	var archive QuestionReportArchive
	if err := json.Unmarshal(rawArchive, &archive); err != nil {
		t.Fatalf("invalid AI report archive JSON: %v", err)
	}
	if archive.Format != "408-question-reports" || archive.Total != 1 || archive.OpenCount != 1 || len(archive.Reports) != 1 {
		t.Fatalf("unexpected archive header: %+v", archive)
	}
	entry := archive.Reports[0]
	if entry.QuestionID != "custom-os-03-001" || entry.Question == nil || entry.Question.Answer == "" || entry.Question.Text == "" {
		t.Fatalf("archive entry missing question snapshot for AI: %+v", entry)
	}

	// Verify persistence across reload.
	reloaded, err := NewModel(bank, &store.Store{Path: storePath})
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.QuestionReport("custom-os-03-001"); got == nil || got.Comment != a.reportComment {
		t.Fatalf("reloaded report mismatch: %+v", got)
	}

	// Verify backup export/import carries questionReports cleanly.
	backupBytes, err := reloaded.ExportBackupData()
	if err != nil {
		t.Fatal(err)
	}
	var env BackupEnvelope
	if err := json.Unmarshal(backupBytes, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.QuestionReports) != 1 || env.QuestionReports[0].QuestionID != "custom-os-03-001" {
		t.Fatalf("backup missing questionReports: %+v", env.QuestionReports)
	}
	freshDir := t.TempDir()
	freshModel, err := NewModel(bank, &store.Store{Path: filepath.Join(freshDir, "progress.json")})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := freshModel.ImportBackupData(backupBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := imported.QuestionReport("custom-os-03-001"); got == nil || got.SuggestedAnswer != a.reportSuggested {
		t.Fatalf("imported backup did not preserve question report: %+v", got)
	}

	// Deleting the report keeps question-reports.json synchronized with 0 entries.
	if err := tt.Click("编辑报错"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("删除报错"); err != nil {
		t.Fatal(err)
	}
	if a.M.QuestionReport("custom-os-03-001") != nil {
		t.Fatal("report was not deleted")
	}
	rawAfterDelete, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	var emptyArchive QuestionReportArchive
	if err := json.Unmarshal(rawAfterDelete, &emptyArchive); err != nil {
		t.Fatal(err)
	}
	if emptyArchive.Total != 0 || emptyArchive.OpenCount != 0 || len(emptyArchive.Reports) != 0 {
		t.Fatalf("archive not cleared after report deletion: %+v", emptyArchive)
	}
}
