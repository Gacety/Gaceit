package nativeapp

import (
	"encoding/json"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

const inkClipboardFormat transfer.Format = "application/x-study408-ink+json"

type inkClipboard struct {
	Version int          `json:"version"`
	Strokes []ink.Stroke `json:"strokes"`
	Bounds  [4]float32   `json:"bounds"`
}

func (a *App) copyInk(s ui.Services) bool {
	if a.inkEditor == nil || !a.inkEditor.HasSelection() {
		return false
	}
	strokes := a.inkEditor.CopySelection()
	_, bounds := a.inkEditor.Selected()
	data, err := json.Marshal(inkClipboard{Version: 1, Strokes: strokes, Bounds: bounds})
	if err != nil {
		a.message = "复制笔迹失败：" + err.Error()
		return false
	}
	s.WriteClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(inkClipboardFormat, data))))
	a.inkPasteSerial = 0
	return true
}
func (a *App) pasteInk(s ui.Services) bool {
	if a.inkEditor == nil || a.inkEditor.Busy() {
		return false
	}
	data, err := s.ReadClipboardData(inkClipboardFormat).Read(inkClipboardFormat)
	if err != nil || len(data) == 0 {
		a.message = "剪贴板中没有可粘贴的笔迹"
		return false
	}
	var copied inkClipboard
	if len(data) > 16<<20 || json.Unmarshal(data, &copied) != nil || copied.Version != 1 || len(copied.Strokes) == 0 || !validInkStrokes(copied.Strokes) {
		a.message = "剪贴板中的笔迹格式无效"
		return false
	}
	for _, v := range copied.Bounds {
		if !finite(float64(v)) {
			a.message = "剪贴板中的选区无效"
			return false
		}
	}
	a.inkPasteSerial++
	center := a.inkEditor.PasteCenter(copied.Bounds, float32(a.inkPasteSerial*24))
	if !a.inkEditor.Paste(copied.Strokes, center) {
		return false
	}
	a.message = ""
	a.inkChanged()
	return true
}
func (a *App) deleteInk() bool {
	if a.inkEditor == nil || !a.inkEditor.DeleteSelection() {
		return false
	}
	a.inkChanged()
	return true
}
func (a *App) canvasEditCommand(command string, s ui.Services) bool {
	if a.canvasNotes || a.canvasTextInteraction || a.inkEditor == nil || a.inkEditor.Busy() {
		return false
	}
	switch command {
	case "copy":
		return a.copyInk(s)
	case "paste":
		return a.pasteInk(s)
	case "delete":
		return a.deleteInk()
	}
	return false
}
func (a *App) canvasSelectionShortcuts(c *ui.Context) {
	if a.canvasNotes || a.canvasTextInteraction || a.inkEditor == nil || a.inkEditor.Busy() {
		return
	}
	s := c.Services()
	if a.inkEditor.HasSelection() && c.Shortcut(ui.Cmd, ui.KeyC) && !c.ShortcutRepeated() {
		a.copyInk(s)
	}
	if c.Shortcut(ui.Cmd, ui.KeyV) && !c.ShortcutRepeated() {
		a.pasteInk(s)
	}
	if a.inkEditor.HasSelection() {
		if c.Shortcut(0, ui.KeyBackspace) || c.Shortcut(0, ui.KeyDelete) {
			a.deleteInk()
		}
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) && !c.ShortcutRepeated() {
		a.inkEditor.Redo()
		a.inkChanged()
	}
}
func (a *App) canvasSelectionMenu(m *ui.Menu, s ui.Services) {
	if m.Item("复制笔迹").Shortcut(ui.Cmd, ui.KeyC).Disabled(!a.inkEditor.HasSelection()).Chosen() {
		a.copyInk(s)
	}
	if m.Item("粘贴笔迹").Shortcut(ui.Cmd, ui.KeyV).Disabled(a.inkEditor.Busy()).Chosen() {
		a.pasteInk(s)
	}
	if m.Item("删除选中笔迹").Shortcut(0, ui.KeyBackspace).Disabled(!a.inkEditor.HasSelection()).Chosen() {
		a.deleteInk()
	}
	m.Separator()
	if m.Item("存入本题笔记").Disabled(!a.inkEditor.HasSelection()).Chosen() {
		a.saveInkSelection()
	}
}
