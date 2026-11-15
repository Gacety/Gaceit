//go:build darwin

package nativeapp

import (
	"github.com/ebitengine/purego"
	"sync"
)

var inkSamplingAPI struct {
	once     sync.Once
	getClass func(string) uintptr
	selector func(string) uintptr
	getBool  func(uintptr, uintptr) bool
	setBool  func(uintptr, uintptr, bool)
}

// AppKit normally merges queued mouse/drag/tablet movement. For handwriting,
// retain actual samples while the native canvas is open, then restore the
// previous process-local setting. Calls originate on MyGo's UI thread only.
func startInkSampling() func() {
	api := &inkSamplingAPI
	api.once.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&api.getClass, lib, "objc_getClass")
		purego.RegisterLibFunc(&api.selector, lib, "sel_registerName")
		purego.RegisterLibFunc(&api.getBool, lib, "objc_msgSend")
		purego.RegisterLibFunc(&api.setBool, lib, "objc_msgSend")
	})
	if api.getClass == nil {
		return func() {}
	}
	event := api.getClass("NSEvent")
	if event == 0 {
		return func() {}
	}
	previous := api.getBool(event, api.selector("isMouseCoalescingEnabled"))
	setter := api.selector("setMouseCoalescingEnabled:")
	api.setBool(event, setter, false)
	return func() { api.setBool(event, setter, previous) }
}
