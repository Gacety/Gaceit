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
	{"vendor/github.com/egoist/mygo/internal/web/surface.go", `	// downX and downY place the last press, where a context menu opens.
	downX, downY float64
	clicks       platform.ClickCounter
}`, `	// downX and downY place the last press, where a context menu opens.
	downX, downY float64
	clicks       platform.ClickCounter

	penDown                                          bool
	touches                                          map[int]touchPoint
	touchPrimaryID                                   int
	touchStartX, touchStartY, touchLastX, touchLastY float64
	touchStartTime, touchMoveTime, touchVX, touchVY  float64
	touchScrolling, touchPinching, touchPinched      bool
	pinchID1, pinchID2                               int
	pinchMidX, pinchMidY, pinchDist                  float64
	momentumReq                                      js.Value
	momentumFn                                       js.Func
	momentumActive                                   bool
	momentumLast                                     float64
}

type touchPoint struct {
	x, y float64
}`},
	{"vendor/github.com/egoist/mygo/internal/web/surface.go", `	// The pointer is the primary one of its kind (isPrimary): the mouse,
	// or the first finger or pen down. The others, as a second finger on
	// a touchscreen, neither press nor move it, as on Windows and Linux,
	// where a second finger presses nothing either: it would take the
	// press of the first, and end it as it lifts.
	s.on(c, "pointermove", true, func(e js.Value) {
		if !primary(e) {
			return
		}
		x, y := s.point(e)
		pointer, pressure := device(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerMove, X: x, Y: y, Mods: s.mods(e), Pointer: pointer, Pressure: pressure})
		// A button pressed or released while another is held comes as a
		// move, which pointerdown and pointerup leave out; the desktop's
		// windows hear each button.
		if kind, b, ok := chord(e.Get("button").Int(), e.Get("buttons").Int()); ok {
			ev := platform.SurfaceEvent{Kind: kind, X: x, Y: y, Button: b, Mods: s.mods(e), Pointer: pointer, Pressure: pressure}
			if kind == platform.PointerDown {
				s.downX, s.downY = x, y
				ev.Clicks = s.clicks.Press(b, x, y, uint32(e.Get("timeStamp").Float()), platform.DefaultDoubleClick)
			}
			s.send(ev)
		}
	})
	s.on(c, "pointerdown", false, func(e js.Value) {
		e.Call("preventDefault")
		if !primary(e) {
			return
		}
		s.focus()
		x, y := s.point(e)
		button := e.Get("button").Int()
		switch button {
		case 3, 4:
			k := platform.KeyBack
			if button == 4 {
				k = platform.KeyForward
			}
			s.send(platform.SurfaceEvent{Kind: platform.KeyPressed, Key: k, Mods: s.mods(e)})
			s.send(platform.SurfaceEvent{Kind: platform.KeyReleased, Key: k, Mods: s.mods(e)})
			return
		}
		c.Call("setPointerCapture", e.Get("pointerId"))
		b := buttonOf(button)
		s.downX, s.downY = x, y
		clicks := s.clicks.Press(b, x, y, uint32(e.Get("timeStamp").Float()), platform.DefaultDoubleClick) // browsers tell no double-click settings
		pointer, pressure := device(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerDown, X: x, Y: y, Button: b, Clicks: clicks, Mods: s.mods(e), Pointer: pointer, Pressure: pressure})
	})
	s.on(c, "pointerup", true, func(e js.Value) {
		if !primary(e) {
			return
		}
		button := e.Get("button").Int()
		if button == 3 || button == 4 {
			return
		}
		x, y := s.point(e)
		pointer, _ := device(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerUp, X: x, Y: y, Button: buttonOf(button), Mods: s.mods(e), Pointer: pointer})
	})
	// The browser took the pointer, as a touch it scrolls or zooms the
	// page with, or one the system took: its buttons go up with no
	// release, and the press commits nothing.
	s.on(c, "pointercancel", true, func(e js.Value) {
		if !primary(e) {
			return
		}
		x, y := s.point(e)
		pointer, _ := device(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerCancel, X: x, Y: y, Mods: s.mods(e), Pointer: pointer})
	})
	s.on(c, "pointerleave", true, func(e js.Value) {
		if !primary(e) {
			return
		}
		s.send(platform.SurfaceEvent{Kind: platform.PointerLeave})
	})`, `	s.touches = map[int]touchPoint{}
	s.momentumFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		s.w.b.sync(func() {
			s.stepMomentum(args)
		})
		return nil
	})
	s.funcs = append(s.funcs, s.momentumFn)

	// The pointer is the primary one of its kind (isPrimary): the mouse,
	// or the first finger or pen down. The others, as a second finger on
	// a touchscreen, neither press nor move it, as on Windows and Linux,
	// where a second finger presses nothing either: it would take the
	// press of the first, and end it as it lifts.
	s.on(c, "pointermove", true, func(e js.Value) {
		pointer, pressure := device(e)
		if !s.penDown && pointer == platform.PointerTouch {
			s.touchMove(e, pressure)
			return
		}
		if !primary(e) {
			return
		}
		x, y := s.point(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerMove, X: x, Y: y, Mods: s.mods(e), Pointer: pointer, Pressure: pressure})
		// A button pressed or released while another is held comes as a
		// move, which pointerdown and pointerup leave out; the desktop's
		// windows hear each button.
		if kind, b, ok := chord(e.Get("button").Int(), e.Get("buttons").Int()); ok {
			ev := platform.SurfaceEvent{Kind: kind, X: x, Y: y, Button: b, Mods: s.mods(e), Pointer: pointer, Pressure: pressure}
			if kind == platform.PointerDown {
				s.downX, s.downY = x, y
				ev.Clicks = s.clicks.Press(b, x, y, uint32(e.Get("timeStamp").Float()), platform.DefaultDoubleClick)
			}
			s.send(ev)
		}
	})
	s.on(c, "pointerdown", false, func(e js.Value) {
		e.Call("preventDefault")
		pointer, pressure := device(e)
		if pointer == platform.PointerPen {
			s.stopMomentum()
			s.penDown = true
			clear(s.touches)
			s.touchScrolling, s.touchPinching, s.touchPinched = false, false, false
		} else if !s.penDown && pointer == platform.PointerTouch {
			c.Call("setPointerCapture", e.Get("pointerId"))
			s.touchDown(e, pressure)
			return
		}
		if !primary(e) {
			return
		}
		s.focus()
		x, y := s.point(e)
		button := e.Get("button").Int()
		switch button {
		case 3, 4:
			k := platform.KeyBack
			if button == 4 {
				k = platform.KeyForward
			}
			s.send(platform.SurfaceEvent{Kind: platform.KeyPressed, Key: k, Mods: s.mods(e)})
			s.send(platform.SurfaceEvent{Kind: platform.KeyReleased, Key: k, Mods: s.mods(e)})
			return
		}
		c.Call("setPointerCapture", e.Get("pointerId"))
		b := buttonOf(button)
		s.downX, s.downY = x, y
		clicks := s.clicks.Press(b, x, y, uint32(e.Get("timeStamp").Float()), platform.DefaultDoubleClick) // browsers tell no double-click settings
		s.send(platform.SurfaceEvent{Kind: platform.PointerDown, X: x, Y: y, Button: b, Clicks: clicks, Mods: s.mods(e), Pointer: pointer, Pressure: pressure})
	})
	s.on(c, "pointerup", true, func(e js.Value) {
		pointer, _ := device(e)
		if pointer == platform.PointerPen {
			s.penDown = false
		} else if !s.penDown && pointer == platform.PointerTouch {
			s.touchUp(e, false)
			return
		}
		if !primary(e) {
			return
		}
		button := e.Get("button").Int()
		if button == 3 || button == 4 {
			return
		}
		x, y := s.point(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerUp, X: x, Y: y, Button: buttonOf(button), Mods: s.mods(e), Pointer: pointer})
	})
	// The browser took the pointer, as a touch it scrolls or zooms the
	// page with, or one the system took: its buttons go up with no
	// release, and the press commits nothing.
	s.on(c, "pointercancel", true, func(e js.Value) {
		pointer, _ := device(e)
		if pointer == platform.PointerPen {
			s.penDown = false
		} else if !s.penDown && pointer == platform.PointerTouch {
			s.touchUp(e, true)
			return
		}
		if !primary(e) {
			return
		}
		x, y := s.point(e)
		s.send(platform.SurfaceEvent{Kind: platform.PointerCancel, X: x, Y: y, Mods: s.mods(e), Pointer: pointer})
	})
	s.on(c, "pointerleave", true, func(e js.Value) {
		pointer, _ := device(e)
		if pointer == platform.PointerPen {
			s.penDown = false
		}
		if !primary(e) {
			return
		}
		s.send(platform.SurfaceEvent{Kind: platform.PointerLeave})
	})`},
	{"vendor/github.com/egoist/mygo/internal/web/surface.go", `	gesture := func(e js.Value) {
		e.Call("preventDefault")
		scale := e.Get("scale").Float()`, `	gesture := func(e js.Value) {
		e.Call("preventDefault")
		if len(s.touches) >= 2 {
			return
		}
		scale := e.Get("scale").Float()`},
	{"vendor/github.com/egoist/mygo/internal/web/surface.go", `func (s *surface) close() {
	s.closed = true
	if s.pending {
		js.Global().Call("cancelAnimationFrame", s.request)
	}
	if !s.observer.IsUndefined() {`, `func (s *surface) close() {
	s.closed = true
	if s.pending {
		js.Global().Call("cancelAnimationFrame", s.request)
	}
	s.stopMomentum()
	if !s.observer.IsUndefined() {`},
	{"vendor/github.com/egoist/mygo/internal/web/surface.go", `func itoa(n int) string     { return strconv.Itoa(n) }
func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }`, `func (s *surface) initPinch(preferA, preferB int) bool {
	if len(s.touches) < 2 {
		return false
	}
	_, okA := s.touches[preferA]
	_, okB := s.touches[preferB]
	if okA && okB && preferA != preferB {
		s.pinchID1, s.pinchID2 = preferA, preferB
	} else {
		i := 0
		for id := range s.touches {
			if i == 0 {
				s.pinchID1 = id
			} else {
				s.pinchID2 = id
				break
			}
			i++
		}
	}
	p1, p2 := s.touches[s.pinchID1], s.touches[s.pinchID2]
	s.pinchMidX = (p1.x + p2.x) * 0.5
	s.pinchMidY = (p1.y + p2.y) * 0.5
	s.pinchDist = math.Hypot(p2.x-p1.x, p2.y-p1.y)
	return true
}

func (s *surface) touchDown(e js.Value, pressure float64) {
	s.stopMomentum()
	pid := e.Get("pointerId").Int()
	x, y := s.point(e)
	ts := e.Get("timeStamp").Float()
	s.touches[pid] = touchPoint{x: x, y: y}
	if len(s.touches) == 1 {
		s.focus()
		s.touchPrimaryID = pid
		s.touchStartX, s.touchStartY = x, y
		s.touchLastX, s.touchLastY = x, y
		s.touchStartTime, s.touchMoveTime = ts, ts
		s.touchVX, s.touchVY = 0, 0
		s.touchScrolling, s.touchPinching, s.touchPinched = false, false, false
		s.downX, s.downY = x, y
		clicks := s.clicks.Press(0, x, y, uint32(ts), platform.DefaultDoubleClick)
		s.send(platform.SurfaceEvent{Kind: platform.PointerDown, X: x, Y: y, Button: 0, Clicks: clicks, Mods: s.mods(e), Pointer: platform.PointerTouch, Pressure: pressure})
		return
	}
	if !s.touchScrolling && !s.touchPinching {
		s.send(platform.SurfaceEvent{Kind: platform.PointerCancel, X: s.touchStartX, Y: s.touchStartY, Mods: s.mods(e), Pointer: platform.PointerTouch})
	}
	s.touchScrolling = true
	s.touchPinching = true
	s.touchPinched = true
	s.touchVX, s.touchVY = 0, 0
	s.initPinch(s.touchPrimaryID, pid)
}

func (s *surface) touchMove(e js.Value, pressure float64) {
	pid := e.Get("pointerId").Int()
	if _, ok := s.touches[pid]; !ok {
		return
	}
	x, y := s.point(e)
	ts := e.Get("timeStamp").Float()
	s.touches[pid] = touchPoint{x: x, y: y}
	if len(s.touches) >= 2 {
		if !s.touchPinching {
			s.touchScrolling = true
			s.touchPinching = true
			s.touchPinched = true
			s.initPinch(s.touchPrimaryID, pid)
			return
		}
		p1, ok1 := s.touches[s.pinchID1]
		p2, ok2 := s.touches[s.pinchID2]
		if !ok1 || !ok2 {
			s.initPinch(s.touchPrimaryID, pid)
			return
		}
		midX := (p1.x + p2.x) * 0.5
		midY := (p1.y + p2.y) * 0.5
		dist := math.Hypot(p2.x-p1.x, p2.y-p1.y)
		mods := s.mods(e)
		if s.pinchDist > 8 && dist > 8 && math.Abs(dist/s.pinchDist-1) > 0.002 {
			s.send(platform.SurfaceEvent{Kind: platform.PointerZoom, X: midX, Y: midY, Scale: dist / s.pinchDist, Mods: mods})
			s.pinchDist = dist
		} else if s.pinchDist <= 8 && dist > 8 {
			s.pinchDist = dist
		}
		if midX != s.pinchMidX || midY != s.pinchMidY {
			s.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: midX, Y: midY, DX: s.pinchMidX - midX, DY: s.pinchMidY - midY, Precise: true, Mods: mods})
			s.pinchMidX, s.pinchMidY = midX, midY
		}
		s.touchLastX, s.touchLastY = midX, midY
		return
	}
	s.touchPrimaryID = pid
	if !s.touchScrolling {
		dx0, dy0 := x-s.touchStartX, y-s.touchStartY
		elapsed := ts - s.touchStartTime
		if elapsed < 450 && dx0*dx0+dy0*dy0 > 64 {
			s.touchScrolling = true
			mods := s.mods(e)
			s.send(platform.SurfaceEvent{Kind: platform.PointerCancel, X: s.touchStartX, Y: s.touchStartY, Mods: mods, Pointer: platform.PointerTouch})
			s.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: x, Y: y, DX: s.touchStartX - x, DY: s.touchStartY - y, Precise: true, Mods: mods})
			if dt := ts - s.touchStartTime; dt > 0 && dt < 200 {
				s.touchVX = max(-4.5, min(4.5, (x-s.touchStartX)/dt))
				s.touchVY = max(-4.5, min(4.5, (y-s.touchStartY)/dt))
			}
			s.touchLastX, s.touchLastY = x, y
			s.touchMoveTime = ts
			return
		}
		if elapsed < 450 {
			return
		}
		s.send(platform.SurfaceEvent{Kind: platform.PointerMove, X: x, Y: y, Mods: s.mods(e), Pointer: platform.PointerTouch, Pressure: pressure})
		s.touchLastX, s.touchLastY = x, y
		s.touchMoveTime = ts
		return
	}
	dx := s.touchLastX - x
	dy := s.touchLastY - y
	if dt := ts - s.touchMoveTime; dt > 0 && dt < 120 {
		ivx := max(-4.5, min(4.5, (x-s.touchLastX)/dt))
		ivy := max(-4.5, min(4.5, (y-s.touchLastY)/dt))
		s.touchVX = s.touchVX*0.4 + ivx*0.6
		s.touchVY = s.touchVY*0.4 + ivy*0.6
	} else if ts-s.touchMoveTime >= 120 {
		s.touchVX, s.touchVY = 0, 0
	}
	s.touchLastX, s.touchLastY = x, y
	s.touchMoveTime = ts
	if dx != 0 || dy != 0 {
		s.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: x, Y: y, DX: dx, DY: dy, Precise: true, Mods: s.mods(e)})
	}
}

func (s *surface) touchUp(e js.Value, canceled bool) {
	pid := e.Get("pointerId").Int()
	x, y := s.point(e)
	ts := e.Get("timeStamp").Float()
	delete(s.touches, pid)
	if s.touchScrolling || s.touchPinching {
		if len(s.touches) == 0 {
			shouldFling := !canceled && !s.touchPinched && ts-s.touchMoveTime <= 60 && math.Hypot(s.touchVX, s.touchVY) > 0.18
			s.touchScrolling = false
			s.touchPinching = false
			s.touchPinched = false
			if shouldFling {
				s.startMomentum()
			} else {
				s.touchVX, s.touchVY = 0, 0
			}
			return
		}
		if len(s.touches) == 1 {
			s.touchPinching = false
			for id, pt := range s.touches {
				s.touchPrimaryID = id
				s.touchLastX, s.touchLastY = pt.x, pt.y
				s.touchMoveTime = ts
				s.touchVX, s.touchVY = 0, 0
				break
			}
		}
		return
	}
	s.touchVX, s.touchVY = 0, 0
	kind := platform.PointerUp
	if canceled {
		kind = platform.PointerCancel
	}
	s.send(platform.SurfaceEvent{Kind: kind, X: x, Y: y, Button: 0, Mods: s.mods(e), Pointer: platform.PointerTouch})
}

func (s *surface) stopMomentum() {
	if s.momentumActive {
		js.Global().Call("cancelAnimationFrame", s.momentumReq)
		s.momentumActive = false
	}
	s.touchVX, s.touchVY = 0, 0
}

func (s *surface) startMomentum() {
	if s.closed {
		return
	}
	if s.momentumActive {
		js.Global().Call("cancelAnimationFrame", s.momentumReq)
	}
	s.momentumActive = true
	s.momentumLast = 0
	s.momentumReq = js.Global().Call("requestAnimationFrame", s.momentumFn)
}

func (s *surface) stepMomentum(args []js.Value) {
	if !s.momentumActive || s.closed {
		s.momentumActive = false
		return
	}
	now := 0.0
	if len(args) > 0 && args[0].Type() == js.TypeNumber {
		now = args[0].Float()
	}
	dt := 16.6
	if s.momentumLast != 0 && now > s.momentumLast {
		dt = min(50, now-s.momentumLast)
	}
	s.momentumLast = now
	decay := math.Exp(-dt / 320.0)
	dx := -s.touchVX * dt
	dy := -s.touchVY * dt
	s.touchVX *= decay
	s.touchVY *= decay
	if math.Abs(dx) >= 0.25 || math.Abs(dy) >= 0.25 {
		s.send(platform.SurfaceEvent{
			Kind:     platform.PointerScroll,
			X:        s.touchLastX,
			Y:        s.touchLastY,
			DX:       dx,
			DY:       dy,
			Precise:  true,
			Momentum: true,
		})
	}
	if math.Hypot(s.touchVX, s.touchVY) < 0.03 {
		s.momentumActive = false
		s.touchVX, s.touchVY = 0, 0
		return
	}
	s.momentumReq = js.Global().Call("requestAnimationFrame", s.momentumFn)
}

func itoa(n int) string     { return strconv.Itoa(n) }
func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }`},
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
