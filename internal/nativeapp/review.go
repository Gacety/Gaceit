package nativeapp

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
)

const day = 24 * time.Hour
const delay = 10 * time.Minute
const spaced = 12 * time.Hour
const reviewConfigKey = "408-review-config-v1"

var zone = time.FixedZone("Asia/Taipei", 8*3600)

// ReviewConfig captures the global parameters used to compute next review times
// and HLR half-life updates so backups can export and restore scheduling rules.
type ReviewConfig struct {
	Algorithm             string        `json:"algorithm"`
	SchedulePolicy        string        `json:"schedulePolicy"`
	LadderDays            []float64     `json:"ladderDays"`
	RelearnDelayMinutes   float64       `json:"relearnDelayMinutes"`
	UncertainIntervalDays float64       `json:"uncertainIntervalDays"`
	CooldownHours         float64       `json:"cooldownHours"`
	PriorWeights          [3]float64    `json:"priorWeights"`
	WeightLimits          [3][2]float64 `json:"weightLimits"`
	HalfLifeMinDays       float64       `json:"halfLifeMinDays"`
	HalfLifeMaxDays       float64       `json:"halfLifeMaxDays"`
	LearningRate          float64       `json:"learningRate"`
	L2Regularization      float64       `json:"l2Regularization"`
	Timezone              string        `json:"timezone"`
}

func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		Algorithm:             "hlr-local-v1",
		SchedulePolicy:        "spaced-3-7-14-30-v1",
		LadderDays:            []float64{3, 7, 14, 30, 60, 120, 240, 365},
		RelearnDelayMinutes:   10,
		UncertainIntervalDays: 1,
		CooldownHours:         12,
		PriorWeights:          [3]float64{1, 1.7, -1.2},
		WeightLimits:          [3][2]float64{{-4, 8}, {.25, 3}, {-4, -.25}},
		HalfLifeMinDays:       15. / 1440,
		HalfLifeMaxDays:       3650,
		LearningRate:          0.2,
		L2Regularization:      0.01,
		Timezone:              "Asia/Taipei",
	}
}

func cloneReviewConfig(cfg ReviewConfig) ReviewConfig {
	cfg.LadderDays = slices.Clone(cfg.LadderDays)
	return cfg
}

func normalizeReviewConfig(cfg ReviewConfig) ReviewConfig {
	def := DefaultReviewConfig()
	if strings.TrimSpace(cfg.Algorithm) == "" {
		cfg.Algorithm = def.Algorithm
	}
	if strings.TrimSpace(cfg.SchedulePolicy) == "" {
		cfg.SchedulePolicy = def.SchedulePolicy
	}
	var ladder []float64
	for _, d := range cfg.LadderDays {
		if !math.IsNaN(d) && !math.IsInf(d, 0) && d > 0 {
			ladder = append(ladder, d)
		}
	}
	if len(ladder) == 0 {
		ladder = def.LadderDays
	} else {
		slices.Sort(ladder)
	}
	cfg.LadderDays = ladder
	if math.IsNaN(cfg.RelearnDelayMinutes) || math.IsInf(cfg.RelearnDelayMinutes, 0) || cfg.RelearnDelayMinutes <= 0 {
		cfg.RelearnDelayMinutes = def.RelearnDelayMinutes
	}
	if math.IsNaN(cfg.UncertainIntervalDays) || math.IsInf(cfg.UncertainIntervalDays, 0) || cfg.UncertainIntervalDays <= 0 {
		cfg.UncertainIntervalDays = def.UncertainIntervalDays
	}
	if math.IsNaN(cfg.CooldownHours) || math.IsInf(cfg.CooldownHours, 0) || cfg.CooldownHours <= 0 {
		cfg.CooldownHours = def.CooldownHours
	}
	priorBad := cfg.PriorWeights == [3]float64{}
	for _, w := range cfg.PriorWeights {
		if math.IsNaN(w) || math.IsInf(w, 0) {
			priorBad = true
		}
	}
	if priorBad {
		cfg.PriorWeights = def.PriorWeights
	}
	for i := 0; i < 3; i++ {
		lo, hi := cfg.WeightLimits[i][0], cfg.WeightLimits[i][1]
		if (lo == 0 && hi == 0) || math.IsNaN(lo) || math.IsInf(lo, 0) || math.IsNaN(hi) || math.IsInf(hi, 0) || lo > hi {
			cfg.WeightLimits[i] = def.WeightLimits[i]
		}
	}
	if math.IsNaN(cfg.HalfLifeMinDays) || math.IsInf(cfg.HalfLifeMinDays, 0) || cfg.HalfLifeMinDays <= 0 {
		cfg.HalfLifeMinDays = def.HalfLifeMinDays
	}
	if math.IsNaN(cfg.HalfLifeMaxDays) || math.IsInf(cfg.HalfLifeMaxDays, 0) || cfg.HalfLifeMaxDays < cfg.HalfLifeMinDays {
		cfg.HalfLifeMaxDays = max(def.HalfLifeMaxDays, cfg.HalfLifeMinDays)
	}
	if math.IsNaN(cfg.LearningRate) || math.IsInf(cfg.LearningRate, 0) || cfg.LearningRate <= 0 {
		cfg.LearningRate = def.LearningRate
	}
	if math.IsNaN(cfg.L2Regularization) || math.IsInf(cfg.L2Regularization, 0) || cfg.L2Regularization < 0 {
		cfg.L2Regularization = def.L2Regularization
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		cfg.Timezone = def.Timezone
	}
	return cfg
}

