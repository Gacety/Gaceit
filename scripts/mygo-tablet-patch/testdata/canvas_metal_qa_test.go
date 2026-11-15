//go:build darwin && study408_gpuqa

package inkcanvas

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"slices"
	"study408/internal/ink"
	"testing"
	"time"
)

func BenchmarkCanvasMetalPan(b *testing.B) {
	for _, direct := range []bool{true, false} {
		name := "tiles"
		if direct {
			name = "indexed_vectors"
		}
		b.Run(name, func(b *testing.B) {
			study408DirectPan = direct
			defer func() { study408DirectPan = false }()
			ed, tt := navigationFixture(b)
			close, err := tt.Study408UseMetal()
			if err != nil {
				b.Fatal(err)
			}
			defer close()
			ed.ResetSurfaceCaches()
			tt.Frame()
			// Warm both directions, excluding one-time compilation and tile misses.
			for i := 0; i < 4; i++ {
				ed.offsetX = -120 - float64((i%2)*200)
				ed.offsetY = -30 - float64((i%2)*80)
				tt.Frame()
			}
			b.ReportAllocs()
			times := make([]time.Duration, b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				ed.offsetX = -120 - float64((i%2)*200)
				ed.offsetY = -30 - float64((i%2)*80)
				tt.Frame()
				times[i] = time.Since(start)
			}
			b.StopTimer()
			slices.Sort(times)
			b.ReportMetric(float64(times[len(times)/2].Nanoseconds())/1e6, "p50-ms")
			b.ReportMetric(float64(times[(len(times)-1)*95/100].Nanoseconds())/1e6, "p95-ms")
			b.ReportMetric(float64(times[len(times)-1].Nanoseconds())/1e6, "max-ms")
		})
	}
}

// Time only a new pen commit after a completed pan. The previous controller
// rebuilt all visible vectors here instead of handing off the latest viewport.
func BenchmarkCanvasMetalPanThenPen(b *testing.B) {
	ed, tt := navigationFixture(b)
	ed.SetTransparentInk(true)
	close, err := tt.Study408UseMetal()
	if err != nil {
		b.Fatal(err)
	}
	defer close()
	ed.ResetSurfaceCaches()
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ed.offsetX = -120 - float64((i%2)*80)
		ed.offsetY = -30
		tt.Frame()
		b.StartTimer()
		for _, e := range []ui.InputEvent{{Kind: ui.InputPointerDown, X: 40, Y: 40}, {Kind: ui.InputPointerMove, X: 70, Y: 45}, {Kind: ui.InputPointerUp, X: 100, Y: 40}} {
			e.Pointer, e.Pressure = ui.PointerPen, .6
			ed.Input(e)
		}
		tt.Frame()
	}
}

func TestCanvasMetalSelectionTransitions(t *testing.T) {
	for _, all := range []bool{false, true} {
		ed, tt := transitionEditor(t, true)
		close, err := tt.Study408UseMetal()
		if err != nil {
			t.Fatal(err)
		}
		ed.ResetSurfaceCaches()
		tt.Frame()
		ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 180})
		tt.Frame()
		first := 7
		if all {
			first = 0
		}
		ed.selection.ids = makeRange(first, len(ed.strokes))
		ed.updateSelectionBounds()
		ed.selection.before = append([]ink.Stroke(nil), ed.strokes...)
		ed.selection.moving = true
		ed.selection.delta = ui.Point{X: -120}
		tt.Frame()
		preview := append([]byte(nil), tt.Study408MetalPixels()...)
		ed.selection.commitPending = true
		tt.Frame()
		committed := append([]byte(nil), tt.Study408MetalPixels()...)
		ed.ResetSurfaceCaches()
		tt.Frame()
		assertMetalPixels(t, preview, tt.Study408MetalPixels(), false, [2]float64{})
		assertMetalPixels(t, committed, tt.Study408MetalPixels(), false, [2]float64{})
		close()
	}
}
func TestCanvasMetalTilesMatchVectorsAndIdleRecovery(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, ratio := range []float32{1, 2} {
			ed := tileFixture(t, dark, 1.375)
			ed.Appearance(dark, ratio)
			tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
			close, err := tt.Study408UseMetal()
			if err != nil {
				t.Fatal(err)
			}
			ed.ResetSurfaceCaches()
			tt.Frame()
			for _, off := range [][2]float64{{-128, -70}, {-350, -110}, {180, 80}, {-128, -70}} {
				ed.offsetX, ed.offsetY = off[0], off[1]
				tt.Frame()
				got := append([]byte(nil), tt.Study408MetalPixels()...)
				study408DirectPan = true
				ed.panCache.reset()
				tt.Frame()
				study408DirectPan = false
				assertMetalPixels(t, got, tt.Study408MetalPixels(), dark, off)

			}
			// Keep the completed viewport, discard all other GPU textures, then reuse
			// tiles at a different camera. Full tile scenes must restore their pixels.
			tt.Study408TrimMetal()
			ed.offsetX = -350
			ed.offsetY = -110
			tt.Frame()
			got := append([]byte(nil), tt.Study408MetalPixels()...)
			ed.ResetSurfaceCaches()
			tt.Frame()
			assertMetalPixels(t, got, tt.Study408MetalPixels(), dark, [2]float64{-350, -110})
			close()
		}
	}
}

