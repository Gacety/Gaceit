"""Download only the public asset paths listed by the verified XLSX import."""
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import urllib.request
import urllib.error
import time

ROOT = Path(__file__).resolve().parents[1]
manifest_path = ROOT / 'data/relax-1000-27/manifest.json'
manifest = json.loads(manifest_path.read_text())
out = ROOT / 'public/assets/relax-1000-27'
out.mkdir(parents=True, exist_ok=True)

def fetch(path):
    assert path.startswith('/practice/collection-assets/relax-1000-27/')
    target = out / Path(path).name
    url = 'https://www.codebrick.tech' + path
    for attempt in range(5):
        try:
            if not target.exists():
                request = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
                with urllib.request.urlopen(request, timeout=25) as response:
                    mime = response.headers.get('Content-Type', '')
                    data = response.read()
                assert mime.startswith('image/') and data.startswith((b'\xff\xd8',b'\x89PNG',b'RIFF')), (url, mime)
                target.write_bytes(data)
                time.sleep(1.2)
            data = target.read_bytes()
            return {'source': url, 'path': '/assets/relax-1000-27/' + target.name, 'bytes':len(data), 'sha256':hashlib.sha256(data).hexdigest()}
        except urllib.error.HTTPError as error:
            if error.code == 429 and attempt < 4:
                delay = max(60, int(error.headers.get('Retry-After', '60')))
                print(f'Rate limited; respecting cooldown of {delay}s', flush=True)
                time.sleep(delay)
                continue
            return {'source':url, 'error':str(error)}
        except Exception as error:
            if attempt == 4:
                return {'source':url, 'error':str(error)}
            time.sleep(3)

with ThreadPoolExecutor(max_workers=1) as pool:
    results = list(pool.map(fetch, manifest['assets']))
report = {'total':len(results), 'downloaded':sum('error' not in r for r in results), 'failed':[r for r in results if 'error' in r], 'files':results}
(ROOT/'data/relax-1000-27/assets-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'total':report['total'],'downloaded':report['downloaded'],'failedCount':len(report['failed'])},ensure_ascii=False))
if report['failed']:
    raise SystemExit(1)
