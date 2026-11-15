//go:build darwin

package metal

import (
	"bytes"
	"github.com/egoist/mygo/internal/scene"
	"testing"
)

func TestStudy408IdleRetentionKeepsCompletedPixels(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		t.Fatal("Metal device unavailable:", err)
	}
	defer r.Release()
	source := scene.NewCanvas(32, 32)
	source.KeepGPUOnIdle = true
	source.Scene.Ops = []scene.Op{{Kind: scene.OpFill, Rect: scene.Rect{W: 32, H: 32}, Color: scene.Color{R: 255, A: 255}}}
	spare := scene.NewCanvas(32, 32)
	spare.Scene.Ops = []scene.Op{{Kind: scene.OpFill, Rect: scene.Rect{W: 32, H: 32}, Color: scene.Color{G: 255, A: 255}}}
	s := &scene.Scene{Width: 64, Height: 32, Scale: 1, Ops: []scene.Op{
		{Kind: scene.OpImage, Rect: scene.Rect{W: 32, H: 32}, Src: scene.Rect{W: 32, H: 32}, Image: source, Opacity: 1},
		{Kind: scene.OpImage, Rect: scene.Rect{X: 32, W: 32, H: 32}, Src: scene.Rect{W: 32, H: 32}, Image: spare, Opacity: 1},
	}}
	before, err := r.renderOffscreen(s)
	if err != nil {
		t.Fatal(err)
	}
	retained := r.canvases[source.ID()]
	if retained == nil || !source.GPUContentsValid {
		t.Fatal("canvas not uploaded")
	}
	texture := retained.tex
	// The complete texture is authoritative. A changed incremental command list
	// must not be replayed merely because idle trimming released spare resources.
	source.Scene.Ops[0].Color = scene.Color{B: 255, A: 255}
	r.releaseTextures(true)
	if r.canvases[source.ID()] == nil || r.canvases[source.ID()].tex != texture || !source.GPUContentsValid {
		t.Fatal("idle released opted-in complete surface")
	}
	if r.canvases[spare.ID()] != nil || spare.GPUContentsValid || !spare.GPUContentsTrimmed {
		t.Fatal("spare canvas retained without opt-in or missing trimmed mark")
	}
	if source.GPUContentsTrimmed {
		t.Fatal("retained surface falsely marked trimmed")
	}
	after, err := r.renderOffscreen(s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("idle changed completed pixels")
	}
	r.releaseTextures(false)
	if source.GPUContentsValid || !source.GPUContentsTrimmed || len(r.canvases) != 0 {
		t.Fatal("renderer shutdown retained resources or missed trimmed mark")
	}
}
func TestStudy408IdleRetentionHonorsBudget(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	source := scene.NewCanvas(4, 4)
	source.KeepGPUOnIdle = true
	s := &scene.Scene{Width: 4, Height: 4, Scale: 1, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 4, H: 4}, Src: scene.Rect{W: 4, H: 4}, Image: source, Opacity: 1}}}
	if _, err := r.renderOffscreen(s); err != nil {
		t.Fatal(err)
	}
	tex := r.canvases[source.ID()]
	if tex == nil {
		t.Fatal("test surface missing")
	}
	// Test the budget using metadata, without allocating a giant GPU texture.
	tex.w, tex.h = 10000, 10000
	r.releaseTextures(true)
	if source.GPUContentsValid || r.canvases[source.ID()] != nil {
		t.Fatal("oversize retention bypassed fixed budget")
	}
}

func TestStudy408CompletedCanvasSkipsObsoleteSources(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	source := scene.NewCanvas(16, 16)
	source.Scene.Ops = []scene.Op{{Kind: scene.OpFill, Rect: scene.Rect{W: 16, H: 16}, Color: scene.Color{R: 255, A: 255}}}
	image := func(c *scene.Image) scene.Op {
		return scene.Op{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Src: scene.Rect{W: 16, H: 16}, Image: c, Opacity: 1}
	}
	complete := scene.NewCanvas(16, 16)
	complete.KeepGPUOnIdle = true
	complete.Scene.Ops = []scene.Op{image(source)}
	s := &scene.Scene{Width: 16, Height: 16, Scale: 1, Ops: []scene.Op{image(complete)}}
	before, err := r.renderOffscreen(s)
	if err != nil {
		t.Fatal(err)
	}
	version := r.canvases[source.ID()].ver
	// Reusing a scratch source must not make a completed viewport traverse or
	// rebuild that obsolete dependency. The viewport is its rendered pixels.
	source.Scene.Ops[0].Color = scene.Color{B: 255, A: 255}
	source.Changed()
	after, err := r.renderOffscreen(s)
	if err != nil || !bytes.Equal(before, after) || r.canvases[source.ID()].ver != version {
		t.Fatal("completed viewport replayed obsolete sources", err)
	}
	r.releaseTextures(true)
	after, err = r.renderOffscreen(s)
	if err != nil || !bytes.Equal(before, after) || r.canvases[source.ID()] != nil {
		t.Fatal("retained viewport reconstructed discarded scratch buffers", err)
	}
	// An explicitly changed viewport still traverses dependencies and rebuilds.
	complete.Changed()
	after, err = r.renderOffscreen(s)
	if err != nil || bytes.Equal(before, after) || r.canvases[source.ID()] == nil {
		t.Fatal("changed viewport failed to reconstruct its sources", err)
	}
}

