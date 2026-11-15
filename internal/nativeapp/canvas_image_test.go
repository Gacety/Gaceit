package nativeapp

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"study408/internal/inkcanvas"
)

func TestCanvasImageStableAcrossZoom(t *testing.T) {
	a := testApp(t)
	a.M.Theme = "dark"
	a.M.Round.CurrentID = "relax-ds-05-024"
	a.SetTool("scratch")
	if !a.ensureInk() {
		t.Fatal(a.message)
	}
	a.inkHomePending = false
	tt := ui.NewTester(a.View, 1440, 960)
	for _, zoom := range []float64{1, 1.25, 1.5, 1.75, 2, 1.5, 1} {
		a.inkEditor.Load(inkcanvas.Document{Version: 1, Zoom: zoom, X: -720 * (zoom - 1), Y: -100})
		tt.Frame()
		if slices.Contains(tt.Texts(), "[image]") {
			t.Fatalf("image placeholder reappeared at %.2fx", zoom)
		}
	}
}

func TestQuestionImageSourceIsStable(t *testing.T) {
	r := NewRich(assetsForTest())
	url := "/practice/collection-assets/relax-1000-27/q-p25-0.jpg"
	a, err := r.image(url)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.image(url)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("image source was reloaded instead of reusing the cached bitmap")
	}
	w, h := ui.ImageSize(a)
	if w <= 0 || h <= 0 {
		t.Fatal("image source has no size")
	}
}
