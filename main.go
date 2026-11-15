// 408 研习 is entirely native MyGo UI. No URL, HTTP server or WebView is created.
package main

import (
	"crypto/sha256"
	"embed"
	"flag"
	"fmt"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"study408/internal/nativeapp"
	"study408/internal/store"
)

//go:embed prototype/src/relax-data.json prototype/public/fonts/NotoSansSC-Bold.otf prototype/public/assets/relax-1000-27 native-assets/math native-assets/fonts
var assets embed.FS

type bankFS struct {
	jsonFile string
	baseDir  string
	fallback fs.FS
}

func (b bankFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if b.jsonFile != "" && (name == "prototype/src/relax-data.json" || name == "questions.json") {
		return os.Open(b.jsonFile)
	}
	if b.baseDir != "" {
		if f, err := os.Open(filepath.Join(b.baseDir, filepath.FromSlash(name))); err == nil {
			return f, nil
		}
		if strings.HasPrefix(name, "prototype/public/assets/") {
			rel := strings.TrimPrefix(name, "prototype/public/")
			if f, err := os.Open(filepath.Join(b.baseDir, filepath.FromSlash(rel))); err == nil {
				return f, nil
			}
		}
	}
	return b.fallback.Open(name)
}

func resolveExternalBank(bankPath string) (fs.FS, string, string, error) {
	abs, err := filepath.Abs(bankPath)
	if err != nil {
		return nil, "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, "", "", err
	}
	var jsonFile, baseDir, label string
	if info.IsDir() {
		baseDir = abs
		label = filepath.Base(abs)
		for _, candidate := range []string{"questions.json", "relax-data.json", filepath.Join("prototype", "src", "relax-data.json")} {
			p := filepath.Join(abs, candidate)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				jsonFile = p
				break
			}
		}
		if jsonFile == "" {
			return nil, "", "", fmt.Errorf("题库目录中未找到 questions.json 或 relax-data.json：%s", abs)
		}
	} else {
		jsonFile = abs
		baseDir = filepath.Dir(abs)
		stem := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
		if stem == "questions" || stem == "relax-data" {
			label = filepath.Base(baseDir)
		} else {
			label = stem
		}
	}
	var clean strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r >= 0x4e00 && r <= 0x9fff {
			clean.WriteRune(r)
		} else {
			clean.WriteByte('-')
		}
	}
	sum := sha256.Sum256([]byte(abs))
	slug := strings.Trim(clean.String(), "-")
	if slug == "" {
		slug = "custom"
	}
	slug = fmt.Sprintf("%s-%x", slug, sum[:4])
	return bankFS{jsonFile: jsonFile, baseDir: baseDir, fallback: assets}, slug, label, nil
}

func main() {
	dataDir := flag.String("data-dir", "", "learning data directory")
	bank := flag.String("bank", "", "external question bank JSON file or directory")
	title := flag.String("title", "", "custom window and bank title")
	render := flag.String("render", "", "render native UI to PNG without opening a window")
	dark := flag.Bool("dark", false, "start with Dracula theme")
	question := flag.String("question", "", "open a source slug")
	tool := flag.String("tool", "", "open scratch or notes")
	listReports := flag.Bool("list-reports", false, "print question error reports archive JSON for AI tools and exit")
	exportReports := flag.String("export-reports", "", "export question error reports JSON to path and exit")
	flag.Parse()
	var appAssets fs.FS = assets
	var bankSlug, bankLabel string
	if strings.TrimSpace(*bank) != "" {
		var err error
		appAssets, bankSlug, bankLabel, err = resolveExternalBank(*bank)
		if err != nil {
			log.Fatal(err)
		}
	}
	font, err := fs.ReadFile(appAssets, "prototype/public/fonts/NotoSansSC-Bold.otf")
	if err != nil {
		log.Fatal(err)
	}
	if err = ui.RegisterFont(font, "Study Noto"); err != nil {
		log.Fatal(err)
	}
	reviewFont, err := fs.ReadFile(appAssets, "native-assets/fonts/NotoSansSC-Variable.ttf")
	if err != nil {
		log.Fatal(err)
	}
	if err = ui.RegisterFont(reviewFont, "Review Noto"); err != nil {
		log.Fatal(err)
	}
	if *dataDir == "" {
		var err error
		*dataDir, err = defaultDataDir(bankSlug)
		if err != nil {
			log.Fatal(err)
		}
	}
	a, err := nativeapp.New(appAssets, &store.Store{Path: filepath.Join(*dataDir, "progress.json")})
	if err != nil {
		log.Fatal(err)
	}
	windowTitle := "408 研习"
	if strings.TrimSpace(*title) != "" {
		windowTitle = strings.TrimSpace(*title)
		a.M.BankTitle = windowTitle
	} else if bankLabel != "" {
		windowTitle = bankLabel
		a.M.BankTitle = bankLabel
	}
	if *listReports || strings.TrimSpace(*exportReports) != "" {
		raw, err := a.M.ExportQuestionReportsJSON()
		if err != nil {
			log.Fatal(err)
		}
		if strings.TrimSpace(*exportReports) != "" {
			if err := os.MkdirAll(filepath.Dir(*exportReports), 0700); err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile(*exportReports, raw, 0600); err != nil {
				log.Fatal(err)
			}
		}
		if *listReports {
			fmt.Println(string(raw))
		}
		return
	}
	if *dark {
		a.M.Theme = "dark"
	}
	if *question != "" && a.M.Find(*question) >= 0 {
		a.ShowQuestion(*question)
	}
	if *tool == "scratch" || *tool == "notes" {
		a.SetTool(*tool)
	}
	if *render != "" {
		height := 900
		if a.M.Round.Mode != "wrong" {
			height = 960
		}
		image := ui.Render(a.View, 1440, height, 1)
		file, err := os.Create(*render)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		if err = png.Encode(file, image); err != nil {
			log.Fatal(err)
		}
		return
	}
	mygo.App.WhenReady(func() {
		a.Window = mygo.NewWindow(mygo.WindowOptions{Title: windowTitle, Width: 1440, Height: 900, UseContentSize: true, MinWidth: 1060, MinHeight: 720, Content: ui.View(a.View)})
		a.UpdateDisplayRatio()
		a.Window.OnMove(a.UpdateDisplayRatio)
		a.Window.OnResize(a.UpdateDisplayRatio)
		a.Window.OnFocus(a.RestoreWritingSurface)
		a.Window.OnClose(func(e *mygo.CloseEvent) {
			if err := a.FlushWriting(); err != nil {
				e.PreventDefault()
				log.Print(err)
				return
			}
			if err := a.M.Save(); err != nil {
				e.PreventDefault()
				log.Print(err)
			}
		})
	})
	if err = mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
