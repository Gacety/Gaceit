package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
)

var icons = map[string]*ui.SVG{}

func init() {
	paths := map[string]string{
		"book":    `<path d="M4 4h6l2 2 2-2h6v15h-6l-2 2-2-2H4zM12 6v15"/>`,
		"note":    `<path d="M5 3h11l3 3v15H5zM9 10h6M9 14h6M9 18h4"/>`,
		"clock":   `<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>`,
		"shuffle": `<path d="M3 6h3c6 0 6 12 12 12h3m-4-4 4 4-4 4M3 18h3c2 0 3-1 4-3m4-6c1-2 2-3 4-3h3m-4-4 4 4-4 4"/>`,
		"pen":     `<path d="m4 16 12-12 4 4-12 12-5 1zM14 6l4 4"/>`,
	}
	for k, path := range paths {
		icons[k] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
}
func (a *App) writing(c *ui.Context) {
	t := c.Theme()
	q := a.M.Current()
	r := a.M.Records[q.ID]
	w, _ := c.Size()
	width := float32(380)
	if w < 1200 {
		width = 330
	}
	ui.Column(c.Key("writing-" + q.ID)).Width(width).Shrink(0).Padding(24).Gap(16).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			label := "本题笔记"
			if a.tool == "scratch" {
				label = "草稿纸"
			}
			ui.Text(c, label).FontSize(19).Grow(1)
			if quiet(c, "关闭").Label("关闭写作面板").Clicked() {
				a.tool = ""
			}
		})
		ui.Text(c, q.Title).FontSize(11).TextColor(t.TextMuted)
		if a.tool == "notes" {
			if a.noteMode(c, q.ID) {
				ui.Scroll(c.Key("reader-note-preview")).Grow(1).MinHeight(0).Children(func() { a.Rich.View(c, r.Notes, 16).Label("笔记Markdown预览") })
			} else {
				a.noteEditor(c, "note-"+q.ID, q.ID, "本题笔记内容").Grow(1).MinHeight(0).Placeholder("记下思路与疑问…").FontSize(16).Padding(14).Background(ui.Transparent).Border(0, ui.Transparent)
			}
			ui.Textf(c, "%d 字 · 随题自动保存", len([]rune(r.Notes))).FontSize(10).TextColor(t.TextMuted)
			return
		}
		ui.Row(c).Gap(4).Wrap().Children(func() {
			for _, v := range []struct{ key, label string }{{"pen", "画笔"}, {"eraser", "橡皮"}} {
				b := quiet(c, v.label)
				if a.penTool == v.key {
					b.Background(t.Accent.Alpha(.12))
				}
				if b.Clicked() {
					a.penTool = v.key
				}
			}
			if quiet(c, "撤销").Disabled(len(a.undo) == 0).Clicked() {
				r.Strokes = a.undo[len(a.undo)-1]
				a.undo = a.undo[:len(a.undo)-1]
				a.save()
			}
			if quiet(c, "清空").Disabled(len(r.Strokes) == 0).Clicked() {
				a.undo = append(a.undo, slices.Clone(r.Strokes))
				r.Strokes = []Stroke{}
				a.save()
			}
		})
		canvas := ui.Box(c.Key("scratch-canvas")).Grow(1).MinHeight(0).FillWidth().Clip().Background(t.Background).Label("草稿画布").HandleInput(func(e ui.InputEvent) bool { return a.scratchInput(e, r) })
		bounds := canvas.Bounds()
		a.canvasW = bounds.W
		a.canvasH = bounds.H
		canvas.Draw(func(p *ui.Painter, rect ui.Rect) { a.drawScratch(p, rect, r) })
		ui.Text(c, "支持鼠标与触控笔 · 自动保存").FontSize(10).TextColor(t.TextMuted)
	})
}
func (a *App) scratchInput(e ui.InputEvent, r *Record) bool {
	point := [2]float32{max(0, min(1, e.X/max(1, a.canvasW))), max(0, min(1, e.Y/max(1, a.canvasH)))}
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 {
			return false
		}
		a.live = &Stroke{Tool: a.penTool, Color: "#718a62", Points: [][2]float32{point}}
		return true
	case ui.InputPointerMove:
		if a.live != nil {
			a.live.Points = append(a.live.Points, point)
			return true
		}
	case ui.InputPointerUp:
		if a.live != nil {
			if !e.Canceled {
				a.undo = append(a.undo, slices.Clone(r.Strokes))
				a.live.Points = append(a.live.Points, point)
				r.Strokes = append(r.Strokes, *a.live)
				a.save()
			}
			a.live = nil
			return true
		}
	}
	return false
}
func (a *App) drawScratch(p *ui.Painter, r ui.Rect, record *Record) {
	t := p.Theme()
	for x := r.X; x < r.X+r.W; x += 24 {
		p.Line(x, r.Y, x, r.Y+r.H, 1, t.Border.Alpha(.35))
	}
	for y := r.Y; y < r.Y+r.H; y += 24 {
		p.Line(r.X, y, r.X+r.W, y, 1, t.Border.Alpha(.35))
	}
	draw := func(s Stroke) {
		color := t.Accent
		if strings.HasPrefix(s.Color, "#") {
			color = ui.Hex(s.Color)
		}
		width := float32(2.6)
		if s.Tool == "eraser" {
			color = t.Background
			width = 24
		}
		if len(s.Points) == 1 {
			x, y := r.X+s.Points[0][0]*r.W, r.Y+s.Points[0][1]*r.H
			p.Fill(ui.Rect{X: x - width/2, Y: y - width/2, W: width, H: width}, color, width/2)
		}
		for i := 1; i < len(s.Points); i++ {
			from, to := s.Points[i-1], s.Points[i]
			x1, y1, x2, y2 := r.X+from[0]*r.W, r.Y+from[1]*r.H, r.X+to[0]*r.W, r.Y+to[1]*r.H
			p.Line(x1, y1, x2, y2, width, color)
			p.Fill(ui.Rect{X: x2 - width/2, Y: y2 - width/2, W: width, H: width}, color, width/2)
		}
	}
	for _, s := range record.Strokes {
		draw(s)
	}
	if a.live != nil {
		draw(*a.live)
	}
}
