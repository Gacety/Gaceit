package ink

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestPixelEraserCutsOnlyTouchedAreaAndFutureInkSurvives(t *testing.T) {
	for _, tool := range []Tool{Pen, Highlighter} {
		s := Stroke{Tool: tool, Points: []Point{{10, 40, 12}, {50, 40, 12}, {90, 40, 12}}, Color: [4]float32{0, 0, 0, 1}, Erasures: [][]Point{{{50, 31, 6}}}}
		img := render(t, []Stroke{s}, 1, false)
		if alpha(img, 50, 31) != 0 || alpha(img, 50, 45) != 255 || alpha(img, 20, 31) != 255 {
			t.Fatal("pixel eraser removed whole stroke or painted opaque paper")
		}
		next := Stroke{Points: []Point{{50, 31, 2}}, Color: [4]float32{1, 0, 0, 1}}
		if alpha(render(t, []Stroke{s, next}, 1, false), 50, 31) != 255 {
			t.Fatal("erasure affected later ink")
		}
	}
}
func TestEraserSweptSegmentHasNoHoles(t *testing.T) {
	s := Stroke{Points: []Point{{10, 40, 10}, {50, 40, 10}, {90, 40, 10}}, Color: [4]float32{0, 0, 0, 1}, Erasures: [][]Point{{{20, 35, 3}, {80, 35, 3}}}}
	img := render(t, []Stroke{s}, 2, false)
	for x := 22; x < 79; x++ {
		if alpha(img, x*2, 35*2) != 0 {
			t.Fatal("fast eraser skipped between samples")
		}
	}
	if alpha(img, 50*2, 44*2) != 255 {
		t.Fatal("erase radius too large")
	}
}
func TestPencilEraserIsTransparent(t *testing.T) {
	s := Stroke{Tool: Pencil, Color: [4]float32{0, 0, 0, 1}, Stamps: []Stamp{{{50, 40, 1, 0}, {50, 30, 200, 0}}}, Erasures: [][]Point{{{50, 40, 30}}}}
	img := render(t, []Stroke{s}, 1, false)
	for _, v := range img.Pix {
		if v != 0 {
			t.Fatal("pencil eraser left opaque color")
		}
	}
}
func TestStrokeHitTestHandlesSweepsAndMisses(t *testing.T) {
	s := Stroke{Points: []Point{{10, 40, 2}, {50, 40, 2}, {90, 40, 2}}}
	if !HitsStroke(s, Point{X: 50, Y: 5, Radius: 3}, Point{X: 50, Y: 75, Radius: 3}) {
		t.Fatal("fast crossing missed")
	}
	if HitsStroke(s, Point{X: 50, Y: 3, Radius: 3}, Point{X: 80, Y: 3, Radius: 3}) {
		t.Fatal("distant stroke hit")
	}
}

func TestErasePreviewUsesOnePhysicalPixelEdge(t *testing.T) {
	for _, ratio := range []float32{1, 2} {
		r, err := NewRenderer()
		if err != nil {
			t.Fatal(err)
		}
		var preview ErasePreview
		page := ui.NewCanvas(int(50*ratio), int(40*ratio))
		ready := false
		center := float32(20) + 0.5/ratio
		path := []Point{{center, center, 5}}
		img := ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
				if !ready {
					page.Draw(p, func(cp *ui.Painter) { cp.Fill(ui.Rect{W: 50 * ratio, H: 40 * ratio}, ui.RGB(0, 0, 0), 0) })
					ready = true
				}
				preview.Draw(p, r, page.Bitmap(), path, Camera{Zoom: 1, PixelRatio: ratio}, 50, 40, [4]float32{1, 1, 1, 1})
			})
		}, 50, 40, ratio)
		edge := img.RGBAAt(int(25*ratio), int(20*ratio))
		if absInt(int(edge.R)-128) > 2 {
			t.Fatalf("DPR %g erase edge=%v", ratio, edge)
		}
		if img.RGBAAt(int(25*ratio)+1, int(20*ratio)).R != 0 {
			t.Fatal("erase preview AA wider than one pixel")
		}
	}
}

func TestViewportBoundsTemporaryMasksForHugeStroke(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	// Without viewport clipping this would request an enormous offscreen mask.
	s := Stroke{Points: []Point{{-100000, 20, 3}, {0, 20, 3}, {100000, 20, 3}}, Color: [4]float32{0, 0, 0, 1}}
	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) { r.DrawViewport(p, s, Camera{Zoom: 1, PixelRatio: 1}, 40, 40) })
	}, 40, 40, 1)
	if img.RGBAAt(20, 20).R != 0 {
		t.Fatal("clipped long stroke missing")
	}
}
