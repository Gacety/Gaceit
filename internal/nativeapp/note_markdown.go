package nativeapp

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"image/png"
	"strings"
	"time"
)

const noteImageLimit = 8 << 20

func validateNoteImage(data []byte) error {
	if len(data) == 0 || len(data) > noteImageLimit {
		return fmt.Errorf("图片不能超过 8 MB")
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("无法读取剪贴板图片")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 20_000_000 {
		return fmt.Errorf("图片尺寸过大，请先裁剪")
	}
	return nil
}
func (a *App) attachNoteImage(id string, data []byte) (string, error) {
	if err := validateNoteImage(data); err != nil {
		return "", err
	}
	r := a.M.Records[id]
	if r == nil {
		return "", fmt.Errorf("题目不存在")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if r.NoteImages == nil {
		r.NoteImages = map[string]string{}
	}
	r.NoteImages[hash] = base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("\n\n![笔记图片](/note-images/%s/%s.png)\n\n", id, hash), nil
}
func (a *App) loadNoteImage(url string) ([]byte, error) {
	path := strings.TrimPrefix(url, "/note-images/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".png") {
		return nil, fmt.Errorf("无效的笔记图片地址")
	}
	r := a.M.Records[parts[0]]
	if r == nil {
		return nil, fmt.Errorf("找不到笔记图片")
	}
	encoded, ok := r.NoteImages[strings.TrimSuffix(parts[1], ".png")]
	if !ok {
		return nil, fmt.Errorf("找不到笔记图片")
	}
	return base64.StdEncoding.DecodeString(encoded)
}
func (a *App) pasteNoteImage(id, key string, services ui.Services) bool {
	data, err := services.ReadClipboardData(transfer.PNG).Read(transfer.PNG)
	if err != nil || len(data) == 0 {
		return false
	}
	md, err := a.attachNoteImage(id, data)
	if err != nil {
		a.noteImageError = err.Error()
		return true
	}
	if a.noteImagePastes == nil {
		a.noteImagePastes = map[string]string{}
	}
	a.noteImagePastes[key] += md
	a.noteImageError = ""
	return true
}

// OnPaste defers insertion until the native text field has published pending
// edits. Selection is read from the CURRENT element, never a stale UI handle.
func (a *App) noteEditor(c *ui.Context, key, id, label string) ui.Element {
	r := a.M.Records[id]
	input := ui.TextArea(c.Key(key), &r.Notes).Label(label).FillWidth()
	services := c.Services()
	input.OnPaste(func(_ string) bool { return a.pasteNoteImage(id, key, services) })
	if caret, ok := a.noteImageCarets[key]; ok {
		input.SetTextSelection(caret, caret)
		delete(a.noteImageCarets, key)
	}
	if md := a.noteImagePastes[key]; md != "" {
		start, end := input.TextSelection()
		runes := []rune(r.Notes)
		start = max(0, min(start, len(runes)))
		end = max(start, min(end, len(runes)))
		r.Notes = string(runes[:start]) + md + string(runes[end:])
		if a.noteImageCarets == nil {
			a.noteImageCarets = map[string]int{}
		}
		a.noteImageCarets[key] = start + len([]rune(md))
		delete(a.noteImagePastes, key)
		a.saveSoon()
		c.After(time.Millisecond)
	}
	if input.Changed() {
		a.saveSoon()
	}
	if a.noteImageError != "" {
		ui.Text(c, a.noteImageError).FontSize(12).TextColor(c.Theme().Danger)
	}
	return input
}
func (a *App) noteMode(c *ui.Context, id string) bool {
	if a.notePreviews == nil {
		a.notePreviews = map[string]bool{}
	}
	preview := a.notePreviews[id]
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Markdown · ⌘V 粘贴图片").FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
		label := "预览"
		if preview {
			label = "编辑"
		}
		if a.exactButton(c, label).Label("切换笔记预览").Height(28).Clicked() {
			preview = !preview
			a.notePreviews[id] = preview
		}
	})
	return preview
}

func validateNoteImages(r *Record) error {
	for hash, encoded := range r.NoteImages {
		if len(hash) != 64 || len(encoded) > base64.StdEncoding.EncodedLen(noteImageLimit) {
			return fmt.Errorf("无效的笔记图片")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("笔记图片数据损坏")
		}
		if err = validateNoteImage(data); err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != hash {
			return fmt.Errorf("笔记图片校验失败")
		}
	}
	return nil
}
