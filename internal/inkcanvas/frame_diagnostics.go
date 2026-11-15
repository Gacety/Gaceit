package inkcanvas

import (
	"slices"
	"time"
)

type PhaseTiming struct {
	Count uint64  `json:"count"`
	P50MS float64 `json:"recent_p50_ms"`
	P95MS float64 `json:"recent_p95_ms"`
	MaxMS float64 `json:"max_ms"`
}
type FrameDiagnostics struct {
	CopiedPixels uint64                 `json:"checkpoint_copied_pixels"`
	Phases       map[string]PhaseTiming `json:"cpu_phases"`
	Rebuilds     uint64                 `json:"full_rebuilds"`
	Appends      uint64                 `json:"append_checkpoints"`
	TileHits     uint64                 `json:"tile_hits"`
	TileMisses   uint64                 `json:"tile_misses"`
	Prefetched   uint64                 `json:"prefetched_tiles"`
}
type phaseSamples struct {
	values  [32]float64
	count   uint64
	maximum float64
}
type frameTrace struct {
	phases            map[string]*phaseSamples
	rebuilds, appends uint64
	copiedPixels      uint64
}

// ObservePhase measures CPU preparation/command recording, not GPU completion.
// Fixed phase names only, bounded samples, explicit export; never logs ink.
func (a *Editor) ObservePhase(name string, start time.Time) {
	switch name {
	case "input", "frame", "prepare", "live", "native_build", "tile_sync":
	default:
		return
	}
	d := &a.frameTrace
	if d.phases == nil {
		d.phases = make(map[string]*phaseSamples)
	}
	s := d.phases[name]
	if s == nil {
		s = &phaseSamples{}
		d.phases[name] = s
	}
	ms := float64(time.Since(start)) / float64(time.Millisecond)
	s.values[s.count%32] = ms
	s.count++
	s.maximum = max(s.maximum, ms)
}
func (a *Editor) FrameDiagnostics() FrameDiagnostics {
	out := FrameDiagnostics{CopiedPixels: a.frameTrace.copiedPixels, Phases: map[string]PhaseTiming{}, Rebuilds: a.frameTrace.rebuilds, Appends: a.frameTrace.appends, TileHits: a.tiles.hits, TileMisses: a.tiles.misses, Prefetched: a.tiles.prefetched}
	for name, s := range a.frameTrace.phases {
		v := append([]float64(nil), s.values[:min(s.count, 32)]...)
		slices.Sort(v)
		if len(v) > 0 {
			out.Phases[name] = PhaseTiming{s.count, v[(len(v)-1)/2], v[(len(v)*95-1)/100], s.maximum}
		}
	}
	return out
}
