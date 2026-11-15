package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestCanvasInteractionKeepsPaperAndChromeColors(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			a := testApp(t)
			a.M.Theme = theme
			a.SetTool("scratch")
			tt := ui.NewTester(a.View, 1440, 900)
			before := tt.Image()
			// Sample solid paper, question-card and toolbar interiors away from input.
			points := [][2]int{{300, 400}, {1100, 400}, {400, 180}, {1250, 855}}
			expected := make([][4]uint32, len(points))
			for i, p := range points {
				r, g, b, v := before.At(p[0], p[1]).RGBA()
				expected[i] = [4]uint32{r, g, b, v}
			}
			tt.UsePointer(ui.PointerPen, .7)
			tt.Press(70, 240)
			for step := 0; step < 12; step++ {
				tt.Move(80+float32(step)*6, 250+float32(step)*4)
				img := tt.Image()
				for i, p := range points {
					r, g, b, v := img.At(p[0], p[1]).RGBA()
					if got := [4]uint32{r, g, b, v}; got != expected[i] {
						t.Fatalf("step %d pixel %v changed from %v to %v", step, p, expected[i], got)
					}
				}
			}
			tt.Release(180, 300)
		})
	}
}
