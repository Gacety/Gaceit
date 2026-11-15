package main

import (
	"fmt"
	"os"
	"strings"
)

type sourcePatch struct{ path, old, new string }

var idlePatches = []sourcePatch{
	{"vendor/github.com/egoist/mygo/internal/scene/scene.go", `func (s *Scene) EachCanvas(fn func(*Image)) {
	seen := map[*Image]bool{}`, `func (s *Scene) EachCanvas(fn func(*Image)) {
 s.EachCanvasNeeded(nil, fn)
}

// EachCanvasNeeded stops at backend-confirmed complete textures. fn still
// visits the cached image so its backend lifetime is updated. A missing or
// changed texture traverses all dependencies, preserving the rebuild fallback.
func (s *Scene) EachCanvasNeeded(needsSources func(*Image) bool, fn func(*Image)) {
 seen := map[*Image]bool{}`},
	{"vendor/github.com/egoist/mygo/internal/scene/scene.go", `seen[c] = true
		c.Scene.eachShown(visit)
		fn(c)`, `seen[c] = true
  if needsSources == nil || needsSources(c) {
   c.Scene.eachShown(visit)
  }
  fn(c)`},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", `// prepare starts the command buffer of a frame of s, with its canvases,
// and builds the frame (build), returning the scene built.
func (r *Renderer) prepare(s *scene.Scene) (cb id, built *scene.Scene, err error) {
	if err := r.initGPU(); err != nil {
		return 0, nil, err
	}
	if r.cur.pipeline == 0 {
		if err := r.makePipelines(r.cur); err != nil {
			return 0, nil, err
		}
	}
	r.inUse = r.slot
	r.waitSlot(r.inUse)
	if r.err != nil {
		return 0, nil, r.err
	}
	r.frame++
	if err := r.syncAtlas(&r.mask, s.MaskAtlas, pixelFormatR8Unorm); err != nil {
		return 0, nil, err
	}
	if err := r.syncAtlas(&r.color, s.ColorAtlas, pixelFormatRGBA8Unorm); err != nil {
		return 0, nil, err
	}
	r.pipes.fallbacks = 0
	r.used = 0
	cb = send(r.queue, "commandBuffer")
	if !r.checked {
		if err := need(cb, "command buffer", "renderCommandEncoderWithDescriptor:", "commit", "presentDrawable:", "waitUntilScheduled", "waitUntilCompleted", "status", "blitCommandEncoder"); err != nil {
			return 0, nil, err
		}
	}
	// Canvases first, into textures of their own, which the frame samples.
	s.EachCanvas(func(c *scene.Image) {
		if err == nil {
			err = r.drawCanvas(cb, c)
		}
	})
	if err != nil {
		return 0, nil, err
	}
	r.b.Wide = r.cur.format == pixelFormatRGBA16Float
	if built, err = r.build(cb, s, &r.drawState); err != nil {
		return 0, nil, err
	}
	return cb, built, nil
}`, `func (r *Renderer) canvasNeedsDraw(c *scene.Image) bool {
	t := r.canvases[c.ID()]
	return t == nil || t.tex == 0 || t.w != c.W || t.h != c.H || t.ver != c.Version()
}

func (r *Renderer) hasStaleCanvases(s *scene.Scene) bool {
	for _, c := range s.Canvases {
		if c != nil && c.Scene != nil && r.canvasNeedsDraw(c) {
			return true
		}
	}
	return false
}

func (r *Renderer) prepareCanvases(s *scene.Scene) (cb id, err error) {
	if err := r.initGPU(); err != nil {
		return 0, err
	}
	if r.cur.pipeline == 0 {
		if err := r.makePipelines(r.cur); err != nil {
			return 0, err
		}
	}
	r.inUse = r.slot
	r.waitSlot(r.inUse)
	if r.err != nil {
		return 0, r.err
	}
	r.frame++
	if err := r.syncAtlas(&r.mask, s.MaskAtlas, pixelFormatR8Unorm); err != nil {
		return 0, err
	}
	if err := r.syncAtlas(&r.color, s.ColorAtlas, pixelFormatRGBA8Unorm); err != nil {
		return 0, err
	}
	r.pipes.fallbacks = 0
	r.used = 0
	cb = send(r.queue, "commandBuffer")
	if !r.checked {
		if err := need(cb, "command buffer", "renderCommandEncoderWithDescriptor:", "commit", "presentDrawable:", "waitUntilScheduled", "waitUntilCompleted", "status", "blitCommandEncoder"); err != nil {
			return 0, err
		}
	}
	// Canvases first, into textures of their own, which the frame samples.
	s.EachCanvasNeeded(r.canvasNeedsDraw, func(c *scene.Image) {
		if err == nil {
			err = r.drawCanvas(cb, c)
		}
	})
	if err != nil {
		return 0, err
	}
	return cb, nil
}

// renderCanvases draws offscreen canvases even when top-level scene ops had no damage.
func (r *Renderer) renderCanvases(s *scene.Scene) error {
	if !r.hasStaleCanvases(s) {
		return nil
	}
	cb, err := r.prepareCanvases(s)
	if err != nil {
		return err
	}
	send(cb, "commit")
	r.track(cb)
	r.lastRender = time.Now()
	r.armTrim()
	r.sweepIfDue()
	return nil
}

// prepare starts the command buffer of a frame of s, with its canvases,
// and builds the frame (build), returning the scene built.
func (r *Renderer) prepare(s *scene.Scene) (cb id, built *scene.Scene, err error) {
	cb, err = r.prepareCanvases(s)
	if err != nil {
		return 0, nil, err
	}
	r.b.Wide = r.cur.format == pixelFormatRGBA16Float
	if built, err = r.build(cb, s, &r.drawState); err != nil {
		return 0, nil, err
	}
	return cb, built, nil
}`},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", `	damage, known := r.diff.Diff(s)
	if known && len(damage) == 0 && !sync {
		r.redrew = redrewNothing
		return nil
	}`, `	damage, known := r.diff.Diff(s)
	if known && len(damage) == 0 && !sync {
		if err := r.renderCanvases(s); err != nil {
			return err
		}
		r.redrew = redrewNothing
		return nil
	}`},
	{"vendor/github.com/egoist/mygo/ui/context.go", "pressPressure float32", "pressPressure float32\n penIsolation func(InputEvent)"},
	{"vendor/github.com/egoist/mygo/ui/handler.go", `func (e *node) HandleInput(fn func(ev InputEvent) bool) *node {
	e.inputFn = fn
	return e
}`, `func (e *node) HandleInput(fn func(ev InputEvent) bool) *node {
 e.inputFn = fn
 return e
}

// IsolatePenInput keeps other devices from changing an active pen capture.
// Only explicitly opted-in drawing surfaces use it. Genuine system cancellation
// still ends the capture. ignored receives metadata without changing UI state.
func (e Element) IsolatePenInput(ignored func(InputEvent)) Element {
 if n := e.node(); n != nil && n.hasState() { n.st.penIsolation = ignored }
 return e
}`},
	{"vendor/github.com/egoist/mygo/ui/input.go", `func (rt *engine) event(ev platform.SurfaceEvent) (taken bool) {`, `func (rt *engine) event(ev platform.SurfaceEvent) (taken bool) {
 // Ignore foreign pointer events BEFORE they can mutate capture or cursor.
 if s := rt.pressed; s != nil && s.penIsolation != nil && s.pressPointer == PointerPen && ev.Pointer != platform.PointerPen {
  kind := InputKind(0)
  switch ev.Kind {
  case platform.PointerDown: kind = InputPointerDown
  case platform.PointerMove: kind = InputPointerMove
  case platform.PointerUp: kind = InputPointerUp
  }
  if kind != 0 {
   s.penIsolation(InputEvent{Kind: kind, Pointer: PointerType(ev.Pointer), Time: rt.now()})
   return true
  }
 }
`},
	{"vendor/github.com/egoist/mygo/internal/scene/scene.go", "type Image struct {", `type Image struct {
	// Opt-in retention for the application's last complete ink viewport.
	// Read/write only on the UI/render thread; other backends leave validity false.
	KeepGPUOnIdle      bool
	GPUContentsValid   bool
	GPUContentsTrimmed bool`},
	{"vendor/github.com/egoist/mygo/internal/scene/scene.go", "func (m *Image) Changed() { m.version++ }", `func (m *Image) Changed() {
	m.version++
	m.GPUContentsValid = false
	m.GPUContentsTrimmed = false
}`},
	{"vendor/github.com/egoist/mygo/ui/canvas.go", "func (cv *Canvas) Size() (w, h int) { return cv.img.W, cv.img.H }", `func (cv *Canvas) Size() (w, h int) { return cv.img.W, cv.img.H }

// KeepOnIdle requests bounded preservation of this completed surface by Metal.
// It schedules no frames. Change it on the UI/render thread only.
func (cv *Canvas) KeepOnIdle(on bool) { cv.img.KeepGPUOnIdle = on }

// HasGPUContents reports whether the backend still owns a complete texture.
// False also means an unsupported/software backend; rebuild from vectors there.
func (cv *Canvas) HasGPUContents() bool { return cv.img.GPUContentsValid }

// Rendered reports whether the active backend already holds this canvas's
// current version (Metal texture or software Pix).
func (cv *Canvas) Rendered() bool {
	return cv.img.GPUContentsValid || (cv.img.PixVersion == cv.img.Version() && len(cv.img.Pix) == 4*cv.img.W*cv.img.H)
}

// Trimmed reports whether a previously rendered GPU backing texture was
// evicted by idle or shutdown texture trimming since the last Draw call.
func (cv *Canvas) Trimmed() bool {
	return cv != nil && cv.img != nil && cv.img.GPUContentsTrimmed
}

// Flush queues this canvas to be rasterized on this frame if not yet rendered.
func (cv *Canvas) Flush(p *Painter) {
	if !cv.Rendered() {
		p.rt.scene.Canvases = append(p.rt.scene.Canvases, cv.img)
	}
}`},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", "func (r *Renderer) releaseTextures() {", "func (r *Renderer) releaseTextures(keepLiveCanvases bool) {"},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", `for key, t := range r.canvases {
		release(&t.tex)
		t.drawState.release()
		delete(r.canvases, key)
	}`, `const maxIdleCanvasBytes = 128 << 20
	kept := int64(0)
	for key, t := range r.canvases {
		img := t.canvas.Value()
		bytes := int64(t.w) * int64(t.h) * 4
		if keepLiveCanvases && img != nil && img.KeepGPUOnIdle && t.tex != 0 && bytes <= maxIdleCanvasBytes-kept {
			kept += bytes
			t.drawState.release()
			continue
		}
		if img != nil {
			img.GPUContentsValid = false
			img.GPUContentsTrimmed = true
		}
		release(&t.tex)
		t.drawState.release()
		delete(r.canvases, key)
	}`},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", `t.ver = c.Version()
	return nil`, `t.ver = c.Version()
	c.GPUContentsValid = true
	c.GPUContentsTrimmed = false
	return nil`},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", `r.releaseTextures()
		r.ages.Reset()
		r.diff.Release()`, `r.releaseTextures(true)
		r.ages.Reset()
		r.diff.Release()`},
	{"vendor/github.com/egoist/mygo/internal/gpu/metal/metal.go", `r.releaseTextures()
		r.ages.Reset()
		release(&r.empty)`, `r.releaseTextures(false)
		r.ages.Reset()
		release(&r.empty)`},
	{"vendor/github.com/egoist/mygo/ui/context.go", "type state struct {", "type state struct {\n penInputOnly bool"},
	{"vendor/github.com/egoist/mygo/ui/runtime.go", "type engine struct {", "type engine struct {\n penHitTest bool"},
	{"vendor/github.com/egoist/mygo/ui/handler.go", "// ObserveInput has fn see the input", `// PenInputOnly makes a transparent ink overlay ignore mouse/trackpad hits.
// Higher controls retain priority. Capture and cancellation stay native.
func (e Element) PenInputOnly() Element {
 if n := e.node(); n != nil && n.hasState() {
  n.st.penInputOnly = true
 }
 return e
}

// ObserveInput has fn see the input`},
	{"vendor/github.com/egoist/mygo/ui/input.go", `if ev.Kind == platform.SurfaceCommandEnabled {`, `switch ev.Kind {
 case platform.PointerDown, platform.PointerMove, platform.PointerUp:
  rt.penHitTest = ev.Pointer == platform.PointerPen
 case platform.PointerScroll, platform.PointerZoom, platform.PointerLeave:
  rt.penHitTest = false
 }
 if ev.Kind == platform.SurfaceCommandEnabled {`},
	{"vendor/github.com/egoist/mygo/ui/input.go", `if !rt.contains(h.st, h.r, x, y) {`, `if h.st.penInputOnly && !rt.penHitTest {
   continue
  }
  if !rt.contains(h.st, h.r, x, y) {`},
}

func applySourcePatch(src string, p sourcePatch) (string, error) {
	// Ignore indentation-only differences after gofmt, but match full tokens.
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	if strings.Contains(normalize(src), normalize(p.new)) {
		return src, nil
	}
	if strings.Count(src, p.old) != 1 {
		return "", fmt.Errorf("MyGo native patch source changed: %s; review pinned patch", p.path)
	}
	return strings.Replace(src, p.old, p.new, 1), nil
}
func patchIdleSources() error {
	sources := map[string]string{}
	for _, p := range idlePatches {
		src, ok := sources[p.path]
		if !ok {
			data, err := os.ReadFile(p.path)
			if err != nil {
				return err
			}
			src = string(data)
		}
		updated, err := applySourcePatch(src, p)
		if err != nil {
			return err
		}
		sources[p.path] = updated
	}
	for path, src := range sources {
		if prior, err := os.ReadFile(path); err == nil && string(prior) == src {
			continue
		}
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			return err
		}
	}
	return nil
}
