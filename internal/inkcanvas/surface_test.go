package inkcanvas

import (
	"bytes"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
)

func TestSurfaceResetRebuildsCompleteInk(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() { ed.Canvas(c) })
	}
	_ = ui.Render(view, 420, 320, 1)
	for i := 0; i < 4; i++ {
		y := float32(80 + i*40)
		ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, X: 40, Y: y, Button: 0, Pointer: ui.PointerMouse})
		ed.Input(ui.InputEvent{Kind: ui.InputPointerMove, X: 180, Y: y + 12, Pointer: ui.PointerMouse})
		ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: 340, Y: y - 8, Button: 0, Pointer: ui.PointerMouse})
	}
	if ed.Revision() == 0 || len(ed.Snapshot().Strokes) != 4 {
		t.Fatal("completed strokes did not advance the edit revision")
	}

	before := ui.Render(view, 420, 320, 1)
	ed.ResetSurfaceCaches()
	after := ui.Render(view, 420, 320, 1)
	if !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("surface reset dropped part of the dry layer")
	}
}

func TestLoadAdaptsNeutralInkToCurrentTheme(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(true, 1)
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: []ink.Stroke{{
		Tool:  ink.Pen,
		Color: [4]float32{0, 0, 0, 1},
		Points: []ink.Point{
			{X: 10, Y: 10, Radius: 1},
			{X: 20, Y: 20, Radius: 1},
		},
	}}})
	if got := ed.Snapshot().Strokes[0].Color; got != ([4]float32{1, 1, 1, 1}) {
		t.Fatalf("legacy black ink stayed invisible on dark paper: %v", got)
	}
}

func TestPenCancelPreservesObservedCurveWithoutBridge(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(true, 1)
	points := [][2]float32{{40, 20}, {60, 40}, {40, 60}, {20, 40}, {40, 20}}
	for i, pt := range points {
		kind := ui.InputPointerMove
		if i == 0 {
			kind = ui.InputPointerDown
		}
		ed.Input(ui.InputEvent{Kind: kind, X: pt[0], Y: pt[1], Pointer: ui.PointerPen, Pressure: .6})
	}
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: 900, Y: 900, Canceled: true, Pointer: ui.PointerPen})
	d := ed.Snapshot()
	if len(d.Strokes) != 1 || len(d.Strokes[0].Points) != len(points) {
		t.Fatal("capture loss dropped the observed curve")
	}
	for _, p := range d.Strokes[0].Points {
		if p.X > 60 || p.Y > 60 {
			t.Fatal("cancel inserted a straight bridge", p)
		}
	}
	ed.Input(ui.InputEvent{Kind: ui.InputPointerDown, X: 150, Y: 100, Pointer: ui.PointerPen, Pressure: .5})
	ed.Input(ui.InputEvent{Kind: ui.InputPointerUp, X: 152, Y: 102, Pointer: ui.PointerPen, Pressure: 0})
	d = ed.Snapshot()
	if len(d.Strokes) != 2 || d.Strokes[1].Points[0].X != 150 {
		t.Fatal("resumed input connected across cancellation")
	}
}

func TestEmptyCanvasAllocatesOnlyOneDrySurface(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 420, 320, 1)
	if ed.dry[0] == nil || ed.dry[1] != nil {
		t.Fatal("empty canvas allocated spare dry texture")
	}
	ed.ResetSurfaceCaches()
	if ed.dry != ([2]*ui.Canvas{}) || ed.panCache.layers != ([2]*ui.Canvas{}) {
		t.Fatal("surface release retained caches")
	}
}

