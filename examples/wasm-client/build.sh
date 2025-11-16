#!/bin/bash

set -e

export GO111MODULE=on

# Get the directory where this script lives
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# Project root is two levels up from wasm-client
PROJECT_ROOT="$( cd "$SCRIPT_DIR/../.." && pwd )"

echo "Installing Playwright dependencies..."
go get github.com/playwright-community/playwright-go
go run github.com/playwright-community/playwright-go/cmd/playwright install

echo "Building WASM client example..."
cd "$SCRIPT_DIR"
GOOS=js GOARCH=wasm go build -o ../_shared/public/client.wasm

echo "Copying wasm_exec.js..."
# Find the path to wasm_exec.js in the Go installation
GO_PATH=$(go env GOROOT)
# Go 1.24+ moved wasm_exec.js to lib/wasm, try both locations
if [ -f "$GO_PATH/lib/wasm/wasm_exec.js" ]; then
    cp "$GO_PATH/lib/wasm/wasm_exec.js" "$SCRIPT_DIR/../_shared/public/wasm_exec.js"
else
    cp "$GO_PATH/misc/wasm/wasm_exec.js" "$SCRIPT_DIR/../_shared/public/wasm_exec.js"
fi

echo "Build complete."