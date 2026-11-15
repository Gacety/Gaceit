# Finish build-time math assets: convert SVG text to supplied font outlines.
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
import pathlib,xml.etree.ElementTree as E,json
font=TTFont('prototype/public/fonts/NotoSansSC-Bold.otf');glyphs=font.getGlyphSet();cmap=font.getBestCmap();upm=font['head'].unitsPerEm
ns='{http://www.w3.org/2000/svg}';E.register_namespace('',ns[1:-1]);fixed=0;missing=[]
for p in pathlib.Path('native-assets/math').glob('*.svg'):
 if '<text' not in p.read_text():continue
 root=E.fromstring(p.read_text())
 for parent in root.iter():
  for child in list(parent):
   if child.tag!=ns+'text':continue
   group=E.Element(ns+'g',{'transform':child.get('transform','')});size=float(child.get('font-size','884px').removesuffix('px'));scale=size/upm;x=0
   for char in ''.join(child.itertext()):
    name=cmap.get(ord(char))
    if name is None:missing.append(char);continue
    pen=SVGPathPen(glyphs);glyphs[name].draw(pen)
    E.SubElement(group,ns+'path',{'d':pen.getCommands(),'transform':f'translate({x},0) scale({scale}, {-scale})'})
    x+=glyphs[name].width*scale
   parent.insert(list(parent).index(child),group);parent.remove(child);fixed+=1
 p.write_text(E.tostring(root,encoding='unicode'))
report=json.loads(pathlib.Path('native-assets/math/report.json').read_text());report.update({'textGlyphsConverted':fixed,'missingGlyphs':sorted(set(missing))});pathlib.Path('native-assets/math/report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
print(report)
if missing:raise SystemExit('Missing supplied font glyphs')