func TestBufferedTabletCurveRetainsIntermediateSamples(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// Deliver the queued samples without any intervening Draw, as after a
	// briefly busy UI thread. No global cursor lookup may replace these points.
	events := []ui.InputEvent{
		{Kind: ui.InputPointerDown, X: 20, Y: 20, Pointer: ui.PointerPen, Pressure: .5},
		{Kind: ui.InputPointerMove, X: 60, Y: 10, Pointer: ui.PointerPen, Pressure: .7},
		{Kind: ui.InputPointerMove, X: 90, Y: 50, Pointer: ui.PointerPen, Pressure: .6},
		{Kind: ui.InputPointerMove, X: 50, Y: 90, Pointer: ui.PointerPen, Pressure: .8},
		{Kind: ui.InputPointerMove, X: 10, Y: 60, Pointer: ui.PointerPen, Pressure: .4},
		{Kind: ui.InputPointerUp, X: 20, Y: 20, Pointer: ui.PointerPen},
	}
	for _, e := range events {
		ed.Input(e)
	}
	d := ed.Snapshot()
	if len(d.Strokes) != 1 || len(d.Strokes[0].Points) != len(events) {
		t.Fatal("queued curve lost samples")
	}
	for i, p := range d.Strokes[0].Points {
		if p.X != events[i].X || p.Y != events[i].Y {
			t.Fatal("event coordinates replaced", i)
		}
	}
}

func TestBoundedZoomKeepsPointerAnchor(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.ConfigureZoom(.8, .5, 2, .6)
	ed.Home()
	if ed.Zoom() != .8 {
		t.Fatal("initial zoom not applied")
	}
	before := ed.toWorld(100, 80)
	ed.Input(ui.InputEvent{Kind: ui.InputZoom, Scale: 10, X: 100, Y: 80})
	if ed.Zoom() > .8*1.12+.000001 || ed.toWorld(100, 80) != before {
		t.Fatal("pinch jumped or moved its anchor")
	}
	for i := 0; i < 100; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputZoom, Scale: 10, X: 100, Y: 80})
	}
	if ed.Zoom() != 2 {
		t.Fatal("maximum zoom not enforced")
	}
	for i := 0; i < 100; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputZoom, Scale: .01, X: 100, Y: 80})
	}
	if ed.Zoom() != .5 {
		t.Fatal("minimum zoom not enforced")
	}
	ed.Load(Document{Version: 1, Zoom: 8})
	if ed.Zoom() != 2 {
		t.Fatal("old session bypassed zoom limit")
	}
}

func TestNavigationBakeIsBoundedAndPixelComplete(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	strokes := make([]ink.Stroke, 40)
	for i := range strokes {
		x, y := float32(i%8)*30+10, float32(i/8)*30+10
		strokes[i] = ink.Stroke{Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 15, Radius: 2}, {X: x + 25, Y: y, Radius: 2}}, Smooth: true, Color: [4]float32{0, 0, 0, 1}}
	}
	for i := range strokes {
		if i%5 == 0 {
			strokes[i].Tool = ink.Highlighter
			strokes[i].Color = [4]float32{.8, .6, .1, .4}
		}
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	view := func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }
	tt := ui.NewTester(view, 320, 240)
	ed.ZoomBy(1.5)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	tt.Frame()
	if !ed.RebuildingNavigation() || ed.navigationBake.next < navigationBakeBatch {
		t.Fatal("zoom did not use bounded bake")
	}
	// A new camera request must supersede unfinished work without installing it.
	ed.ZoomBy(.8)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	tt.Frame()
	if ed.navigationBake.camera != ed.camera() || ed.navigationBake.next < navigationBakeBatch {
		t.Fatal("stale camera work was retained")
	}
	for i := 0; i < 20 && ed.RebuildingNavigation(); i++ {
		tt.Frame()
	}
	if ed.RebuildingNavigation() {
		t.Fatal("bake did not finish")
	}
	before := image.NewRGBA(tt.Image().Rect)
	copy(before.Pix, tt.Image().Pix)
	ed.ResetSurfaceCaches()
	tt.Frame()
	if !bytes.Equal(before.Pix, tt.Image().Pix) {
		for i, v := range before.Pix {
			if v != tt.Image().Pix[i] {
				t.Fatalf("progressive bake differs at (%d,%d) channel %d: %d vs %d", (i/4)%320, i/(320*4), i%4, v, tt.Image().Pix[i])
			}
		}
	}
	if len(ed.Snapshot().Strokes) != len(strokes) {
		t.Fatal("navigation changed vector strokes")
	}
}