func assertMetalPixels(t *testing.T, a, b []byte, dark bool, off [2]float64) {
	t.Helper()
	if bytes.Equal(a, b) {
		return
	}
	if len(a) != len(b) {
		t.Fatal("Metal dimensions differ")
	}
	different, maxDelta := 0, 0
	for i, v := range a {
		d := int(v) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d > 0 {
			different++
		}
		maxDelta = max(maxDelta, d)
	}
	// Rendering in local tile coordinates can round one 8-bit channel by one
	// unit at an antialiased edge. Larger/area-wide differences are not allowed.
	if maxDelta > 1 || different > len(a)/1000 {
		t.Fatal("Metal tile pixel mismatch", dark, off, "channels", different, "maxDelta", maxDelta)
	}
}

func TestCanvasMetalRapidZoomSupersedesWithoutGhosts(t *testing.T) {
	ed := tileFixture(t, true, 1)
	ed.Appearance(true, 2)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
	close, err := tt.Study408UseMetal()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	ed.ResetSurfaceCaches()
	tt.Frame()
	for _, factor := range []float64{1.2, .65, 1.7, .8, 1.1} {
		base := ed.camera()
		before := append([]byte(nil), tt.Study408MetalPixels()...)
		ed.ZoomBy(1.4)
		ed.zoomChangedAt = time.Now().Add(-time.Second)
		tt.Frame()
		ed.zoom = float64(base.Zoom)
		ed.offsetX = float64(base.Offset[0])
		ed.offsetY = float64(base.Offset[1])
		tt.Frame()
		if ed.RebuildingNavigation() || ed.PreviewingZoom() {
			t.Fatal("stale replacement survived reverse pinch")
		}
		assertMetalPixels(t, before, tt.Study408MetalPixels(), true, [2]float64{float64(base.Zoom), 0})

		ed.ZoomBy(factor)
		ed.zoomChangedAt = time.Now().Add(-time.Second)
		tt.Frame()
		// Interrupt a partially baked view with a different camera.
		ed.ZoomBy(1.03)
		ed.zoomChangedAt = time.Now().Add(-time.Second)
		tt.Frame()
		for i := 0; i < 20 && (ed.RebuildingNavigation() || ed.recompose); i++ {
			tt.Frame()
		}
		if ed.RebuildingNavigation() {
			t.Fatal("zoom never completed")
		}
		got := append([]byte(nil), tt.Study408MetalPixels()...)
		ed.ResetSurfaceCaches()
		tt.Frame()
		assertMetalPixels(t, got, tt.Study408MetalPixels(), true, [2]float64{float64(ed.camera().Zoom), 0})
	}
}

func TestCanvasMetalTransparentInkOverNativeContent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ed := tileFixture(t, dark, 1)
		ed.SetTransparentInk(true)
		tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Background(ui.Hex("#3A78BC")).Children(func() { ed.Canvas(c) }) }, 340, 260)
		close, err := tt.Study408UseMetal()
		if err != nil {
			t.Fatal(err)
		}
		ed.ResetSurfaceCaches()
		tt.Frame()
		// Partial region erasure and ordered selection movement must restore native
		// background via alpha; never leave opaque paper or duplicate stale strokes.
		ed.SetTool("eraser")
		ed.SetEraserRegion(true)
		ed.SetWidth(40)
		ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, Pointer: ui.PointerPen, X: 100, Y: 100})
		tt.Frame()
		ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, Pointer: ui.PointerPen, X: 130, Y: 120})
		tt.Frame()
		got := append([]byte(nil), tt.Study408MetalPixels()...)
		ed.ResetSurfaceCaches()
		tt.Frame()
		assertMetalPixels(t, got, tt.Study408MetalPixels(), dark, [2]float64{})
		ed.selection.ids = []int{0, 2}
		ed.updateSelectionBounds()
		ed.DeleteSelection()
		tt.Frame()
		got = append([]byte(nil), tt.Study408MetalPixels()...)
		ed.ResetSurfaceCaches()
		tt.Frame()
		assertMetalPixels(t, got, tt.Study408MetalPixels(), dark, [2]float64{})
		ed.offsetX = -170
		ed.offsetY = -30
		tt.Frame()
		got = append([]byte(nil), tt.Study408MetalPixels()...)
		ed.ResetSurfaceCaches()
		tt.Frame()
		assertMetalPixels(t, got, tt.Study408MetalPixels(), dark, [2]float64{})
		close()
	}
}

