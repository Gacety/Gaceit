package inkcanvas

import (
	"github.com/egoist/mygo/ui"
	"study408/internal/ink"
	"testing"
	"time"
)

func TestReverseZoomBackToCompleteViewCancelsStaleBake(t *testing.T) {
	ed := tileFixture(t, false, 1)
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 340, 260)
	original := ed.camera()
	ed.ZoomBy(1.5)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	tt.Frame()
	if !ed.RebuildingNavigation() {
		t.Fatal("fixture needs partial bake")
	}
	ed.zoom = float64(original.Zoom)
	ed.offsetX = float64(original.Offset[0])
	ed.offsetY = float64(original.Offset[1])
	tt.Frame()
	if ed.RebuildingNavigation() || ed.PreviewingZoom() {
		t.Fatal("reverse zoom retained stale work")
	}
	if ed.dryCamera != original {
		t.Fatal("reverse zoom replaced complete original camera")
	}
}
func TestLongStrokePreviewAndBakeUseComplexityBudgets(t *testing.T) {
	ed, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ed.Appearance(false, 1)
	var strokes []ink.Stroke
	for i := 0; i < 8; i++ {
		points := make([]ink.Point, 3000)
		for j := range points {
			points[j] = ink.Point{X: float32(20 + j%100), Y: float32(20 + i*12 + j%7), Radius: 1}
		}
		strokes = append(strokes, ink.Stroke{Points: points, Color: [4]float32{0, 0, 0, 1}, Smooth: true})
	}
	ed.Load(Document{Version: 1, Zoom: 1, Strokes: strokes})
	if ed.previewVectorsAffordable() {
		t.Fatal("few long strokes bypassed preview budget")
	}
	tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 180, 140)
	ed.ZoomBy(1.1)
	ed.zoomChangedAt = time.Now().Add(-time.Second)
	tt.Frame()
	if ed.navigationBake.next != 1 {
		t.Fatal("multiple long strokes consumed the same bake frame", ed.navigationBake.next)
	}
}

func TestZoomPreviewRendersExposedBandWhenInteriorIsDense(t *testing.T) {
	t.Run("sparse_band_renders_with_dense_interior", func(t *testing.T) {
		ed, err := New()
		if err != nil {
			t.Fatal(err)
		}
		ed.Appearance(true, 1)
		var strokes []ink.Stroke
		// Interior strokes exceed zoomPreviewWorkLimit (1024), outside the zoomed-out band.
		for i := 0; i < 6; i++ {
			pts := make([]ink.Point, 250)
			for j := range pts {
				pts[j] = ink.Point{X: float32(20 + j%140), Y: float32(20 + i*18), Radius: 1.5}
			}
			strokes = append(strokes, ink.Stroke{Tool: ink.Pen, Color: [4]float32{1, 1, 1, 1}, Smooth: true, Points: pts})
		}
		// One short stroke outside the 2x initial viewport, inside the newly exposed band on zoom-out.
		strokes = append(strokes, ink.Stroke{
			Tool: ink.Pen, Color: [4]float32{1, 1, 1, 1}, Smooth: true,
			Points: []ink.Point{{X: 500, Y: 140, Radius: 5}, {X: 520, Y: 160, Radius: 5}},
		})
		ed.Load(Document{Version: 1, Zoom: 2, Strokes: strokes})
		if ed.previewVectorsAffordable() {
			t.Fatal("setup: expected whole-document preview budget to be exceeded")
		}
		tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 420, 320)
		tt.Frame()
		ed.ZoomBy(0.25)
		tt.Frame()
		c := ed.camera()
		x := int(510*c.Zoom + c.Offset[0])
		y := int(150*c.Zoom + c.Offset[1])
		if x < 0 || x >= 420 || y < 0 || y >= 320 {
			t.Fatalf("exposed stroke outside viewport: %d,%d", x, y)
		}
		pix := tt.Image().RGBAAt(x, y)
		if pix.R < 120 {
			t.Fatalf("exposed sparse band left blank paper because of dense interior at %d,%d: %#v", x, y, pix)
		}
	})

	t.Run("dense_band_stays_paper_only", func(t *testing.T) {
		ed, err := New()
		if err != nil {
			t.Fatal(err)
		}
		ed.Appearance(true, 1)
		pts := make([]ink.Point, 1500)
		for j := range pts {
			pts[j] = ink.Point{X: float32(500 + j%20), Y: float32(140 + (j/20)%20), Radius: 5}
		}
		ed.Load(Document{Version: 1, Zoom: 2, Strokes: []ink.Stroke{{
			Tool: ink.Pen, Color: [4]float32{1, 1, 1, 1}, Smooth: true, Points: pts,
		}}})
		tt := ui.NewTester(func(c *ui.Context) { ui.Box(c).Fill().Children(func() { ed.Canvas(c) }) }, 420, 320)
		tt.Frame()
		ed.ZoomBy(0.25)
		tt.Frame()
		c := ed.camera()
		x := int(510*c.Zoom + c.Offset[0])
		y := int(150*c.Zoom + c.Offset[1])
		pix := tt.Image().RGBAAt(x, y)
		if pix.R > 60 {
			t.Fatalf("dense exposed band bypassed zoom preview budget at %d,%d: %#v", x, y, pix)
		}
	})
}