func TestContinuousPanNeverWaitsForStopToRevealInk(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var strokes []ink.Stroke
	for i := 0; i < 18; i++ {
		x := float32(30 + i*55)
		strokes = append(strokes, ink.Stroke{Points: []ink.Point{{X: x, Y: 40, Radius: 3}, {X: x + 20, Y: 100, Radius: 3}}, Color: [4]float32{0, 0, 0, 1}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	view := func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }
	tt := ui.NewTester(view, 320, 240)
	for _, delta := range []float32{180, 180, -120, 240, -400, 600, -300} {
		// Do not wait for a timer/idle frame: every moved-to view must already
		// contain the strokes newly exposed by this input event.
		ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: delta, DY: 0})
		tt.Frame()
		reference, err := New()
		if err != nil {
			t.Fatal(err)
		}
		reference.Load(ed.Snapshot())
		full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { reference.Canvas(c) }) }, 320, 240, 1)
		if !bytes.Equal(tt.Image().Pix, full.Pix) {
			t.Fatal("moving viewport left newly visible ink blank", delta)
		}
	}
}

func TestPanningDuringZoomBakeCannotStarveInk(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var strokes []ink.Stroke
	for i := 0; i < 60; i++ {
		x, y := float32(i%10)*35, float32(i/10)*22
		strokes = append(strokes, ink.Stroke{Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 15, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 320, 240)
	ed.ZoomBy(1.2)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	tt.Frame()
	if !ed.RebuildingNavigation() {
		t.Fatal("setup: scale rebake already finished")
	}
	for i := 0; i < 4; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 40, DY: 10})
		tt.Frame()
		if ed.RebuildingNavigation() {
			t.Fatal("pan kept restarting unfinished zoom work")
		}
		ref, err := New()
		if err != nil {
			t.Fatal(err)
		}
		ref.Load(ed.Snapshot())
		full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 320, 240, 1)
		if !bytes.Equal(tt.Image().Pix, full.Pix) {
			t.Fatal("interrupted zoom left missing ink while panning", i)
		}
	}
}

func TestRetainedIdleViewRecoveryPreservesPixels(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: []ink.Stroke{{Points: []ink.Point{{X: 20, Y: 40, Radius: 2}, {X: 200, Y: 100, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}}}})
	recoverNow := false
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			if recoverNow {
				ed.recoverIdleSurface(p, r, true)
				recoverNow = false
			}
			ed.Draw(p, r)
		})
	}
	tt := ui.NewTester(view, 320, 240)
	// Emulate the valid-retained signal; the separate real-Metal test verifies
	// that backend signal and unchanged GPU pixels across actual cache trimming.
	for _, pan := range []bool{false, true} {
		if pan {
			ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 60, DY: 30})
			tt.Frame()
		}
		before := append([]byte(nil), tt.Image().Pix...)
		recoverNow = true
		tt.Frame()
		if !bytes.Equal(before, tt.Image().Pix) {
			t.Fatal("retained-view recovery changed ink", pan)
		}
	}
	if ed.retainedIdleRecoveries != 2 || ed.fullIdleRecoveries != 0 {
		t.Fatal("valid retained surfaces caused vector rebuild")
	}
}

func TestZoomOutPreviewShowsNewlyExposedStroke(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(true, 1)
	// At 2x the old viewport is roughly world 0..210 x 0..160; this stroke is
	// outside it but visible after zooming out around the viewport center.
	ed.Load(Document{Version: 1, Zoom: 2, Strokes: []ink.Stroke{{
		Tool: ink.Pen, Color: [4]float32{1, 1, 1, 1}, Smooth: true,
		Points: []ink.Point{{X: 500, Y: 140, Radius: 5}, {X: 520, Y: 160, Radius: 5}},
	}}})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 420, 320)
	tt.Frame()
	ed.ZoomBy(.25)
	tt.Frame()
	c := ed.camera()
	x := int(510*c.Zoom + c.Offset[0])
	y := int(150*c.Zoom + c.Offset[1])
	if x < 0 || x >= 420 || y < 0 || y >= 320 {
		t.Fatalf("test stroke outside viewport: %d,%d", x, y)
	}
	pix := tt.Image().RGBAAt(x, y)
	if pix.R < 120 {
		t.Fatalf("zoom-out preview painted paper over newly exposed ink at %d,%d: %#v", x, y, pix)
	}
}

