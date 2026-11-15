//go:build darwin && study408_gpuqa

package metal

import "github.com/egoist/mygo/internal/scene"

// Available only in the disposable QA overlay, never in the app binary.
func NewStudy408QARenderer() (*Renderer, error)                   { return newRenderer() }
func (r *Renderer) Study408QADraw(s *scene.Scene) ([]byte, error) { return r.renderOffscreen(s) }
func (r *Renderer) Study408QATrim()                               { r.releaseTextures(true) }
