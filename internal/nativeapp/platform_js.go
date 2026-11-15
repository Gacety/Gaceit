//go:build js

package nativeapp

import (
	"sync"
	"syscall/js"
	"time"
)

const webReportsStorageKey = "study408:question-reports"

var (
	registerWebReportHelper sync.Once
	registerWebFullscreen   sync.Once
	fullscreenWebApp        *App
)

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

func isWebPlatform() bool { return true }

func (a *App) ensureFullscreenListener() {
	fullscreenWebApp = a
	registerWebFullscreen.Do(func() {
		doc := js.Global().Get("document")
		if doc.IsUndefined() || doc.IsNull() {
			return
		}
		onChange := js.FuncOf(func(js.Value, []js.Value) any {
			if fullscreenWebApp != nil && fullscreenWebApp.Window != nil {
				fullscreenWebApp.Window.Invalidate()
			}
			return nil
		})
		doc.Call("addEventListener", "fullscreenchange", onChange)
		doc.Call("addEventListener", "webkitfullscreenchange", onChange)
	})
}

func (a *App) isFullscreen() bool {
	a.ensureFullscreenListener()
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return false
	}
	for _, prop := range []string{"fullscreenElement", "webkitFullscreenElement"} {
		v := doc.Get(prop)
		if !v.IsUndefined() && !v.IsNull() {
			return true
		}
	}
	return false
}

func callFullscreenMethod(target js.Value, names ...string) (js.Value, bool) {
	for _, name := range names {
		if target.Get(name).Type() == js.TypeFunction {
			var res js.Value
			var failed bool
			func() {
				defer func() {
					if recover() != nil {
						failed = true
					}
				}()
				res = target.Call(name)
			}()
			if !failed {
				return res, true
			}
		}
	}
	return js.Undefined(), false
}

func (a *App) toggleFullscreen() {
	a.ensureFullscreenListener()
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return
	}
	attachResult := func(res js.Value) {
		if res.IsUndefined() || res.IsNull() || res.Get("then").Type() != js.TypeFunction {
			return
		}
		var onOK, onErr js.Func
		release := func() {
			onOK.Release()
			onErr.Release()
		}
		onOK = js.FuncOf(func(js.Value, []js.Value) any {
			release()
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return nil
		})
		onErr = js.FuncOf(func(js.Value, []js.Value) any {
			release()
			a.message = "当前浏览器限制全屏，可通过浏览器“添加到主屏幕”获得全屏体验"
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return nil
		})
		res.Call("then", onOK, onErr)
	}
	if a.isFullscreen() {
		if res, ok := callFullscreenMethod(doc, "exitFullscreen", "webkitExitFullscreen"); ok {
			attachResult(res)
		}
		return
	}
	el := doc.Get("documentElement")
	if el.IsUndefined() || el.IsNull() {
		el = doc.Call("getElementById", "mygo")
	}
	if el.IsUndefined() || el.IsNull() {
		a.message = "当前浏览器不支持网页全屏，可通过“添加到主屏幕”全屏使用"
		return
	}
	res, ok := callFullscreenMethod(el, "requestFullscreen", "webkitRequestFullscreen")
	if !ok {
		a.message = "当前浏览器不支持网页全屏，可通过浏览器“添加到主屏幕”全屏使用"
		return
	}
	a.message = ""
	attachResult(res)
}
