package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestStemItemFormatting(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"描述（）。 I. 第一项 II. 第二项 III. 第三项 IV. 第四项", "描述（）。\n\nI. 第一项\n\nII. 第二项\n\nIII. 第三项\n\nIV. 第四项"},
		{"描述：I．第一项 II．第二项 III.第三项", "描述：\n\nI．第一项\n\nII．第二项\n\nIII.第三项"},
		{"描述：Ⅰ第一项 Ⅱ第二项 Ⅲ第三项", "描述：\n\nⅠ第一项\n\nⅡ第二项\n\nⅢ第三项"},
		{"描述：①第一项；②第二项；③第三项", "描述：\n\n①第一项；\n\n②第二项；\n\n③第三项"},
		{"描述：(I) 第一项 (II) 第二项", "描述：\n\n(I) 第一项\n\n(II) 第二项"},
	} {
		got := formatStemItems(tc.src)
		if got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
		if formatStemItems(got) != got {
			t.Fatal("formatting not idempotent")
		}
	}
	for _, src := range []string{
		"I/O 软件，IV 型，I、II、III 的组合", "指令 MOV. ADD. IV. 无枚举",
		"`I. code II. code`", "```text\nI. code II. code\n```",
		`公式 \( I. x II. y \) 和 \[ I. x II. y \]`,
		"$I. x II. y$ 和 $$I. x II. y$$",
		`<table><tr><td>I. x II. y</td></tr></table>`,
		"![I. x II. y](/assets/a.png)", "[I. x II. y](https://example.com)",
	} {
		if formatStemItems(src) != src {
			t.Fatal("modified protected/non-list text", src)
		}
	}
}
func TestStemFormattingAcrossSourceBank(t *testing.T) {
	a := testApp(t)
	count := 0
	for _, q := range a.M.Questions {
		before := q.Text
		after := formatStemItems(before)
		if strings.Join(strings.Fields(before), "") != strings.Join(strings.Fields(after), "") {
			t.Fatal("changed source characters", q.ID)
		}
		if after != before {
			count++
		}
		if q.ID == "relax-os-05-035" && strings.Count(after, "\n\n") != 4 {
			t.Fatal("reported question not split into four statements", after)
		}
	}
	t.Logf("formatted %d question stems", count)
	if count < 160 {
		t.Fatal("expected all roman enumeration stems to be covered", count)
	}
}
func TestCaptureStemLayout(t *testing.T) {
	if os.Getenv("CAPTURE_NATIVE_STEM") == "" {
		t.Skip("visual capture")
	}
	a := testApp(t)
	a.mode("wrong")
	a.M.Round.CurrentID = "relax-os-05-035"
	a.resetAnswer()
	for _, theme := range []string{"dark", "light"} {
		a.M.Theme = theme
		img := ui.Render(a.View, 1440, 1000, 1)
		f, err := os.Create("../../native-review/stem-items-" + theme + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
