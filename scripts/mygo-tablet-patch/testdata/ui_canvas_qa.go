//go:build darwin && study408_gpuqa

package ui

import (
	"fmt"
	"github.com/egoist/mygo/internal/gpu/metal"
	"github.com/egoist/mygo/internal/scene"
)

var study408Renderers = map[*headless]*metal.Renderer{}
var study408Pixels = map[*headless][]byte{}

func study408QAPresent(h *headless, s *scene.Scene) bool {
	r := study408Renderers[h]
	if r == nil {
		return false
	}
	pix, err := r.Study408QADraw(s)
	if err != nil {
		panic(fmt.Sprintf("Metal QA: %v", err))
	}
	study408Pixels[h] = pix
	return true
}
func (t *Tester) Study408UseMetal() (func(), error) {
	r, err := metal.NewStudy408QARenderer()
	if err != nil {
		return nil, err
	}
	study408Renderers[t.h] = r
	return func() { delete(study408Renderers, t.h); delete(study408Pixels, t.h); r.Release() }, nil
}
func (t *Tester) Study408MetalPixels() []byte { return study408Pixels[t.h] }
func (t *Tester) Study408TrimMetal()          { study408Renderers[t.h].Study408QATrim() }
