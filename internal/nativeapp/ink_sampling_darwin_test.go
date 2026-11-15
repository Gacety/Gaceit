//go:build darwin

package nativeapp

import (
	"github.com/ebitengine/purego"
	"runtime"
	"testing"
)

func TestNativeInkSamplingRestoresProcessSetting(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	lib, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skip("AppKit unavailable", err)
	}
	defer purego.Dlclose(lib)
	restore := startInkSampling()
	defer restore()
	api := &inkSamplingAPI
	if api.getClass == nil {
		t.Fatal("Objective-C sampling API did not load")
	}
	event := api.getClass("NSEvent")
	if event == 0 {
		t.Fatal("NSEvent unavailable")
	}
	getter := api.selector("isMouseCoalescingEnabled")
	if api.getBool(event, getter) {
		t.Fatal("coalescing still enabled during ink input")
	}
	restore()
	// Preserve either initial state, including another component disabling it.
	before := api.getBool(event, getter)
	again := startInkSampling()
	if api.getBool(event, getter) {
		t.Fatal("second ink entry failed")
	}
	again()
	if api.getBool(event, getter) != before {
		t.Fatal("close did not restore previous setting")
	}
}
