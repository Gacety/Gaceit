package main

import (
	"embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Icons are unmodified Phosphor Regular SVGs, vendored at a fixed commit.
// See assets/phosphor/SOURCE.json and LICENSE for source hashes and MIT terms.
//
//go:embed assets/phosphor/*
var iconAssets embed.FS
var iconOnce sync.Once
var iconSVGs map[string]*ui.SVG
var toolIconNames = map[int]string{
	0: "pen-nib", 1: "highlighter", 2: "pencil-simple", 3: "eraser", 4: "selection-slash",
	5: "sliders-horizontal", 6: "arrow-u-up-left", 7: "hand-palm", 8: "sidebar-simple",
	10: "minus", 11: "plus", 12: "crosshair-simple", 13: "pencil-circle", 14: "lasso",
}

func libraryIcon(name string) *ui.SVG {
	iconOnce.Do(func() {
		iconSVGs = make(map[string]*ui.SVG)
		names := []string{"caret-down", "dots-six-vertical"}
		for _, name := range toolIconNames {
			names = append(names, name)
		}
		for _, name := range names {
			data, err := iconAssets.ReadFile("assets/phosphor/" + name + ".svg")
			if err != nil {
				panic(err)
			}
			iconSVGs[name] = ui.MustParseSVG(data)
		}
	})
	return iconSVGs[name]
}
func drawToolIcon(p *ui.Painter, r ui.Rect, kind int, col ui.Color) {
	name, ok := toolIconNames[kind]
	if !ok {
		return
	}
	size := float32(22)
	x, y := r.X+(min(r.W, 34)-size)/2, r.Y+(r.H-size)/2
	p.Icon(libraryIcon(name), ui.Rect{X: x, Y: y, W: size, H: size}, col)
	if (kind == 0 || kind == 3) && r.W > 38 {
		p.Icon(libraryIcon("caret-down"), ui.Rect{X: r.X + r.W - 13, Y: r.Y + (r.H-10)/2, W: 10, H: 10}, col)
	}
}
func drawGrip(p *ui.Painter, r ui.Rect) {
	p.Icon(libraryIcon("dots-six-vertical"), ui.Rect{X: r.X + (r.W-14)/2, Y: r.Y + (r.H-20)/2, W: 14, H: 20}, ui.RGB(119, 133, 141))
}