func TestCanvasMetalDenseMultiBatchBakeAndPrefetch(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	ed.SetTransparentInk(true)
	var strokes []ink.Stroke
	// 36 strokes in the initial viewport -> 5 navigation bake batches (>= 3 ping-pongs).
	for i := 0; i < 36; i++ {
		x, y := float32(20+(i%6)*45), float32(20+(i/6)*35)
		strokes = append(strokes, ink.Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: x, Y: y, Radius: 2.5}, {X: x + 22, Y: y + 14, Radius: 2.5}}})
	}
	// 28 strokes packed inside adjacent tile {1, 0} -> 4 prefetch batches (>= 3 ping-pongs).
	for i := 0; i < 28; i++ {
		x, y := float32(280+(i%4)*40), float32(20+(i/4)*28)
		strokes = append(strokes, ink.Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: x, Y: y, Radius: 2.5}, {X: x + 20, Y: y + 12, Radius: 2.5}}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 240, 240)
	close, err := tt.Study408UseMetal()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	ed.ResetSurfaceCaches()
	tt.Frame()

	ed.ZoomBy(1.1)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	for i := 0; i < 20 && (ed.RebuildingNavigation() || i == 0); i++ {
		tt.Frame()
	}
	if ed.RebuildingNavigation() {
		t.Fatal("multi-batch zoom bake did not finish")
	}
	got := append([]byte(nil), tt.Study408MetalPixels()...)
	ed.ResetSurfaceCaches()
	tt.Frame()
	assertMetalPixels(t, got, tt.Study408MetalPixels(), false, [2]float64{})

	// Warm adjacent dense tile across >= 3 batches at 1x, then pan into it and test after trim.
	ed.zoom = 1
	ed.offsetX, ed.offsetY = 0, 0
	ed.ResetSurfaceCaches()
	for i := 0; i < 30 && ed.tiles.entries[tileKey{1, 0}].canvas == nil; i++ {
		tt.Frame()
	}
	if !ed.tiles.entries[tileKey{1, 0}].cyclic {
		t.Fatal("dense adjacent tile did not complete multi-batch prefetch")
	}
	ed.offsetX = -260
	tt.Frame()
	got = append([]byte(nil), tt.Study408MetalPixels()...)
	ed.ResetSurfaceCaches()
	tt.Frame()
	assertMetalPixels(t, got, tt.Study408MetalPixels(), false, [2]float64{-260, 0})

	// After trimming GPU textures, panning away and back must reconstruct cleanly.
	tt.Study408TrimMetal()
	ed.offsetX = -520
	tt.Frame()
	ed.offsetX = -260
	tt.Frame()
	got = append([]byte(nil), tt.Study408MetalPixels()...)
	ed.ResetSurfaceCaches()
	tt.Frame()
	assertMetalPixels(t, got, tt.Study408MetalPixels(), false, [2]float64{-260, 0})
}

func TestCanvasMetalAutoIdleRecoveryWithoutExplicitTimeout(t *testing.T) {
	ed := tileFixture(t, false, 1)
	ed.SetTransparentInk(true)
	// Reproduce the app's DrawPaper + Canvas stack without invoking ed.RecoverAfterIdle
	// externally (simulating a recent paint < 1.8s followed by Metal's 2.0s trim).
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { ed.DrawPaper(p, r) })
			ed.Canvas(c)
		})
	}, 340, 260)
	close, err := tt.Study408UseMetal()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	ed.ResetSurfaceCaches()
	tt.Frame()
	ed.offsetX = -85
	ed.offsetY = -45
	tt.Frame()
	before := append([]byte(nil), tt.Study408MetalPixels()...)

	// Trim non-retained GPU textures (paper.canvas + dry[0] spare), keeping idleSurface.
	tt.Study408TrimMetal()
	if ed.paper.canvas == nil || !ed.paper.canvas.Trimmed() {
		t.Fatal("expected paper canvas to be marked trimmed after Metal trim")
	}
	if ed.idleSurface.source == nil || !ed.idleSurface.source.HasGPUContents() || ed.idleSurface.source.Trimmed() {
		t.Fatal("expected retained idle surface to remain valid")
	}
	tt.Frame()
	if ed.paper.canvas.Trimmed() {
		t.Fatal("DrawPaper did not recover trimmed paper texture")
	}
	if ed.retainedIdleRecoveries == 0 {
		t.Fatal("expected prepareSurface to auto-recover retained idle surface without explicit timeout")
	}
	assertMetalPixels(t, before, tt.Study408MetalPixels(), false, [2]float64{-85, -45})

	// Now drop KeepOnIdle on the retained surface so even idleSurface is trimmed,
	// and verify prepareSurface auto-falls back to full vector reconstruction.
	ed.idleSurface.source.KeepOnIdle(false)
	tt.Study408TrimMetal()
	tt.Frame()
	if ed.fullIdleRecoveries == 0 {
		t.Fatal("expected prepareSurface to auto-fallback to full rebuild when retained surface was trimmed")
	}
	assertMetalPixels(t, before, tt.Study408MetalPixels(), false, [2]float64{-85, -45})
}