func TestRecomposeRequestSurvivesActiveBake(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 40; i++ {
		x, y := float32(20+(i%8)*30), float32(20+(i/8)*30)
		strokes = append(strokes, ink.Stroke{Tool: ink.Pen, Color: [4]float32{0, 0, 0, 1}, Smooth: true, Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 10, Radius: 2}}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 320, 240)
	tt.Frame()
	ed.RequestRecompose()
	tt.Frame()
	if !ed.RebuildingNavigation() {
		t.Fatal("recompose did not start")
	}
	ed.RequestRecompose() // a second card-height change while the bake runs
	for i := 0; i < 20 && (ed.RebuildingNavigation() || ed.recompose); i++ {
		tt.Frame()
	}
	if ed.RebuildingNavigation() || ed.recompose {
		t.Fatal("recompose request was lost by the active bake")
	}
}

func TestPanningDuringZoomAndRecomposeBakeCannotStarveInk(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var strokes []ink.Stroke
	for i := 0; i < 60; i++ {
		x, y := float32(i%10)*35, float32(i/10)*22
		strokes = append(strokes, ink.Stroke{Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 15, Radius: 2}}, Color: [4]float32{0, 0, 0, 1}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 320, 240)
	ed.ZoomBy(1.2)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	ed.RequestRecompose()
	tt.Frame()
	if !ed.RebuildingNavigation() {
		t.Fatal("setup: scale rebake already finished")
	}
	for i := 0; i < 4; i++ {
		ed.Input(ui.InputEvent{Kind: ui.InputScroll, Precise: true, DX: 40, DY: 10})
		tt.Frame()
		if ed.RebuildingNavigation() {
			t.Fatal("pan kept restarting unfinished zoom work", i)
		}
		ref, err := New()
		if err != nil {
			t.Fatal(err)
		}
		ref.Load(ed.Snapshot())
		full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 320, 240, 1)
		if !bytes.Equal(tt.Image().Pix, full.Pix) {
			t.Fatal("interrupted zoom+recompose left missing ink while panning", i)
		}
	}
}

func TestDenseTilePrefetchSurvivesMultipleBatchesAndTrimFallback(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 28; i++ {
		x, y := float32(280+(i%4)*40), float32(20+(i/4)*28)
		strokes = append(strokes, ink.Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: x, Y: y, Radius: 2.5}, {X: x + 20, Y: y + 12, Radius: 2.5}}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 200, 200)
	for i := 0; i < 30 && ed.tiles.entries[tileKey{1, 0}].canvas == nil; i++ {
		tt.Frame()
	}
	entry, ok := ed.tiles.entries[tileKey{1, 0}]
	if !ok || !entry.cyclic {
		t.Fatalf("expected multi-batch cyclic tile warmup, got ok=%v cyclic=%v", ok, entry.cyclic)
	}
	ed.offsetX = -260
	tt.Frame()
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.Load(ed.Snapshot())
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 200, 200, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("multi-batch prefetched tile differs from full vector render")
	}

	// Simulate an unrendered/trimmed cyclic tile and ensure panning rebuilds it from vectors.
	entry.canvas = ui.NewCanvas(inkTilePixels, inkTilePixels)
	ed.tiles.entries[tileKey{1, 0}] = entry
	ed.offsetX = -520
	tt.Frame()
	ed.offsetX = -260
	tt.Frame()
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("trimmed cyclic tile failed to rebuild from vectors")
	}
}

