package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"io/fs"
	"path/filepath"
	"study408/internal/store"
	"testing"
	"time"
)

func performanceApp(b *testing.B) *App {
	b.Helper()
	font, _ := fs.ReadFile(assetsForTest(), "prototype/public/fonts/NotoSansSC-Bold.otf")
	ui.RegisterFont(font, "Study Noto")
	font, _ = fs.ReadFile(assetsForTest(), "native-assets/fonts/NotoSansSC-Variable.ttf")
	ui.RegisterFont(font, "Review Noto")
	a, err := New(assetsForTest(), &store.Store{Path: filepath.Join(b.TempDir(), "progress.json")})
	if err != nil {
		b.Fatal(err)
	}
	a.studyDetail = true
	return a
}
func BenchmarkPracticeQueue(b *testing.B) {
	a := performanceApp(b)
	now := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.M.Queue("全部学科", "全部章节", now)
	}
}
func BenchmarkWrongQueue(b *testing.B) {
	a := performanceApp(b)
	a.M.Round.Mode = "wrong"
	a.M.Round.WrongScope = "all"
	now := time.Now()
	a.M.Queue("全部学科", "全部章节", now)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.M.Queue("全部学科", "全部章节", now)
	}
}
func BenchmarkCanvasIdleFrame(b *testing.B) {
	a := performanceApp(b)
	a.SetTool("scratch")
	tt := ui.NewTester(a.View, 1440, 900)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}
func BenchmarkReaderIdleFrame(b *testing.B) {
	a := performanceApp(b)
	tt := ui.NewTester(a.View, 1440, 900)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}
func BenchmarkNoteTyping(b *testing.B) {
	a := performanceApp(b)
	a.SetTool("notes")
	tt := ui.NewTester(a.View, 1440, 900)
	if err := tt.Click("本题笔记内容"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Type("a")
	}
}
