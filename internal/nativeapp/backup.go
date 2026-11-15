package nativeapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo"
	"study408/internal/store"
)

// ReviewExportState pairs each question's HLR parameters and schedule state
// with its computed next review timestamp for inspection and portable backups.
type ReviewExportState struct {
	Review
	Wrong        bool    `json:"wrong"`
	NextReviewAt *string `json:"nextReviewAt,omitempty"`
}

type BackupEnvelope struct {
	Format          string                       `json:"format"`
	Version         int                          `json:"version"`
	ReviewConfig    *ReviewConfig                `json:"reviewConfig,omitempty"`
	ReviewStates    map[string]ReviewExportState `json:"reviewStates,omitempty"`
	QuestionReports []QuestionReportEntry        `json:"questionReports,omitempty"`
	Data            store.Snapshot               `json:"data"`
}

func (m *Model) ExportBackupData() ([]byte, error) {
	cfg := normalizeReviewConfig(m.ReviewConfig)
	snap := make(store.Snapshot, len(m.snapshot)+1)
	for k, v := range m.snapshot {
		snap[k] = v
	}
	if cfgBytes, err := json.Marshal(cfg); err == nil {
		snap[reviewConfigKey] = string(cfgBytes)
	}
	states := map[string]ReviewExportState{}
	for _, q := range m.Questions {
		r := m.Records[q.ID]
		if r == nil || (r.Review == nil && !r.Wrong && len(r.Attempts) == 0) {
			continue
		}
		state := ReviewState(q, *r)
		if state.HalfLifeDays == nil {
			state.HalfLifeDays = ptr(halfLifeWithConfig(state, cfg))
		}
		entry := ReviewExportState{
			Review: *state,
			Wrong:  r.Wrong,
		}
		if nextAt := nextReviewWithConfig(q, *r, cfg); !nextAt.IsZero() {
			entry.NextReviewAt = ptr(stamp(nextAt))
		}
		states[q.ID] = entry
	}
	if len(m.Reports) > 0 {
		if repBytes, err := json.Marshal(m.Reports); err == nil {
			snap[questionReportsKey] = string(repBytes)
		}
	}
	archive := m.BuildQuestionReportArchive(time.Now())
	env := BackupEnvelope{
		Format:          "408-study-backup",
		Version:         1,
		ReviewConfig:    &cfg,
		ReviewStates:    states,
		QuestionReports: archive.Reports,
		Data:            snap,
	}
	return json.MarshalIndent(env, "", "  ")
}

func (m *Model) ImportBackupData(data []byte) (*Model, error) {
	var backup BackupEnvelope
	if json.Unmarshal(data, &backup) != nil || backup.Format != "408-study-backup" || backup.Version != 1 || backup.Data == nil {
		return nil, fmt.Errorf("这不是有效的 408 学习备份")
	}
	if err := store.Validate(backup.Data); err != nil {
		return nil, err
	}
	trial := *m
	trial.snapshot = store.Snapshot{}
	for k, v := range m.snapshot {
		trial.snapshot[k] = v
	}
	for k, v := range backup.Data {
		trial.snapshot[k] = v
	}
	if backup.ReviewConfig != nil {
		cfg := normalizeReviewConfig(*backup.ReviewConfig)
		if cfgBytes, err := json.Marshal(cfg); err == nil {
			trial.snapshot[reviewConfigKey] = string(cfgBytes)
		}
	}
	trial.rawRecords = map[string]map[string]json.RawMessage{}
	if err := trial.restore(); err != nil {
		SetReviewConfig(m.ReviewConfig)
		return nil, fmt.Errorf("导入取消：%w", err)
	}
	for id, rs := range backup.ReviewStates {
		r := trial.Records[id]
		if r == nil || r.Review != nil {
			continue
		}
		if ValidReview(&rs.Review) {
			cp := rs.Review
			cp.Weights = slices.Clone(rs.Review.Weights)
			r.Review = &cp
		}
	}
	for _, entry := range backup.QuestionReports {
		if entry.QuestionID == "" {
			continue
		}
		if _, exists := trial.Reports[entry.QuestionID]; !exists {
			if trial.Reports == nil {
				trial.Reports = map[string]QuestionReport{}
			}
			rep := entry.QuestionReport
			rep.Categories = slices.Clone(entry.Categories)
			trial.Reports[entry.QuestionID] = rep
		}
	}
	return &trial, nil
}

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