func TestNavigationBakeGuardsUnrenderedPreviousLayer(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 32; i++ {
		x, y := float32(20+(i%8)*30), float32(20+(i/8)*30)
		strokes = append(strokes, ink.Stroke{Smooth: true, Color: [4]float32{0, 0, 0, 1}, Points: []ink.Point{{X: x, Y: y, Radius: 2}, {X: x + 20, Y: y + 10, Radius: 2}}})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	doubleDraw := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			ed.Draw(p, r)
			if doubleDraw {
				doubleDraw = false
				ed.Draw(p, r)
			}
		})
	}, 320, 240)
	tt.Frame()
	ed.ZoomBy(1.2)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	// Batch 1 (frame 1) -> Batch 2 + attempted Batch 3 before present (frame 2).
	tt.Frame()
	doubleDraw = true
	tt.Frame()
	for i := 0; i < 20 && ed.RebuildingNavigation(); i++ {
		tt.Frame()
	}
	if ed.RebuildingNavigation() {
		t.Fatal("guarded bake did not finish")
	}
	ref, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ref.Appearance(false, 1)
	ref.Load(ed.Snapshot())
	full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 320, 240, 1)
	if !bytes.Equal(tt.Image().Pix, full.Pix) {
		t.Fatal("unpresented intermediate frame corrupted ping-pong navigation bake")
	}
}

func TestPanCacheLargeJumpAndFractionalDPIPhaseSync(t *testing.T) {
	for _, ratio := range []float32{1.375, 1.5} {
		ed := tileFixture(t, false, 1)
		ed.SetTransparentInk(true)
		ed.Appearance(false, ratio)
		ed.offsetX = -12.3
		ed.offsetY = -8.7
		tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 320, 240)
		tt.SetScale(ratio)
		ed.ResetSurfaceCaches()
		tt.Frame()

		checkFrame := func(step string) {
			t.Helper()
			if ed.panCache.layers[0] != nil {
				cam := ed.panCache.tileCamera()
				if dx, dy := phaseDrift(cam.Offset[0], ed.tiles.phase[0]), phaseDrift(cam.Offset[1], ed.tiles.phase[1]); dx > 1e-3 || dy > 1e-3 {
					t.Fatalf("ratio=%v %s: tile phase drift dx=%g dy=%g (cam=%v tile=%v)", ratio, step, dx, dy, cam.Offset, ed.tiles.phase)
				}
			}
			ref := tileFixture(t, false, 1)
			ref.SetTransparentInk(true)
			ref.Appearance(false, ratio)
			ref.offsetX = ed.offsetX
			ref.offsetY = ed.offsetY
			full := ui.Render(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ref.Canvas(c) }) }, 320, 240, ratio)
			got := tt.Image().Pix
			if bytes.Equal(got, full.Pix) {
				return
			}
			different, maxDelta := 0, 0
			for i, v := range got {
				d := int(v) - int(full.Pix[i])
				if d < 0 {
					d = -d
				}
				if d > 0 {
					different++
				}
				maxDelta = max(maxDelta, d)
			}
			if maxDelta > 1 || different > len(full.Pix)/1000 {
				t.Fatalf("ratio=%v %s: pan pixel mismatch off=(%v,%v) diff=%d maxDelta=%d", ratio, step, ed.offsetX, ed.offsetY, different, maxDelta)
			}
		}

		// Incremental pans by integer physical pixels (> 72px recenter threshold) at fractional DPI.
		stepX := -88.0 / float64(ratio)
		stepY := -80.0 / float64(ratio)
		for i := 0; i < 6; i++ {
			ed.offsetX += stepX
			ed.offsetY += stepY
			tt.Frame()
			checkFrame("incremental")
		}
		// Arbitrary large jumps to fractional coordinates (e.g. Home / far jump) and back.
		for _, off := range [][2]float64{{240.4, 180.6}, {-950.7, -420.3}, {-12.3, -8.7}} {
			ed.offsetX, ed.offsetY = off[0], off[1]
			tt.Frame()
			checkFrame("largeJump")
			ed.offsetX -= stepX
			ed.offsetY += stepY
			tt.Frame()
			checkFrame("postJumpIncremental")
		}
	}
}
