//go:build js

package nativeapp

import (
	"sync"
	"syscall/js"
	"time"
)

const webReportsStorageKey = "study408:question-reports"

var registerWebReportHelper sync.Once

func triggerBrowserDownload(filename string, data []byte) {
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return
	}
	u8 := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(u8, data)
	parts := js.Global().Get("Array").New(u8)
	blob := js.Global().Get("Blob").New(parts, map[string]any{"type": "application/json;charset=utf-8"})
	urlObj := js.Global().Get("URL")
	href := urlObj.Call("createObjectURL", blob).String()
	link := doc.Call("createElement", "a")
	link.Set("href", href)
	link.Set("download", filename)
	link.Get("style").Set("display", "none")
	body := doc.Get("body")
	body.Call("appendChild", link)
	link.Call("click")
	body.Call("removeChild", link)
	var revoke js.Func
	revoke = js.FuncOf(func(js.Value, []js.Value) any {
		defer revoke.Release()
		urlObj.Call("revokeObjectURL", href)
		return nil
	})
	js.Global().Call("setTimeout", revoke, 2000)
}

func (a *App) exportBackup() {
	a.flushInk()
	a.collectSave(true)
	if err := a.M.Save(); err != nil {
		a.message = err.Error()
	}
	data, err := a.M.ExportBackupData()
	if err != nil {
		a.message = "导出失败：" + err.Error()
		return
	}
	triggerBrowserDownload("408-研习-"+time.Now().Format("2006-01-02")+".json", data)
	a.message = ""
}

func (a *App) importBackup() {
	a.closeCanvas()
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return
	}
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".json,application/json")
	input.Get("style").Set("display", "none")
	doc.Get("body").Call("appendChild", input)

	var onChange js.Func
	cleanup := func() {
		input.Call("remove")
		onChange.Release()
	}
	onChange = js.FuncOf(func(js.Value, []js.Value) any {
		files := input.Get("files")
		if files.IsUndefined() || files.Get("length").Int() == 0 {
			cleanup()
			return nil
		}
		file := files.Call("item", 0)
		var onLoad, onFail js.Func
		done := func() {
			onLoad.Release()
			onFail.Release()
			cleanup()
		}
		onLoad = js.FuncOf(func(_ js.Value, args []js.Value) any {
			defer done()
			if len(args) == 0 {
				return nil
			}
			raw := []byte(args[0].String())
			trial, err := a.M.ImportBackupData(raw)
			if err != nil {
				a.message = err.Error()
				if a.Window != nil {
					a.Window.Invalidate()
				}
				return nil
			}
			if !js.Global().Call("confirm", "导入会覆盖当前浏览器中的学习记录。\n\n确定继续导入吗？").Bool() {
				SetReviewConfig(a.M.ReviewConfig)
				return nil
			}
			a.M = trial
			a.inkQuestion = ""
			a.inkDirty = false
			a.resetAnswer()
			a.save()
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return nil
		})
		onFail = js.FuncOf(func(js.Value, []js.Value) any {
			defer done()
			a.message = "读取备份文件失败"
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return nil
		})
		file.Call("text").Call("then", onLoad, onFail)
		return nil
	})
	input.Call("addEventListener", "change", onChange)
	input.Call("click")
}

func (m *Model) syncQuestionReportsFile() error {
	data, err := m.ExportQuestionReportsJSON()
	if err != nil {
		return err
	}
	text := string(data)
	func() {
		defer func() { _ = recover() }()
		storage := js.Global().Get("localStorage")
		if !storage.IsUndefined() && !storage.IsNull() {
			storage.Call("setItem", webReportsStorageKey, text)
		}
		registerWebReportHelper.Do(func() {
			js.Global().Set("study408GetQuestionReports", js.FuncOf(func(js.Value, []js.Value) any {
				s := js.Global().Get("localStorage")
				if s.IsUndefined() || s.IsNull() {
					return ""
				}
				v := s.Call("getItem", webReportsStorageKey)
				if v.Type() == js.TypeString {
					return v.String()
				}
				return ""
			}))
		})
	}()
	return nil
}

func (a *App) exportQuestionReports() {
	a.collectSave(true)
	data, err := a.M.ExportQuestionReportsJSON()
	if err != nil {
		a.message = "导出报错失败：" + err.Error()
		return
	}
	triggerBrowserDownload(questionReportsFilename, data)
	a.message = ""
}

func sanitizeInitialQuestions(questions []Question) {
	for i := range questions {
		questions[i].History.Chosen = ""
		questions[i].History.Result = ""
		questions[i].History.InWrongBook = false
	}
}
