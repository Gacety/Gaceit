package nativeapp

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"golang.org/x/net/html"
)

type formula struct {
	File                 string `json:"file"`
	Width, Height, Depth float32
}
type Rich struct {
	NoteImage   func(string) ([]byte, error)
	Assets      fs.FS
	Manifest    map[string]formula
	Cache       richCache[*ui.SVG]
	Prepared    richCache[string]
	Images      richCache[ui.ImageSource]
	Scale       float32
	Options     markdown.Options
	StemOptions markdown.Options
}

func NewRich(assets fs.FS) *Rich {
	r := &Rich{Assets: assets, Cache: richCache[*ui.SVG]{maxEntries: 256, maxBytes: 8 << 20}, Prepared: richCache[string]{maxEntries: 256, maxBytes: 2 << 20}, Images: richCache[ui.ImageSource]{maxEntries: 32, maxBytes: 96 << 20}}
	data, _ := fs.ReadFile(assets, "native-assets/math/manifest.json")
	_ = json.Unmarshal(data, &r.Manifest)
	r.Options = markdown.Options{LoadImage: r.image, InlineMath: r.inline, Math: r.display, Block: r.imageBlock, WrapCode: true}
	r.StemOptions = markdown.Options{LoadImage: r.image, InlineMath: r.inline, Math: r.display, Block: r.imageBlock, WrapCode: true, ShareSelection: true}
	return r
}

// image loads one stable source per file. Scaling happens when the element
// is laid out, not by changing the Markdown image URL: a new URL on every
// pinch step made the Markdown cache start a fresh asynchronous load and
// showed its placeholder for a frame, which read as flicker.
func (r *Rich) image(url string) (ui.ImageSource, error) {
	if i := strings.Index(url, "#z="); i >= 0 {
		url = url[:i]
	}
	url = strings.ReplaceAll(url, "/practice/collection-assets/", "/assets/")
	if !strings.HasPrefix(url, "/assets/") && !strings.HasPrefix(url, "/note-images/") {
		return nil, fmt.Errorf("非本地原题图片")
	}
	if cached, ok := r.Images.get(url); ok {
		return cached, nil
	}
	var data []byte
	var err error
	if strings.HasPrefix(url, "/note-images/") {
		if r.NoteImage == nil {
			return nil, fmt.Errorf("笔记图片不可用")
		}
		data, err = r.NoteImage(url)
	} else {
		cleanPath := path.Clean(strings.TrimPrefix(url, "/"))
		if !strings.HasPrefix(cleanPath, "assets/") {
			return nil, fmt.Errorf("非本地原题图片")
		}
		data, err = fs.ReadFile(r.Assets, "prototype/public/"+cleanPath)
		if err != nil {
			data, err = fs.ReadFile(r.Assets, cleanPath)
		}
	}
	if err != nil {
		return nil, err
	}
	src, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil, err
	}
	w, h := ui.ImageSize(src)
	r.Images.set(url, src, len(url)+int(w*h)*4)
	return src, nil
}

// imageBlock replaces Markdown's asynchronous image paragraph with a stable
// native image. Block promotion in prepareNative gives each source image its
// own paragraph, so this hook sees it directly.
func (r *Rich) imageBlock(c *ui.Context, d *parser.Document, i int) bool {
	in := d.Inlines(i)
	if len(in) == 0 {
		return false
	}
	var img *parser.Inline
	imgIndex := -1
	for j := 0; j < len(in); {
		n := &in[j]
		switch n.Kind {
		case parser.Image:
			if img != nil {
				return false
			}
			img = n
			imgIndex = j
		case parser.Text:
			if strings.TrimSpace(n.Text) != "" {
				return false
			}
		case parser.SoftBreak, parser.LineBreak:
		default:
			return false
		}
		j = n.Next
	}
	if img == nil || img.Pending {
		return false
	}
	src, err := r.image(img.Dest)
	if err != nil || src == nil {
		return false
	}
	w, h := ui.ImageSize(src)
	if w <= 0 || h <= 0 {
		return false
	}
	scale := r.Scale
	if scale <= 0 {
		scale = 1
	}
	alt := ""
	for j := imgIndex + 1; j < img.Next && j < len(in); j++ {
		switch in[j].Kind {
		case parser.Text:
			alt += in[j].Text
		case parser.SoftBreak, parser.LineBreak:
			alt += " "
		}
	}
	font := float32(14)
	if r.Options.Theme != nil && r.Options.Theme.FontSize > 0 {
		font = r.Options.Theme.FontSize
	}
	ui.Row(c).Children(func() {
		ui.Image(c, src).Width(w * scale).MaxWidthPercent(100).AspectRatio(w / h).Radius(font * .3).Label(alt)
	})
	return true
}

func mathRound(v float64) int { return int(v + .5) }

