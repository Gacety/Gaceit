package ink

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestPreparedGeometryHasBoundedMemoryAndScaleKeys(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	s := Stroke{Points: []Point{{0, 0, 2}, {10, 10, 2}, {20, 0, 2}}, Smooth: true}
	first := r.completedSegments(s, 1)
	again := r.completedSegments(s, 1)
	if &first[0] != &again[0] {
		t.Fatal("immutable geometry not reused")
	}
	scaled := r.completedSegments(s, .5)
	if &scaled[0] == &first[0] {
		t.Fatal("different pixel size reused wrong AA geometry")
	}
	for i := 0; i < 1000; i++ {
		s.Translation = [2]float32{float32(i), 0}
		r.completedSegments(s, 1)
	}
	if len(r.geometry.entries) > maxPreparedStrokes || r.geometry.total > maxPreparedSegments {
		t.Fatal("geometry cache exceeded budget")
	}
	r.ReleaseGeometryCache()
	if r.geometry.entries != nil || r.geometry.total != 0 {
		t.Fatal("cache release retained data")
	}
}

func TestSmallStrokeMasksAreBoundedAndReusable(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	s := Stroke{Points: []Point{{10, 10, 2}, {30, 30, 2}, {50, 10, 2}}, Smooth: true, Color: [4]float32{0, 0, 0, 1}}
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, box ui.Rect) { r.DrawRegion(p, s, Camera{Zoom: 1, PixelRatio: 1}, box) })
	}
	tt := ui.NewTester(view, 100, 80)
	if len(r.masks.entries) != 1 {
		t.Fatal("small complete mask not cached")
	}
	var first *ui.Canvas
	for _, cv := range r.masks.entries {
		first = cv
	}
	tt.Frame()
	if len(r.masks.entries) != 1 {
		t.Fatal("unchanged stroke allocated new mask")
	}
	for _, cv := range r.masks.entries {
		if cv != first {
			t.Fatal("mask not reused")
		}
	}
	for i := 0; i < 300; i++ {
		s.Translation = [2]float32{float32(i % 10), float32(i / 10)}
		tt.Frame()
	}
	if r.masks.pixels > maxMaskPixels || len(r.masks.entries) > maxMaskEntries || r.masks.segments > maxMaskSegments {
		t.Fatal("stroke mask cache exceeded budget")
	}
	r.ReleaseGeometryCache()
	if r.masks.entries != nil {
		t.Fatal("closing canvas retained cached masks")
	}
}