func TestStudy408ZeroDamageRendersOffscreenPingPongCanvases(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		t.Fatal("Metal device unavailable:", err)
	}
	defer r.Release()
	r.cur = &r.formats[0]

	shown := scene.NewCanvas(32, 32)
	shown.Scene.Ops = []scene.Op{{Kind: scene.OpFill, Rect: scene.Rect{W: 32, H: 32}, Color: scene.Color{A: 255}}}
	top := &scene.Scene{
		Width: 32, Height: 32, Scale: 1,
		Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 32, H: 32}, Src: scene.Rect{W: 32, H: 32}, Image: shown, Opacity: 1}},
	}
	if _, err := r.renderOffscreen(top); err != nil {
		t.Fatal(err)
	}
	r.diff.Remember(top)

	layers := [2]*scene.Image{scene.NewCanvas(32, 32), scene.NewCanvas(32, 32)}
	copyOp := func(img *scene.Image) scene.Op {
		return scene.Op{Kind: scene.OpImage, Rect: scene.Rect{W: 32, H: 32}, Src: scene.Rect{W: 32, H: 32}, Image: img, Opacity: 1}
	}
	checkZeroDamage := func() {
		damage, known := r.diff.Diff(top)
		if !known || len(damage) != 0 {
			t.Fatalf("expected zero top-level damage, got %d known=%v", len(damage), known)
		}
		if err := r.renderCanvases(top); err != nil {
			t.Fatal(err)
		}
	}

	// Batch 1 -> layers[0] (red at top-left)
	layers[0].Scene.Reset(32, 32, scene.Color{})
	layers[0].Scene.Ops = []scene.Op{{Kind: scene.OpFill, Rect: scene.Rect{W: 12, H: 12}, Color: scene.Color{R: 255, A: 255}}}
	layers[0].Changed()
	top.Canvases = []*scene.Image{layers[0]}
	checkZeroDamage()
	if !layers[0].GPUContentsValid {
		t.Fatal("batch 1 offscreen canvas was not rasterized on zero-damage frame")
	}

	// Batch 2 -> layers[1] (layers[0] + green at top-right)
	layers[1].Scene.Reset(32, 32, scene.Color{})
	layers[1].Scene.Ops = []scene.Op{copyOp(layers[0]), {Kind: scene.OpFill, Rect: scene.Rect{X: 16, W: 12, H: 12}, Color: scene.Color{G: 255, A: 255}}}
	layers[1].Changed()
	top.Canvases = []*scene.Image{layers[1]}
	checkZeroDamage()
	if !layers[1].GPUContentsValid {
		t.Fatal("batch 2 offscreen canvas was not rasterized on zero-damage frame")
	}

	// Batch 3 overwrites layers[0] (layers[1] + blue at bottom-left)
	layers[0].Scene.Reset(32, 32, scene.Color{})
	layers[0].Scene.Ops = []scene.Op{copyOp(layers[1]), {Kind: scene.OpFill, Rect: scene.Rect{Y: 16, W: 12, H: 12}, Color: scene.Color{B: 255, A: 255}}}
	layers[0].Changed()
	top.Canvases = []*scene.Image{layers[0]}
	checkZeroDamage()

	// Final publish draws layers[0]; all 3 batches must survive.
	top.Canvases = nil
	top.Ops = []scene.Op{copyOp(layers[0])}
	pix, err := r.renderOffscreen(top)
	if err != nil {
		t.Fatal(err)
	}
	pixelAt := func(x, y int) (b, g, red, a byte) {
		i := (y*32 + x) * 4
		return pix[i], pix[i+1], pix[i+2], pix[i+3]
	}
	if _, _, red, a := pixelAt(4, 4); red < 250 || a < 250 {
		t.Fatalf("batch 1 red lost after 3-batch zero-damage ping-pong: r=%d a=%d", red, a)
	}
	if _, g, _, a := pixelAt(20, 4); g < 250 || a < 250 {
		t.Fatalf("batch 2 green lost after 3-batch zero-damage ping-pong: g=%d a=%d", g, a)
	}
	if blue, _, _, a := pixelAt(4, 20); blue < 250 || a < 250 {
		t.Fatalf("batch 3 blue lost after 3-batch zero-damage ping-pong: b=%d a=%d", blue, a)
	}
}
