package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

type idleSurface = surfaceView

// Only one complete view is retained; the spare buffers and stroke masks remain
// reclaimable. This is an opt-in hint, not a periodic keepalive frame.
func (a *Editor) keepIdleSurface(source *ui.Canvas, destination, viewport ui.Rect, c ink.Camera) {
	a.retainSurface(surfaceView{source, destination, viewport, c, a.bakedCount})
}
func (a *Editor) retainSurface(v surfaceView) {
	if a.idleSurface.source != v.source {
		a.releaseIdleSurface()
		v.source.KeepOnIdle(true)
	}
	a.idleSurface = v
}
func (a *Editor) releaseIdleSurface() {
	if a.idleSurface.source != nil {
		a.idleSurface.source.KeepOnIdle(false)
	}
	a.idleSurface = idleSurface{}
}

// RecoverAfterIdle reuses a backend-confirmed complete texture when possible.
// If the device really lost it or the backend cannot confirm retention, keep
// the existing conservative vector-rebuild path. Never trust a blank texture.
func (a *Editor) RecoverAfterIdle(p *ui.Painter, rect ui.Rect) bool {
	ready := a.idleSurface.source != nil && a.idleSurface.source.HasGPUContents()
	return a.recoverIdleSurface(p, rect, ready)
}

// Separate the backend validity check from the deterministic recovery operation.
func (a *Editor) recoverIdleSurface(p *ui.Painter, rect ui.Rect, ready bool) bool {
	old := a.idleSurface
	if !ready || old.source == nil || old.viewport != rect || old.camera.PixelRatio != a.camera().PixelRatio || a.dryInvalid {
		a.ResetSurfaceCaches()
		a.fullIdleRecoveries++
		return false
	}
	a.navigationBake = navigationBake{}
	a.panCache.reset()
	a.live.Release()
	a.erasePreview.Reset()
	a.selection.releasePreview()
	if spare := 1 - a.dryFront; a.dry[spare] != old.source {
		a.dry[spare] = nil
	}
	a.adoptSurface(p, old)
	a.retainedIdleRecoveries++
	return true
}

func (a *Editor) hasTrimmedWorkBuffers() bool {
	for _, l := range a.dry {
		if l != nil && l.Trimmed() {
			return true
		}
	}
	for _, l := range a.panCache.layers {
		if l != nil && l.Trimmed() {
			return true
		}
	}
	for _, l := range a.navigationBake.layers {
		if l != nil && l.Trimmed() {
			return true
		}
	}
	return false
}
