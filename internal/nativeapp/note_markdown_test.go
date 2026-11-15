package nativeapp

import (
	"bytes"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

func noteTestImage(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			im.SetRGBA(x, y, color.RGBA{230, 50, 80, 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, im); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
func TestNoteImagePasteAtCaretAndPortableReload(t *testing.T) {
	for _, canvas := range []bool{false, true} {
		name := "reader"
		if canvas {
			name = "canvas"
		}
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			id := a.M.Current().ID
			r := a.M.Records[id]
			r.Notes = "前后"
			if canvas {
				a.SetTool("scratch")
				a.canvasNotes = true
			} else {
				a.SetTool("notes")
			}
			tt := ui.NewTester(a.View, 1440, 900)
			if err := tt.Click("本题笔记内容"); err != nil {
				t.Fatal(err)
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyRight)
			tt.Key(0, ui.KeyLeft)
			data := noteTestImage(t)
			tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, data))))
			tt.Key(ui.Cmd, ui.KeyV)
			tt.Frame()
			if !strings.HasPrefix(r.Notes, "前\n\n![笔记图片](/note-images/") || !strings.HasSuffix(r.Notes, "\n\n后") {
				t.Fatal("image not inserted at Unicode caret", r.Notes)
			}
			if len(r.NoteImages) != 1 {
				t.Fatal("image not attached")
			}
			if err := tt.Click("切换笔记预览"); err != nil {
				t.Fatal(err)
			}
			if _, ok := tt.Find("笔记Markdown预览"); !ok {
				t.Fatal("preview missing")
			}
			// Resolve the actual Markdown image through the native renderer.
			start := strings.Index(r.Notes, "/note-images/")
			end := strings.Index(r.Notes[start:], ")")
			if _, err := a.Rich.image(r.Notes[start : start+end]); err != nil {
				t.Fatal("attached image did not render", err)
			}
			a.flushSave()
			b, err := New(assetsForTest(), a.M.Disk)
			if err != nil {
				t.Fatal(err)
			}
			if b.M.Records[id].Notes != r.Notes || len(b.M.Records[id].NoteImages) != 1 {
				t.Fatal("image/Markdown did not survive reload")
			}
			if _, err := b.Rich.image(r.Notes[start : start+end]); err != nil {
				t.Fatal("reloaded image failed", err)
			}
			copy := a.M.saveCopy()
			r.NoteImages["later"] = "change"
			if _, ok := copy.Records[id].NoteImages["later"]; ok {
				t.Fatal("worker shares mutable attachments map")
			}
		})
	}
}
func TestNotePlainTextPasteAndMarkdown(t *testing.T) {
	a := testApp(t)
	a.SetTool("notes")
	tt := ui.NewTester(a.View, 1440, 900)
	if err := tt.Click("本题笔记内容"); err != nil {
		t.Fatal(err)
	}
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("# 标题\n\n**重点**\n\n- 一\n- 二")))))
	tt.Key(ui.Cmd, ui.KeyV)
	if err := tt.Click("切换笔记预览"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("标题"); !ok {
		t.Fatal("Markdown heading not rendered")
	}
	if !strings.Contains(a.M.Records[a.M.Current().ID].Notes, "**重点**") {
		t.Fatal("preview changed source")
	}
}

func TestCaptureNoteMarkdown(t *testing.T) {
	if os.Getenv("CAPTURE_NOTE_MARKDOWN") == "" {
		t.Skip("manual visual capture")
	}
	a := testApp(t)
	a.SetTool("scratch")
	a.canvasNotes = true
	id := a.M.Current().ID
	md, err := a.attachNoteImage(id, noteTestImage(t))
	if err != nil {
		t.Fatal(err)
	}
	a.M.Records[id].Notes = "# 复习笔记\n\n**关键步骤**\n\n- 先分析边界\n- 再验证结果\n" + md
	a.notePreviews = map[string]bool{id: true}
	for _, theme := range []string{"light", "dark"} {
		a.M.Theme = theme
		tt := ui.NewTester(a.View, 1440, 900)
		f, err := os.Create("../../native-review/note-markdown-" + theme + ".png")
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
