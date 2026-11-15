//go:build !js

package nativeapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

func (a *App) exportBackup() {
	a.flushInk()
	if a.Window == nil {
		return
	}
	a.collectSave(true)
	if err := a.M.Save(); err != nil {
		a.message = err.Error()
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.Window, Title: "导出学习备份", DefaultPath: "408-研习-" + time.Now().Format("2006-01-02") + ".json"})
	if err != nil || path == "" {
		return
	}
	data, err := a.M.ExportBackupData()
	if err != nil {
		a.message = "导出失败：" + err.Error()
		return
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		a.message = "导出失败：" + err.Error()
	} else {
		a.message = "已导出备份"
	}
}

func (a *App) importBackup() {
	a.closeCanvas()
	if a.Window == nil {
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.Window, Title: "导入 408 学习备份"})
	if err != nil || len(paths) == 0 {
		return
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		a.message = err.Error()
		return
	}
	trial, err := a.M.ImportBackupData(data)
	if err != nil {
		a.message = err.Error()
		return
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: a.Window, Type: mygo.MessageQuestion, Message: "导入会覆盖同名学习记录", Detail: "将自动保留导入前的文件备份。", Buttons: []string{"取消", "导入"}})
	if err != nil || res.Button != 1 {
		SetReviewConfig(a.M.ReviewConfig)
		return
	}
	if a.M.Disk != nil {
		old, _ := json.MarshalIndent(a.M.snapshot, "", "  ")
		path := filepath.Join(filepath.Dir(a.M.Disk.Path), fmt.Sprintf("progress-before-import-%d.json", time.Now().Unix()))
		if err = os.WriteFile(path, old, 0600); err != nil {
			a.message = err.Error()
			return
		}
	}
	a.M = trial
	a.inkQuestion = ""
	a.inkDirty = false
	a.resetAnswer()
	a.save()
}

func (m *Model) syncQuestionReportsFile() error {
	path := m.QuestionReportsFilePath()
	if path == "" {
		return nil
	}
	if len(m.Reports) == 0 {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}
	data, err := m.ExportQuestionReportsJSON()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".question-reports-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *App) exportQuestionReports() {
	if a.Window == nil {
		return
	}
	a.collectSave(true)
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Window,
		Title:       "导出题目报错 JSON",
		DefaultPath: questionReportsFilename,
	})
	if err != nil || path == "" {
		return
	}
	data, err := a.M.ExportQuestionReportsJSON()
	if err != nil {
		a.message = "导出报错失败：" + err.Error()
		return
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		a.message = "导出报错失败：" + err.Error()
		return
	}
	a.message = ""
}

func sanitizeInitialQuestions([]Question) {}
