package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"slices"
	"testing"
	"time"
)

func TestCaptureHTMLReviewLayout(t *testing.T) {
	if os.Getenv("CAPTURE_NATIVE_REVIEW") == "" {
		t.Skip("manual visual capture")
	}
	a := testApp(t)
	a.mode("wrong")
	a.M.Theme = "dark"
	id := "relax-ds-03-060"
	a.M.Round.WrongIDs = append([]string{id}, a.M.Round.WrongIDs...)
	a.M.Round.CurrentID = id
	a.resetAnswer()
	a.choices[0] = "B"
	for _, mode := range []string{"dark", "light"} {
		a.M.Theme = mode
		img := ui.Render(a.View, 1440, 900, 1)
		f, err := os.Create("../../native-review/review-html-" + mode + ".png")
		if err != nil {
			t.Fatal(err)
		}
		_ = png.Encode(f, img)
		f.Close()
	}
	a.M.Theme = "dark"
	tt := ui.NewTester(a.View, 1440, 900)
	labels := []string{"错题题目区域", "复习答题卡侧栏", "上一题", "下一题", "提交答案", "第1空 选项A", "第1空 选项B", "题目目录"}
	for _, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal("missing", label)
		}
		t.Logf("%s: x=%.2f y=%.2f w=%.2f h=%.2f", label, r.X, r.Y, r.W, r.H)
	}
	if !slices.Contains(a.queue, id) {
		t.Fatal("missing source question")
	}
}

// One bounded visual pass for the three supplied HTML boards. Sample note text
// exists only in this test's temporary store, never in the user's library.
func TestCaptureLibraryBoards(t *testing.T) {
	if os.Getenv("CAPTURE_NATIVE_REVIEW") == "" {
		t.Skip("manual visual capture")
	}
	a := testApp(t)
	a.M.Theme = "dark"
	noteSamples := []struct{ subject, text string }{
		{"数据结构", "概念不清\n\n**错因：** 没有先画出队列状态，就凭印象选了答案。\n\n**下次：** 先模拟一遍，再选，别凭印象。"},
		{"操作系统", "计算失误\n\n时间片轮转：先看到达顺序，再看时间片耗尽后的位置。"},
		{"计算机网络", "概念不清\n\n三次握手的第三次可以携带数据，容易记反。"},
		{"计算机组成原理", "直接映射和组相联的地址划分，自己列表推一遍。"},
	}
	for _, sample := range noteSamples {
		for _, q := range a.M.Questions {
			if q.Subject == sample.subject {
				a.M.Records[q.ID].Notes = sample.text
				break
			}
		}
	}
	for _, mode := range []string{"practice", "random", "notes"} {
		a.mode(mode)
		a.studyDetail = false
		if mode == "random" {
			a.M.Round.Settings.Subjects = []string{"数据结构", "计算机组成原理"}
			a.M.Round.Settings.Range = "unpracticed"
		}
		img := ui.Render(a.View, 1440, 900, 1)
		f, err := os.Create("../../native-review/" + mode + "-html-dark.png")
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	a.M.Theme = "light"
	img := ui.Render(a.View, 1440, 900, 1)
	f, err := os.Create("../../native-review/notes-html-light.png")
	if err != nil {
		t.Fatal(err)
	}
	_ = png.Encode(f, img)
	f.Close()
	// Confirm that the primary setup action starts a real, unique round.
	a.mode("random")
	a.studyDetail = false
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Click("开始随机练习")
	if !a.studyDetail || len(a.M.Round.RandomIDs) != 20 {
		t.Fatal("random setup did not open a round")
	}
}

func TestCaptureUnifiedReaders(t *testing.T) {
	if os.Getenv("CAPTURE_NATIVE_REVIEW") == "" {
		t.Skip("manual visual capture")
	}
	a := testApp(t)
	a.M.Theme = "dark"
	a.subject = "计算机网络"
	a.chapter = "绪论"
	a.open("relax-cn-01-001")
	for _, mode := range []string{"practice", "random"} {
		if mode == "random" {
			a.mode("random")
			a.M.Round.Settings.Subject = "计算机网络"
			a.M.Round.Settings.Subjects = nil
			if err := a.M.Draw(20, time.Now()); err != nil {
				t.Fatal(err)
			}
			a.open(a.M.Round.RandomCurrentID)
		}
		before := slices.Clone(a.M.Round.RandomIDs)
		tt := ui.NewTester(a.View, 1440, 900)
		for _, label := range []string{a.readerAreaLabel(), a.readerRailLabel(), "提交答案", "草稿纸", "本题笔记", "答题卡 第2题"} {
			if _, ok := tt.Find(label); !ok {
				t.Fatal("missing", label)
			}
		}
		first := a.M.Round.CurrentID
		attempts := len(a.M.Records[first].Attempts)
		if err := tt.Click("答题卡 第2题"); err != nil {
			t.Fatal(err)
		}
		if a.M.Round.CurrentID == first || len(a.M.Records[first].Attempts) != attempts {
			t.Fatal("card navigation changed attempts or did not navigate")
		}
		if mode == "random" && !slices.Equal(before, a.M.Round.RandomIDs) {
			t.Fatal("card navigation redrew round")
		}
		a.open(first)
		img := ui.Render(a.View, 1440, 900, 1)
		f, err := os.Create("../../native-review/" + mode + "-reader-dark.png")
		if err != nil {
			t.Fatal(err)
		}
		_ = png.Encode(f, img)
		f.Close()
	}
	a.M.Theme = "light"
	img := ui.Render(a.View, 1440, 900, 1)
	f, err := os.Create("../../native-review/random-reader-light.png")
	if err != nil {
		t.Fatal(err)
	}
	_ = png.Encode(f, img)
	f.Close()
}