var (
	reviewConfigMu     sync.RWMutex
	activeReviewConfig = DefaultReviewConfig()
)

func CurrentReviewConfig() ReviewConfig {
	reviewConfigMu.RLock()
	defer reviewConfigMu.RUnlock()
	return cloneReviewConfig(activeReviewConfig)
}

func SetReviewConfig(cfg ReviewConfig) ReviewConfig {
	norm := normalizeReviewConfig(cfg)
	reviewConfigMu.Lock()
	activeReviewConfig = cloneReviewConfig(norm)
	if norm.Timezone == "Asia/Taipei" {
		zone = time.FixedZone("Asia/Taipei", 8*3600)
	} else if loc, err := time.LoadLocation(norm.Timezone); err == nil {
		zone = loc
	}
	reviewConfigMu.Unlock()
	return norm
}

func (c ReviewConfig) relearnDelay() time.Duration {
	return time.Duration(c.RelearnDelayMinutes * float64(time.Minute))
}
func (c ReviewConfig) uncertainDuration() time.Duration {
	return time.Duration(c.UncertainIntervalDays * float64(day))
}
func (c ReviewConfig) cooldownDuration() time.Duration {
	return time.Duration(c.CooldownHours * float64(time.Hour))
}

// Qualified independent recall advances the user-selected schedule. HLR still
// learns from actual time gaps; its estimates no longer force next-day reviews.
func independentInterval(before *Review, cfg ReviewConfig) float64 {
	if before.Stage != "review" {
		return cfg.LadderDays[0]
	}
	for _, n := range cfg.LadderDays {
		if n > before.IntervalDays {
			return n
		}
	}
	return cfg.LadderDays[len(cfg.LadderDays)-1]
}

var prior = [3]float64{1, 1.7, -1.2}

