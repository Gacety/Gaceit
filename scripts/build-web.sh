#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

go run ./scripts/mygo-tablet-patch

OUT="dist-web"
rm -rf "$OUT"
mkdir -p "$OUT"

GOROOT="$(go env GOROOT)"
WASM_EXEC=""
for candidate in "$GOROOT/lib/wasm/wasm_exec.js" "$GOROOT/misc/wasm/wasm_exec.js"; do
  if [ -f "$candidate" ]; then
    WASM_EXEC="$candidate"
    break
  fi
done

if [ -z "$WASM_EXEC" ]; then
  echo "error: wasm_exec.js not found under GOROOT=$GOROOT" >&2
  exit 1
fi

cp "$WASM_EXEC" "$OUT/wasm_exec.js"
cp web/index.html "$OUT/index.html"
: > "$OUT/.nojekyll"

GOOS=js GOARCH=wasm go build -trimpath -ldflags='-s -w' -o "$OUT/study408.wasm" .

ls -lh "$OUT"
printf '\nBuilt web bundle in %s\n' "$OUT"
