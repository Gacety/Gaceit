#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
go run ./scripts/mygo-tablet-patch
exec go run . "$@"
