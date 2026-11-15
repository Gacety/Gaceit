package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"time"
)

// Diagnostics contain timing/counts only: no question text, coordinates or ink.
// The ring is bounded and is exported only through an explicit user action.
type InputAnomaly struct {
	Kind  string    `json:"kind"`
	At    time.Time `json:"at"`
	GapMS float64   `json:"gap_ms,omitempty"`
}
type InputDiagnostics struct {
	Render        FrameDiagnostics `json:"render"`
	PenEvents     uint64           `json:"pen_events"`
	ForeignEvents uint64           `json:"ignored_other_device_events"`
	Cancellations uint64           `json:"capture_cancellations"`
	RepeatedDowns uint64           `json:"repeated_pen_downs"`
	LongGaps      uint64           `json:"delivery_gaps_over_80ms"`
	MaxGapMS      float64          `json:"max_delivery_gap_ms"`
	Recent        []InputAnomaly   `json:"recent"`
}
type inputTrace struct {
	totals      InputDiagnostics
	ring        [32]InputAnomaly
	next, count int
	last        time.Time
}

func (d *inputTrace) note(kind string, at time.Time, gap time.Duration) {
	d.ring[d.next] = InputAnomaly{kind, at, float64(gap) / float64(time.Millisecond)}
	d.next = (d.next + 1) % len(d.ring)
	d.count = min(d.count+1, len(d.ring))
}
func eventTime(e ui.InputEvent) time.Time {
	if !e.Time.IsZero() {
		return e.Time
	}
	return time.Now()
}
func (a *Editor) noteForeignInput(e ui.InputEvent) {
	a.inputTrace.totals.ForeignEvents++
	a.inputTrace.note("other_device_ignored", eventTime(e), 0)
}
func (a *Editor) traceInput(e ui.InputEvent) {
	if e.Kind != ui.InputPointerDown && e.Kind != ui.InputPointerMove && e.Kind != ui.InputPointerUp {
		return
	}
	d := &a.inputTrace
	if e.Canceled && a.Busy() {
		d.totals.Cancellations++
		d.note("capture_cancel", eventTime(e), 0)
		d.last = time.Time{}
		return
	}
	if e.Pointer != ui.PointerPen {
		return
	}
	at := eventTime(e)
	if e.Kind == ui.InputPointerDown {
		if a.Busy() {
			d.totals.RepeatedDowns++
			d.note("repeated_pen_down", at, 0)
		}
		d.last = time.Time{}
	}
	if a.Busy() || e.Kind == ui.InputPointerDown {
		d.totals.PenEvents++
		if !d.last.IsZero() && at.After(d.last) {
			gap := at.Sub(d.last)
			d.totals.MaxGapMS = max(d.totals.MaxGapMS, float64(gap)/float64(time.Millisecond))
			if gap > 80*time.Millisecond {
				d.totals.LongGaps++
				d.note("delivery_gap", at, gap)
			}
		}
		d.last = at
	}
	if e.Kind == ui.InputPointerUp {
		d.last = time.Time{}
	}
}
func (a *Editor) InputDiagnostics() InputDiagnostics {
	out := a.inputTrace.totals
	out.Render = a.FrameDiagnostics()
	out.Recent = make([]InputAnomaly, 0, a.inputTrace.count)
	for i := 0; i < a.inputTrace.count; i++ {
		out.Recent = append(out.Recent, a.inputTrace.ring[(a.inputTrace.next-a.inputTrace.count+i+32)%32])
	}
	return out
}
