#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ "$(uname -s)" != Darwin ]; then
  echo 'Metal idle retention check requires macOS.'
  exit 0
fi
go run ./scripts/mygo-tablet-patch
IDLE_TEST_TARGET='vendor/github.com/egoist/mygo/internal/gpu/metal/study408_idle_test.go'
if [ -e "$IDLE_TEST_TARGET" ]; then
  echo 'Refusing to overwrite an existing Metal test file.' >&2
  exit 1
fi
trap 'rm -f "$IDLE_TEST_TARGET"' EXIT
cp scripts/mygo-tablet-patch/testdata/study408_idle_test.go "$IDLE_TEST_TARGET"
go test github.com/egoist/mygo/internal/gpu/metal -run '^TestStudy408' -count=1
