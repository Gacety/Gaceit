package nativeapp

import (
	"encoding/json"
	"maps"
	"slices"
)

// Only immutable vector samples are shared with the worker. All maps and
// mutable record metadata are detached on the UI thread before it starts.
func (m *Model) saveCopy() *Model {
	out := *m
	out.Subjects = slices.Clone(m.Subjects)
	out.ReviewConfig = cloneReviewConfig(m.ReviewConfig)
	out.Reports = cloneQuestionReports(m.Reports)
	out.Study.Passes = slices.Clone(m.Study.Passes)
	for i := range out.Study.Passes {
		out.Study.Passes[i].IDs = slices.Clone(m.Study.Passes[i].IDs)
		out.Study.Passes[i].PriorWrong = slices.Clone(m.Study.Passes[i].PriorWrong)
	}
	out.Study.Workspaces = maps.Clone(m.Study.Workspaces)
	for id, w := range out.Study.Workspaces {
		w.RandomIDs = slices.Clone(w.RandomIDs)
		out.Study.Workspaces[id] = w
	}
	out.snapshot = make(map[string]string, len(m.snapshot))
	for k, v := range m.snapshot {
		out.snapshot[k] = v
	}
	out.rawRecords = make(map[string]map[string]json.RawMessage, len(m.rawRecords))
	for id, fields := range m.rawRecords {
		copy := make(map[string]json.RawMessage, len(fields))
		for k, v := range fields {
			copy[k] = v
		}
		out.rawRecords[id] = copy
	}
	out.Records = make(map[string]*Record, len(m.Records))
	for id, r := range m.Records {
		copy := *r
		copy.NoteImages = maps.Clone(r.NoteImages)
		copy.NoteTags = slices.Clone(r.NoteTags)
		copy.Excluded = slices.Clone(r.Excluded)
		copy.Strokes = slices.Clone(r.Strokes)
		copy.InkClips = slices.Clone(r.InkClips)
		if r.Ink != nil {
			d := *r.Ink
			d.Strokes = slices.Clone(d.Strokes)
			copy.Ink = &d
		}
		if r.Review != nil {
			review := *r.Review
			review.Weights = slices.Clone(r.Review.Weights)
			copy.Review = &review
		}
		copy.Attempts = slices.Clone(r.Attempts)
		out.Records[id] = &copy
	}
	out.Round.WrongIDs = slices.Clone(m.Round.WrongIDs)
	out.Round.RandomIDs = slices.Clone(m.Round.RandomIDs)
	out.Round.Settings.Subjects = slices.Clone(m.Round.Settings.Subjects)
	if m.Round.ReviewDeck != nil {
		deck := *m.Round.ReviewDeck
		deck.IDs = slices.Clone(deck.IDs)
		out.Round.ReviewDeck = &deck
	}
	return &out
}

type saveResult struct {
	source, copy *Model
	err          error
}

func (a *App) collectSave(wait bool) {
	if a.saveWorker == nil {
		return
	}
	var result saveResult
	if wait {
		result = <-a.saveWorker
	} else {
		select {
		case result = <-a.saveWorker:
		default:
			return
		}
	}
	a.saveWorker = nil
	if a.M != result.source {
		return
	}
	if result.err != nil {
		a.M.SaveError = result.err.Error()
		a.message = "保存失败，请导出备份：" + result.err.Error()
		a.saveSoon()
		return
	}
	a.M.snapshot = result.copy.snapshot
	a.M.rawRecords = result.copy.rawRecords
	a.M.SaveError = ""
}
func (a *App) startSave() {
	if a.saveWorker != nil {
		return
	}
	source := a.M
	copy := source.saveCopy()
	result := make(chan saveResult, 1)
	a.saveWorker = result
	a.savePending = false
	window := a.Window
	go func() {
		err := copy.Save()
		result <- saveResult{source: source, copy: copy, err: err}
		if window != nil {
			window.Invalidate()
		}
	}()
}
