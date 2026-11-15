package main

import (
	"github.com/egoist/mygo/ui"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"study408/internal/ink"
	"testing"
)

// Optional native-UI render fixtures for visual review; no live window needed.
func TestToolbarReviewCaptures(t *testing.T) {
	dir := os.Getenv("INK_REVIEW_DIR")
	if dir == "" {
		t.Skip("set INK_REVIEW_DIR to capture UI states")
	}
	r, err := ink.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	a := &app{renderer: r, width: 8, ratio: 1, showTools: true, strokes: fixtures()}
	tt := ui.NewTester(a.view, 960, 720)
	save := func(name string) {
		t.Helper()
		f, e := os.Create(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, tt.Image()); e != nil {
			t.Fatal(e)
		}
	}
	save("ink-tools-desktop.png")
	save("ink-infinite-desktop.png")
	save("ink-phosphor-desktop.png")
	crop, err := os.Create(filepath.Join(dir, "ink-phosphor-toolbar.png"))
	if err != nil {
		t.Fatal(err)
	}
	frame := tt.Image()
	bounds := image.Rect(int(a.panelX), int(a.panelY), int(a.panelX+paletteWidth), int(a.panelY+paletteHeight))
	if err = png.Encode(crop, frame.SubImage(bounds)); err != nil {
		t.Fatal(err)
	}
	crop.Close()
	if err := tt.Click("收纳工具栏"); err != nil {
		t.Fatal(err)
	}
	save("ink-infinite-collapsed.png")
	save("ink-phosphor-collapsed.png")
	if err := tt.Click("展开工具栏"); err != nil {
		t.Fatal(err)
	}
	tt.Pinch(200, 300, 1.75)
	save("ink-infinite-zoomed.png")
	if err := tt.Click("回到原点"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("朱红"); err != nil {
		t.Fatal(err)
	}
	save("ink-tools-color-red.png")
	if err := tt.Click("天蓝"); err != nil {
		t.Fatal(err)
	}
	save("ink-tools-color-blue.png")
	if err := tt.Click("墨黑"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("普通笔选项"); err != nil {
		t.Fatal(err)
	}
	save("ink-tools-pen-menu.png")
	if err := tt.Click("橡皮擦"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("区域擦除"); err != nil {
		t.Fatal(err)
	}
	save("ink-tools-eraser-menu.png")
	save("ink-unified-eraser-region.png")
	if err := tt.Click("整笔擦除"); err != nil {
		t.Fatal(err)
	}
	save("ink-unified-eraser-stroke.png")
	a.panelDock = -1
	a.panelExpanded = false
	tt.SetSize(640, 520)
	tt.Frame()
	save("ink-tools-compact-left.png")
	save("ink-infinite-compact.png")
	save("ink-phosphor-compact.png")
}
