#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ "$(uname -s)" != Darwin ]; then
 echo 'Canvas Metal QA requires macOS.'
 exit 0
fi
go run ./scripts/mygo-tablet-patch
CANVAS_QA_DIR=$(mktemp -d "${TMPDIR:-/tmp}/study408-canvas-qa.XXXXXX")
trap 'rm -rf "$CANVAS_QA_DIR"' EXIT
python3 - "$PWD" "$CANVAS_QA_DIR" <<'PY'
import json,pathlib,sys
root,tmp=map(pathlib.Path,sys.argv[1:]); mapping={}
def overlay(rel,text):
 dst=tmp/(str(len(mapping))+'.go');dst.write_text(text);mapping[str(root/rel)]=str(dst)
p='vendor/github.com/egoist/mygo/ui/headless.go';src=(root/p).read_text();needle='h.img.Render(s)';assert src.count(needle)==1
overlay(p,src.replace(needle,'if !study408QAPresent(h,s) {h.img.Render(s)}'))
p='internal/inkcanvas/pan_cache.go';src=(root/p).read_text();needle='func (s *panSurface) paint(p *ui.Painter, a *Editor, regions []ui.Rect) {';assert src.count(needle)==1
overlay(p,src.replace(needle,needle+'\n if study408DirectPan {s.study408PaintDirect(p,a,regions);return}'))
for src,dst in [
 ('metal_canvas_qa.go','vendor/github.com/egoist/mygo/internal/gpu/metal/study408_qa.go'),
 ('ui_canvas_qa.go','vendor/github.com/egoist/mygo/ui/study408_qa.go'),
 ('controller_canvas_qa.go','internal/inkcanvas/study408_qa.go'),
 ('canvas_metal_qa_test.go','internal/inkcanvas/study408_qa_test.go')]:
 assert not (root/dst).exists(),dst
 overlay(dst,(root/'scripts/mygo-tablet-patch/testdata'/src).read_text())
(tmp/'overlay.json').write_text(json.dumps({'Replace':mapping}))
PY
go test -overlay "$CANVAS_QA_DIR/overlay.json" -tags study408_gpuqa ./internal/inkcanvas -run '^TestCanvasMetal' -bench 'BenchmarkCanvasMetalPan' -benchtime=20x -count=1
