package nativeapp

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestIndependentReviewLadderAcrossReload(t *testing.T) {
	now := time.Date(2026, 11, 11, 23, 30, 0, 0, zone)
	q := Question{}
	r := Record{Wrong: true}
	for _, want := range []float64{3, 7, 14, 30, 60, 120, 240, 365, 365} {
		state, meta := ApplyReview(q, r, "right", now)
		if meta["qualified"] != true || state.IntervalDays != want || parse(state.DueAt).Sub(now) != time.Duration(want)*day {
			t.Fatalf("wrong ladder: want %v, got %+v", want, state)
		}
		r.Review = state
		r.Attempts = append(r.Attempts, map[string]any{"time": stamp(now), "correct": true})
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var restored Record
		if err = json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		r = restored
		if !r.Wrong || !ValidReview(r.Review) || Due(q, r, now.Add(day)) {
			t.Fatal("reload lost membership or shortened interval")
		}
		early, earlyMeta := ApplyReview(q, r, "right", now.Add(day))
		if earlyMeta["qualified"] != false || !reflect.DeepEqual(early, state) {
			t.Fatal("early retry advanced ladder")
		}
		now = parse(state.DueAt)
	}
}
func TestUncertainAndRelearningHaveDifferentDelays(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, zone)
	for _, outcome := range []string{"uncertain", "wrong", "unknown"} {
		r := Record{Wrong: true}
		state, _ := ApplyReview(Question{}, r, outcome, now)
		want := delay
		if outcome == "uncertain" {
			want = day
		}
		if parse(state.DueAt).Sub(now) != want || state.Right != 0 || state.Streak != 0 {
			t.Fatal("wrong short schedule", outcome, state)
		}
		if outcome == "unknown" && (state.Unknown != 1 || state.Wrong != 0) {
			t.Fatal("unknown merged into incorrect")
		}
		if outcome == "uncertain" && (state.Uncertain != 1 || state.Wrong != 0) {
			t.Fatal("hint counted as incorrect")
		}
		r.Review = state
		early, meta := ApplyReview(Question{}, r, "right", now.Add(5*time.Minute))
		if meta["qualified"] != false || !reflect.DeepEqual(early, state) {
			t.Fatal("immediate correction inflated memory")
		}
		if outcome == "uncertain" {
			early, meta = ApplyReview(Question{}, r, "right", now.Add(time.Hour))
			if meta["qualified"] != false || !reflect.DeepEqual(early, state) {
				t.Fatal("hint retry bypassed one-day interval")
			}
		}
		recovered, _ := ApplyReview(Question{}, r, "right", now.Add(want))
		if recovered.IntervalDays != 3 {
			t.Fatal("relearning did not restart three-day schedule")
		}
	}
}
func TestLegacyReviewDateAndDisputedKeyRemainProtected(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, zone)
	r := Record{Wrong: true}
	state, _ := ApplyReview(Question{}, r, "right", now.Add(-day))
	// A valid existing one-day schedule remains unchanged on load and early retry.
	state.IntervalDays = 1
	state.DueAt = ptr(stamp(now.Add(time.Hour)))
	r.Review = state
	if NextReview(Question{}, r) != parse(state.DueAt) {
		t.Fatal("existing schedule rewritten")
	}
	got, meta := ApplyReview(Question{}, r, "right", now)
	if meta["qualified"] != false || !reflect.DeepEqual(got, state) {
		t.Fatal("legacy future schedule changed")
	}
	got, _ = ApplyReview(Question{}, r, "right", now.Add(time.Hour))
	if got.IntervalDays != 3 {
		t.Fatal("legacy review did not join new ladder")
	}
	q := Question{Errata: "标答存疑"}
	blocked, meta := ApplyReview(q, r, "wrong", now.Add(day))
	if meta["blocked"] != true || meta["qualified"] != false || !reflect.DeepEqual(blocked, state) || Due(q, r, now.Add(30*day)) {
		t.Fatal("disputed key changed schedule")
	}
}