type Review struct {
	Version      int       `json:"version"`
	Algorithm    string    `json:"algorithm"`
	Stage        string    `json:"stage"`
	Weights      []float64 `json:"weights"`
	PriorRight   float64   `json:"priorRight"`
	PriorWrong   float64   `json:"priorWrong"`
	PriorUnknown float64   `json:"priorUnknown"`
	Right        float64   `json:"right"`
	Wrong        float64   `json:"wrong"`
	Unknown      float64   `json:"unknown"`
	Uncertain    float64   `json:"uncertain"`
	Streak       float64   `json:"streak"`
	Reps         float64   `json:"reps"`
	Lapses       float64   `json:"lapses"`
	Trained      float64   `json:"trained"`
	LastReviewAt *string   `json:"lastReviewAt"`
	DueAt        *string   `json:"dueAt"`
	LastOutcome  *string   `json:"lastOutcome"`
	IntervalDays float64   `json:"intervalDays"`
	HalfLifeDays *float64  `json:"halfLifeDays"`
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
func ptr[T any](v T) *T        { return &v }
func parse(s *string) time.Time {
	if s == nil {
		return time.Time{}
	}
	v, _ := time.Parse(time.RFC3339Nano, *s)
	return v
}
func clamp(x, a, b float64) float64 { return math.Min(b, math.Max(a, x)) }
func ValidReview(r *Review) bool {
	cfg := CurrentReviewConfig()
	if r == nil || r.Version != 1 || (r.Algorithm != "hlr-local-v1" && r.Algorithm != cfg.Algorithm) || len(r.Weights) != 3 {
		return false
	}
	if r.Stage != "new" && r.Stage != "learning" && r.Stage != "relearning" && r.Stage != "review" {
		return false
	}
	for _, v := range r.Weights {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	for _, v := range []float64{r.Right, r.Wrong, r.Unknown, r.Uncertain, r.Streak, r.Reps, r.Lapses, r.Trained, r.PriorRight, r.PriorWrong, r.PriorUnknown, r.IntervalDays} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return (r.LastReviewAt == nil || !parse(r.LastReviewAt).IsZero()) && (r.DueAt == nil || !parse(r.DueAt).IsZero())
}
func ReviewState(q Question, r Record) *Review {
	cfg := CurrentReviewConfig()
	if ValidReview(r.Review) {
		copy := *r.Review
		copy.Weights = append([]float64(nil), r.Review.Weights...)
		return &copy
	}
	state := &Review{Version: 1, Algorithm: cfg.Algorithm, Stage: "new", Weights: append([]float64(nil), cfg.PriorWeights[:]...)}
	if q.History.Result == "正确" {
		state.PriorRight = .25
	}
	if q.History.Result == "错误" {
		state.PriorWrong = .5
	}
	right, wrong := 0., 0.
	for _, a := range r.Attempts {
		if a["correct"] == true {
			right += .25
		} else if a["correct"] == false {
			wrong += .25
		}
	}
	state.PriorRight += math.Min(1, right)
	state.PriorWrong += math.Min(1, wrong)
	return state
}
func features(s *Review) [3]float64 {
	return [3]float64{1, math.Sqrt(s.Right + s.PriorRight), math.Sqrt(s.Wrong + s.Unknown + .5*s.Uncertain + s.PriorWrong + s.PriorUnknown)}
}
func halfLifeWithConfig(s *Review, cfg ReviewConfig) float64 {
	x := features(s)
	sum := 0.
	for i, w := range s.Weights {
		sum += w * x[i]
	}
	return clamp(math.Exp2(clamp(sum, -6, 12)), cfg.HalfLifeMinDays, cfg.HalfLifeMaxDays)
}
func HalfLife(s *Review) float64 {
	return halfLifeWithConfig(s, CurrentReviewConfig())
}
func exposure(s *Review, r Record) time.Time {
	last := parse(s.LastReviewAt)
	if last.IsZero() {
		return last
	}
	for _, a := range r.Attempts {
		str, _ := a["time"].(string)
		v, _ := time.Parse(time.RFC3339Nano, str)
		if v.After(last) {
			last = v
		}
	}
	return last
}
func nextReviewWithConfig(q Question, r Record, cfg ReviewConfig) time.Time {
	// A fallback new state has no DueAt. This read-only query needs neither
	// a cloned Review nor a cloned weight vector for every eligible question.
	if !ValidReview(r.Review) {
		return time.Time{}
	}
	s := r.Review
	due := parse(s.DueAt)
	if due.IsZero() {
		return due
	}
	last := exposure(s, r)
	cool := cfg.relearnDelay()
	if s.Stage == "review" {
		cool = cfg.cooldownDuration()
	}
	if !last.IsZero() && last.Add(cool).After(due) {
		due = last.Add(cool)
	}
	return due
}
func NextReview(q Question, r Record) time.Time {
	return nextReviewWithConfig(q, r, CurrentReviewConfig())
}
func Due(q Question, r Record, now time.Time) bool {
	return r.Wrong && !strings.Contains(q.Errata, "标答存疑") && !NextReview(q, r).After(now)
}
func ApplyReview(q Question, r Record, outcome string, now time.Time) (*Review, map[string]any) {
	cfg := CurrentReviewConfig()
	before := ReviewState(q, r)
	success := outcome == "right"
	last := exposure(before, r)
	elapsed := now.Sub(last)
	days := math.Max(0, elapsed.Hours()/24)
	meta := map[string]any{"algorithm": cfg.Algorithm, "outcome": outcome, "qualified": true, "schedulePolicy": cfg.SchedulePolicy, "elapsedDays": nil, "recallBefore": nil}
	if strings.Contains(q.Errata, "标答存疑") {
		meta["qualified"] = false
		meta["blocked"] = true
		meta["reason"] = "标答存疑，仅记录作答，不自动排期。"
		return before, meta
	}
	if !last.IsZero() {
		meta["elapsedDays"] = days
		meta["recallBefore"] = clamp(math.Exp2(-days/halfLifeWithConfig(before, cfg)), .0001, .9999)
	}
	relearn := cfg.relearnDelay()
	cooldown := cfg.cooldownDuration()
	short := !last.IsZero() && elapsed < relearn
	neg := before.LastOutcome != nil && (*before.LastOutcome == "wrong" || *before.LastOutcome == "unknown" || *before.LastOutcome == "uncertain")
	tmp := r
	tmp.Wrong = true
	if short && (success || neg) || success && ((!parse(before.DueAt).IsZero() && parse(before.DueAt).After(now)) || before.Stage == "review" && (!Due(q, tmp, now) || elapsed < cooldown)) {
		meta["qualified"] = false
		meta["reason"] = "短间隔或提前练习仅记录作答，不延长既定复习间隔。"
		return before, meta
	}
	after := *before
	after.Weights = append([]float64(nil), before.Weights...)
	after.Reps++
	after.LastReviewAt = ptr(stamp(now))
	after.LastOutcome = ptr(outcome)
	if !last.IsZero() && elapsed >= cooldown && outcome != "uncertain" {
		x := features(before)
		h := halfLifeWithConfig(before, cfg)
		p := clamp(math.Exp2(-days/h), .0001, .9999)
		observed := 0.
		if success {
			observed = 1
		}
		derivative := 2 * (p - observed) * math.Ln2 * math.Ln2 * p * (days / h)
		rate := cfg.LearningRate / math.Sqrt(1+before.Trained)
		for i, w := range before.Weights {
			next := w - rate*clamp(derivative*x[i]+cfg.L2Regularization*(w-cfg.PriorWeights[i]), -1, 1)
			limits := cfg.WeightLimits
			after.Weights[i] = clamp(next, limits[i][0], limits[i][1])
		}
		after.Trained++
	}
	if !success {
		switch outcome {
		case "wrong":
			after.Wrong++
		case "unknown":
			after.Unknown++
		case "uncertain":
			after.Uncertain++
		}
		after.Streak = 0
		if before.Stage == "review" && outcome != "uncertain" {
			after.Lapses++
		}
		after.Stage = "learning"
		if before.Reps > 0 {
			after.Stage = "relearning"
		}
		after.IntervalDays = 0
		after.DueAt = ptr(stamp(now.Add(relearn)))
		meta["reason"] = fmt.Sprintf("安排约 %.0f 分钟后的短期重学。", cfg.RelearnDelayMinutes)
		if outcome == "uncertain" {
			after.IntervalDays = cfg.UncertainIntervalDays
			after.DueAt = ptr(stamp(now.Add(cfg.uncertainDuration())))
			meta["reason"] = fmt.Sprintf("答对但不确定或使用提示，安排约 %.0f 天后再次独立回忆。", cfg.UncertainIntervalDays)
		}
	} else {
		after.Right++
		after.Streak++
		after.Stage = "review"
		interval := independentInterval(before, cfg)
		after.IntervalDays = interval
		after.DueAt = ptr(stamp(now.Add(time.Duration(interval * float64(day)))))
		meta["reason"] = fmt.Sprintf("独立答出，安排约 %.0f 天后复习。", interval)
	}
	after.HalfLifeDays = ptr(halfLifeWithConfig(&after, cfg))
	return &after, meta
}
