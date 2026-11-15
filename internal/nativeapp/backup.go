package nativeapp

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

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