func (r *Rich) svg(tex string) (*ui.SVG, formula, bool) {
	tex = strings.TrimSpace(tex)
	f, ok := r.Manifest[tex]
	if !ok {
		return nil, f, false
	}
	if cached, ok := r.Cache.get(tex); ok {
		return cached, f, true
	}
	data, err := fs.ReadFile(r.Assets, "native-assets/math/"+f.File)
	if err != nil {
		return nil, f, false
	}
	svg, err := ui.ParseSVG(data)
	if err != nil {
		return nil, f, false
	}
	r.Cache.set(tex, svg, len(tex)+len(data))
	return svg, f, true
}
func (r *Rich) inline(tex string, size float32) (markdown.InlineObject, bool) {
	svg, f, ok := r.svg(tex)
	if !ok {
		return markdown.InlineObject{}, false
	}
	return markdown.InlineObject{Width: f.Width * size, Height: (f.Height - f.Depth) * size, Depth: f.Depth * size, Draw: func(p *ui.Painter, box ui.Rect) { p.Icon(svg, box, p.Theme().Text) }}, true
}
func (r *Rich) display(c *ui.Context, tex string) {
	svg, f, ok := r.svg(tex)
	if !ok {
		ui.Text(c, tex).Selectable()
		return
	}
	ui.ScrollHorizontal(c).FillWidth().Children(func() { ui.Icon(c, svg).Size(f.Width*19, f.Height*19).Label(tex) })
}

var tableRE = regexp.MustCompile(`(?is)<table\b[^>]*>.*?</table>`)

// A source image inline in a paragraph is not loaded by the Markdown view;
// giving it its own block keeps the original picture visible without editing
// the source text.
var blockImageRE = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

func plainNode(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type == html.ElementNode && n.Data == "br" {
		return " "
	}
	var s strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.WriteString(plainNode(child))
	}
	return s.String()
}
func prepareNative(src string) string {
	src = strings.ReplaceAll(src, "/practice/collection-assets/", "/assets/")
	src = tableRE.ReplaceAllStringFunc(src, func(table string) string {
		root, err := html.Parse(strings.NewReader(table))
		if err != nil {
			return table
		}
		rows := [][]string{}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "tr" {
				cells := []string{}
				for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
					if ch.Type == html.ElementNode && (ch.Data == "td" || ch.Data == "th") {
						cell := strings.TrimSpace(plainNode(ch))
						cell = strings.ReplaceAll(cell, "|", "\\|")
						cell = strings.ReplaceAll(cell, "\n", " ")
						cells = append(cells, cell)
					}
				}
				if len(cells) > 0 {
					rows = append(rows, cells)
				}
				return
			}
			for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
				walk(ch)
			}
		}
		walk(root)
		if len(rows) == 0 {
			return table
		}
		width := 0
		for _, row := range rows {
			width = max(width, len(row))
		}
		var out strings.Builder
		out.WriteString("\n\n")
		for i, row := range rows {
			out.WriteString("| ")
			for j := 0; j < width; j++ {
				if j < len(row) {
					out.WriteString(row[j])
				}
				out.WriteString(" | ")
			}
			out.WriteString("\n")
			if i == 0 {
				out.WriteString("|" + strings.Repeat(" --- |", width) + "\n")
			}
		}
		out.WriteString("\n")
		return out.String()
	})
	return blockImageRE.ReplaceAllString(src, "\n\n$0\n\n")
}
func (r *Rich) View(c *ui.Context, text string, size float32) ui.Element {
	return r.view(c, text, size, false)
}
func (r *Rich) Stem(c *ui.Context, text string, size float32) ui.Element {
	return r.view(c, text, size, true)
}
func (r *Rich) view(c *ui.Context, text string, size float32, stem bool) ui.Element {
	t := c.Theme()
	prepared := text
	key := text
	if stem {
		key = "\x00stem:" + text
	}
	if cached, ok := r.Prepared.get(key); ok {
		prepared = cached
	} else {
		if stem {
			prepared = formatStemItems(text)
		}
		prepared = prepareNative(prepared)
		r.Prepared.set(key, prepared, len(key)+len(prepared))
	}
	opts := &r.Options
	if stem {
		opts = &r.StemOptions
	}
	opts.Theme = &markdown.Theme{Font: t.Font, FontSize: size, LineHeight: func() float32 {
		if t.Font == "Review Noto" {
			return 1.75
		}
		return 1.7
	}(), Text: t.Text, Muted: t.TextMuted, Link: t.Accent, Strong: t.Text, TableBorder: t.Border, TableHeader: t.Surface, CodeBlockBackground: t.Surface}
	el := markdown.View(c, prepared, opts).FillWidth().MinWidth(0)
	if stem {
		el.Selectable().ContextMenu(func(m *ui.Menu) {
			m.EditItems()
			m.Separator()
			if m.Item("复制完整题干").Chosen() {
				c.WriteClipboard(formatStemItems(text))
			}
			if m.Item("复制题干 Markdown").Chosen() {
				c.WriteClipboard(prepared)
			}
		})
	}
	return el
}
