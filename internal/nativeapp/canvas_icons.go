package nativeapp

import (
	"embed"
	"github.com/egoist/mygo/ui"
)

// Phosphor Icons Regular, MIT. Original pinned assets and attribution are
// embedded; redo mirrors the library undo arrow. See assets/phosphor/SOURCE.json.
//
//go:embed assets/phosphor/*
var canvasIconAssets embed.FS
var canvasIcons = loadCanvasIcons()

func loadCanvasIcons() map[string]*ui.SVG {
	names := map[string]string{
		"画笔":   "pen-nib",
		"荧光笔":  "highlighter",
		"橡皮擦":  "eraser",
		"套索":   "lasso",
		"撤销":   "arrow-u-up-left",
		"重做":   "arrow-u-up-right",
		"移动画布": "hand-palm",
		"铅笔":   "pencil-simple",
		"设置":   "sliders-horizontal",
	}
	icons := map[string]*ui.SVG{}
	for label, name := range names {
		data, err := canvasIconAssets.ReadFile("assets/phosphor/" + name + ".svg")
		if err != nil {
			panic(err)
		}
		icons[label] = ui.MustParseSVG(data)
	}
	return icons
}
