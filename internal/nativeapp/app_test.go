package nativeapp

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"study408/internal/store"
	"testing"
	"time"
)

func assetsForTest() fs.FS { return os.DirFS("../..") }
func testApp(t *testing.T) *App {
	t.Helper()
	font, _ := fs.ReadFile(assetsForTest(), "prototype/public/fonts/NotoSansSC-Bold.otf")
	_ = ui.RegisterFont(font, "Study Noto")
	reviewFont, _ := fs.ReadFile(assetsForTest(), "native-assets/fonts/NotoSansSC-Variable.ttf")
	_ = ui.RegisterFont(reviewFont, "Review Noto")
	a, err := New(assetsForTest(), &store.Store{Path: filepath.Join(t.TempDir(), "progress.json")})
	if err != nil {
		t.Fatal(err)
	}
	// These existing functional tests target the question reader.
	a.studyDetail = true
	return a
}
func TestOriginalLibraryAndMigration(t *testing.T) {
	a := testApp(t)
	if len(a.M.Questions) != 1576 {
		t.Fatal(len(a.M.Questions))
	}
	wrong, errata := 0, 0
	for _, q := range a.M.Questions {
		if a.M.Records[q.ID].Wrong {
			wrong++
		}
		if q.Errata != "" {
			errata++
		}
	}
	if wrong != 249 || errata != 30 {
		t.Fatal(wrong, errata)
	}
	q := a.M.Current()
	a.M.Records[q.ID].Notes = "原生笔记：保存后恢复"
	a.M.snapshot["408-other-data"] = `{"keep":true}`
	if err := a.M.Save(); err != nil {
		t.Fatal(err)
	}
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil || m.Records[q.ID].Notes != "原生笔记：保存后恢复" || m.snapshot["408-other-data"] == "" {
		t.Fatal(err, m)
	}
}
func TestReviewParityWithBrowserModel(t *testing.T) {
	data, _ := os.ReadFile("review-fixtures.json")
	var cases []struct {
		Question       Question
		Record         Record
		Outcome, Now   string
		State          Review
		Qualified, Due bool
		Next           *string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for i, tc := range cases {
		now, _ := time.Parse(time.RFC3339Nano, tc.Now)
		got, meta := ApplyReview(tc.Question, tc.Record, tc.Outcome, now)
		if meta["qualified"] != tc.Qualified || Due(tc.Question, tc.Record, now) != tc.Due {
			t.Fatalf("case %d eligibility mismatch", i)
		}
		b, _ := json.Marshal(got)
		want, _ := json.Marshal(tc.State)
		var x, y any
		_ = json.Unmarshal(b, &x)
		_ = json.Unmarshal(want, &y)
		if !near(x, y) {
			t.Fatalf("case %d model mismatch\ngot %s\nwant %s", i, b, want)
		}
		ready := NextReview(tc.Question, tc.Record)
		if tc.Next == nil && !ready.IsZero() || tc.Next != nil && stamp(ready) != *tc.Next {
			t.Fatalf("case %d next time", i)
		}
	}
}
func near(x, y any) bool {
	switch a := x.(type) {
	case float64:
		b, ok := y.(float64)
		return ok && math.Abs(a-b) < 1e-8
	case map[string]any:
		b, ok := y.(map[string]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if !near(v, b[k]) {
				return false
			}
		}
		return true
	case []any:
		b, ok := y.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i, v := range a {
			if !near(v, b[i]) {
				return false
			}
		}
		return true
	default:
		return x == y
	}
}
func TestNativeViewAnswerAndExclusiveTools(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 960)
	for _, label := range []string{"草稿纸", "本题笔记", "收起笔记卡", "退出草稿"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if a.tool != "" {
		t.Fatal(a.tool)
	}
	if err := tt.Click("第1空 选项A"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("提交答案"); err != nil {
		t.Fatal(err)
	}
	if !a.correct || !a.answered || len(a.M.Records[a.M.Current().ID].Attempts) != 1 {
		t.Fatal("native answer not recorded")
	}
	a.M.Records[a.M.Current().ID].Notes = "提交后补写"
	_ = a.M.Undo()
	if len(a.M.Records[a.M.Current().ID].Attempts) != 0 || a.M.Records[a.M.Current().ID].Notes != "提交后补写" {
		t.Fatal("undo lost new note")
	}
}
func TestStableRandomRound(t *testing.T) {
	a := testApp(t)
	a.M.Round.Mode = "random"
	if err := a.M.Draw(20, time.Now()); err != nil {
		t.Fatal(err)
	}
	order := strings.Join(a.M.Round.RandomIDs, ",")
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil || strings.Join(m.Round.RandomIDs, ",") != order {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, id := range m.Round.RandomIDs {
		if set[id] {
			t.Fatal("duplicate")
		}
		set[id] = true
	}
	if a.M.Draw(99999, time.Now()) == nil {
		t.Fatal("invalid draw accepted")
	}
}
func TestNativeMathAndTables(t *testing.T) {
	a := testApp(t)
	if len(a.Rich.Manifest) != 1675 {
		t.Fatal(len(a.Rich.Manifest))
	}
	for tex := range a.Rich.Manifest {
		svg, _, ok := a.Rich.svg(tex)
		if !ok || svg == nil {
			t.Fatal("bad SVG", tex)
		}
	}
	html := `<table><tr><td>下标</td><td>元素</td></tr><tr><td>1</td><td>\\(x_i\\)</td></tr></table>`
	md := prepareNative(html)
	if strings.Contains(md, "<table>") || !strings.Contains(md, "| 下标 | 元素 |") {
		t.Fatal(md)
	}
	if _, err := a.Rich.image("/assets/relax-1000-27/q-p102-2.jpg"); err != nil {
		t.Fatal(err)
	}
}
func TestDisputedAndMultiBlank(t *testing.T) {
	a := testApp(t)
	for _, q := range a.M.Questions {
		if strings.Contains(q.Errata, "标答存疑") {
			r := a.M.Records[q.ID]
			if Due(q, *r, time.Now()) {
				t.Fatal("disputed key scheduled")
			}
		}
		if len(q.AnswerChoices) > 1 {
			a.M.Round.CurrentID = q.ID
			if _, err := a.M.Submit([]string{"A"}, "independent", false, time.Now()); err == nil {
				t.Fatal("incomplete multi blank graded")
			}
			return
		}
	}
	t.Fatal("missing multi blank")
}

func TestNativeWritingSessionAcrossToolSwitch(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 960)
	if err := tt.Click("本题笔记"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("本题笔记内容"); err != nil {
		t.Fatal(err)
	}
	tt.Type("原生输入：思路与疑问")
	if a.M.Records[a.M.Current().ID].Notes != "原生输入：思路与疑问" {
		t.Fatal("note input not saved")
	}
	if err := tt.Click("草稿纸"); err != nil {
		t.Fatal(err)
	}
	if a.tool != "scratch" {
		t.Fatal("tools not exclusive")
	}
	box, ok := tt.Find("无限草稿画布")
	if !ok || box.H < 300 {
		t.Fatal("writing pane not filling height", box)
	}
	tt.UsePointer(ui.PointerPen, .7)
	tt.Press(box.X+20, box.Y+30)
	tt.Move(box.X+70, box.Y+80)
	tt.Release(box.X+90, box.Y+90)
	tt.UsePointer(ui.PointerMouse, 0)
	r := a.M.Records[a.M.Current().ID]
	if len(a.inkEditor.Snapshot().Strokes) != 1 {
		t.Fatal("native ink stroke not committed")
	}
	if err := tt.Click("本题笔记"); err != nil {
		t.Fatal(err)
	}
	if !a.canvasNotes || len(a.inkEditor.Snapshot().Strokes) != 1 || r.Notes != "原生输入：思路与疑问" {
		t.Fatal("switch lost content")
	}
	a.flushSave()
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil || m.Records[m.Current().ID].Ink != nil || m.Records[m.Current().ID].Notes != r.Notes {
		t.Fatal("writing failed reload", err)
	}
}
func TestFinalDueAnswerStillShowsFeedback(t *testing.T) {
	a := testApp(t)
	for _, r := range a.M.Records {
		r.Wrong = false
	}
	q := a.M.Current()
	r := a.M.Records[q.ID]
	r.Wrong = true
	a.M.Round.Mode = "wrong"
	a.M.Round.WrongIDs = []string{q.ID}
	a.choices = append([]string(nil), q.AnswerChoices...)
	a.submit(false)
	tt := ui.NewTester(a.View, 1440, 960)
	if !tt.HasText("与表内答案一致") || len(a.queue) != 1 {
		t.Fatal("last due answer lost its feedback")
	}
}

func TestReviewRailNavigationAndStableNumbers(t *testing.T) {
	a := testApp(t)
	for _, r := range a.M.Records {
		r.Wrong = false
	}
	ids := []string{a.M.Questions[0].ID, a.M.Questions[1].ID}
	for _, id := range ids {
		a.M.Records[id].Wrong = true
	}
	a.M.Round.Mode = "wrong"
	a.M.Round.CurrentID = ids[0]
	a.M.Round.WrongIDs = append([]string(nil), ids...)
	tt := ui.NewTester(a.View, 1440, 960)
	rail, ok := tt.Find("复习答题卡侧栏")
	if !ok || rail.X < 1000 {
		t.Fatal("answer card is not on the right", rail)
	}
	if err := tt.Click("答题卡 第2题"); err != nil {
		t.Fatal(err)
	}
	if a.M.Round.CurrentID != ids[1] {
		t.Fatal("card did not navigate")
	}
	if err := tt.Click("答题卡 第1题"); err != nil {
		t.Fatal(err)
	}
	a.choices = append([]string(nil), a.M.Current().AnswerChoices...)
	a.submit(false)
	tt = ui.NewTester(a.View, 1440, 960)
	if !slices.Equal(a.queue, ids) || a.dueCount() != 1 {
		t.Fatal("grading renumbered the card or changed schedule", a.queue)
	}
	if err := tt.Click("答题卡 第2题"); err != nil {
		t.Fatal(err)
	}
	restored, err := New(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	ui.NewTester(restored.View, 1440, 960)
	if !slices.Equal(restored.queue, ids) || restored.M.Round.CurrentID != ids[1] {
		t.Fatal("round card/cursor failed reload")
	}
	if err := tt.Click("重新随机排序"); err != nil {
		t.Fatal(err)
	}
	if len(a.queue) != 1 || a.queue[0] != ids[1] {
		t.Fatal("redraw did not filter due first", a.queue)
	}
	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	if len(a.queue) != 2 {
		t.Fatal("all-wrong scope not restored")
	}
}
func TestReviewRailReplacedByWritingPanel(t *testing.T) {
	a := testApp(t)
	a.mode("wrong")
	tt := ui.NewTester(a.View, 1440, 960)
	if err := tt.Click("本题笔记"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("复习答题卡侧栏"); ok {
		t.Fatal("rail and writing pane shown together")
	}
	if err := tt.Click("关闭写作面板"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("复习答题卡侧栏"); !ok {
		t.Fatal("answer card did not return")
	}
}

func TestReviewLayoutDesktopBounds(t *testing.T) {
	for _, size := range [][2]int{{1440, 900}, {1060, 720}} {
		a := testApp(t)
		a.mode("wrong")
		tt := ui.NewTester(a.View, size[0], size[1])
		rail, ok := tt.Find("复习答题卡侧栏")
		if !ok {
			t.Fatal("missing rail")
		}
		card, ok := tt.Find("错题题目区域")
		if !ok {
			t.Fatal("missing question")
		}
		if card.X < 240 || card.X+card.W > rail.X || rail.X+rail.W > float32(size[0])+.5 {
			t.Fatalf("three columns overlap at %v: card %+v rail %+v", size, card, rail)
		}
		submit, ok := tt.Find("提交答案")
		if !ok || submit.Y+submit.H > float32(size[1]) {
			t.Fatal("submit clipped", size, submit)
		}
		if err := tt.Click("答题卡 第1题"); err != nil {
			t.Fatal(size, err)
		}
	}
}
func TestReviewKeyboardAndUnchangedSource(t *testing.T) {
	a := testApp(t)
	for _, r := range a.M.Records {
		r.Wrong = false
	}
	q := a.M.Questions[0]
	a.M.Records[q.ID].Wrong = true
	a.M.Round.Mode = "wrong"
	a.M.Round.CurrentID = q.ID
	a.M.Round.WrongIDs = []string{q.ID}
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Key(0, ui.Key1)
	if a.choices[0] != "A" {
		t.Fatal("number shortcut did not choose")
	}
	tt.Key(0, ui.KeyEnter)
	if !a.answered || !a.correct {
		t.Fatal("enter did not grade")
	}
	if a.M.Questions[0].Answer != q.Answer || a.M.Questions[0].History.Result != q.History.Result || a.M.Questions[0].Explanation != q.Explanation {
		t.Fatal("review modified original evidence")
	}
}

func TestExactReviewRetainsRecallActionsInNativeMenu(t *testing.T) {
	a := testApp(t)
	a.mode("wrong")
	tt := ui.NewTester(a.View, 1440, 900)
	box, ok := tt.Find("错题题目区域")
	if !ok {
		t.Fatal("missing question")
	}
	tt.PressButton(1, box.X+4, box.Y+box.H/2)
	tt.ReleaseButton(1, box.X+4, box.Y+box.H/2)
	if err := tt.ChooseMenuItem("不会，先看解析"); err != nil {
		t.Fatal(err)
	}
	if !a.answered || !a.unknown {
		t.Fatal("native unknown action was lost")
	}
}
func TestReviewHTMLGeometry(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.mode("wrong")
	id := "relax-ds-03-060"
	a.M.Round.CurrentID = id
	a.M.Round.WrongIDs = append([]string{id}, a.M.Round.WrongIDs...)
	a.resetAnswer()
	tt := ui.NewTester(a.View, 1440, 900)
	want := map[string]ui.Rect{"错题题目区域": {X: 280, Y: 88, W: 788, H: 536.25}, "复习答题卡侧栏": {X: 1100, Y: 88, W: 300, H: 788}, "题目目录": {X: 1318, Y: 9.5, W: 82, H: 36}, "提交答案": {X: 944, Y: 828, W: 124, H: 48}, "第1空 选项A": {X: 313, Y: 303.25, W: 722, H: 64}, "第1空 选项B": {X: 313, Y: 379.25, W: 722, H: 64}, "草稿纸": {X: 787, Y: 117, W: 66, H: 36}, "本题笔记": {X: 857, Y: 117, W: 80, H: 36}, "移出错题本": {X: 941, Y: 117, W: 94, H: 36}, "重新随机排序": {X: 1299, Y: 291, W: 80, H: 32}}
	for label, expected := range want {
		got, ok := tt.Find(label)
		if !ok || math.Abs(float64(got.X-expected.X)) > .1 || math.Abs(float64(got.Y-expected.Y)) > .1 || math.Abs(float64(got.W-expected.W)) > .1 || math.Abs(float64(got.H-expected.H)) > .1 {
			t.Fatalf("HTML geometry mismatch for %s: got %+v want %+v", label, got, expected)
		}
	}
}
