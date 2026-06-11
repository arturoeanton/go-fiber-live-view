#!/bin/sh
# Builds the wasm client and copies the Go wasm runtime into liveview/assets.
set -e
cd "$(dirname "$0")/wasm"

WASM_EXEC="$(go env GOROOT)/lib/wasm/wasm_exec.js"
if [ ! -f "$WASM_EXEC" ]; then
    # Go < 1.24 keeps it under misc/wasm
    WASM_EXEC="$(go env GOROOT)/misc/wasm/wasm_exec.js"
fi

cp "$WASM_EXEC" ../liveview/assets/
GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o ../liveview/assets/json.wasm
echo "wasm client built -> liveview/assets/json.wasm"
