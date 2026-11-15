// Command mygo-tablet-patch applies pinned native input and idle-cache fixes to vendor.
// Reapply after go mod vendor; the module cache is never modified.
package main

import (
	"fmt"
	"os"
	"strings"
)

const oldTabletPoint = `func (s *surface) tabletPoint(ev id) {
 p := msgPoint(class("NSEvent"), sel("mouseLocation"))
 inWindow := msgRectToRect(s.w.win, sel("convertRectFromScreen:"), NSRect{Origin: p})
 local := msgPointFromView(s.view, sel("convertPoint:fromView:"), inWindow.Origin, 0)
 pressure := float64(msgFloat32(ev, sel("pressure")))
 s.send(platform.SurfaceEvent{Kind: platform.PointerMove, X: local.X, Y: local.Y, Mods: eventMods(ev), Pointer: platform.PointerPen, Pressure: pressure})
}`
const newTabletPoint = `func (s *surface) tabletPoint(ev id) {
 // Read the position stored in THIS event, not the newer global cursor.
 // Queued tablet samples must retain their own curve and pressure pairing.
 x, y := s.location(ev)
 pressure := float64(msgFloat32(ev, sel("pressure")))
 s.send(platform.SurfaceEvent{Kind: platform.PointerMove, X: x, Y: y, Mods: eventMods(ev), Pointer: platform.PointerPen, Pressure: pressure})
}`

func patchTabletSource(source string) (string, error) {
	// Upstream is gofmt'd. Keep a readable template and normalize its indentation.
	old := strings.ReplaceAll(oldTabletPoint, "\n ", "\n\t")
	replacement := strings.ReplaceAll(newTabletPoint, "\n ", "\n\t")
	if strings.Count(source, old) != 1 {
		return "", fmt.Errorf("MyGo tabletPoint source changed; review tablet patch before building")
	}
	return strings.Replace(source, old, replacement, 1), nil
}

func run() error {
	const target = "vendor/github.com/egoist/mygo/internal/darwin/surface.go"
	modules, err := os.ReadFile("vendor/modules.txt")
	if err != nil {
		return err
	}
	if !strings.Contains(string(modules), "# github.com/egoist/mygo v0.8.3\n") {
		return fmt.Errorf("tablet patch requires vendored MyGo v0.8.3; review it after dependency updates")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	replacement := strings.ReplaceAll(newTabletPoint, "\n ", "\n\t")
	if strings.Count(string(data), replacement) == 1 {
		return patchIdleSources()
	}
	patched, err := patchTabletSource(string(data))
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, []byte(patched), 0644); err != nil {
		return err
	}
	return patchIdleSources()
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
