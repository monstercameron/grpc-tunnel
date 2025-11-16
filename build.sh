#!/bin/bash

set -e

export GO111MODULE=on

echo "Installing Playwright dependencies..."
go get github.com/playwright-community/playwright-go
go run github.com/playwright-community/playwright-go/cmd/playwright install

echo "Building WASM client example..."
(cd examples/wasm-client && GOOS=js GOARCH=wasm go build -o ../../public/client.wasm)

echo "Copying wasm_exec.js..."
# Find the path to wasm_exec.js in the Go installation
GO_PATH=$(go env GOROOT)
# Go 1.24+ moved wasm_exec.js to lib/wasm, try both locations
if [ -f "$GO_PATH/lib/wasm/wasm_exec.js" ]; then
    cp "$GO_PATH/lib/wasm/wasm_exec.js" public/wasm_exec.js
else
    cp "$GO_PATH/misc/wasm/wasm_exec.js" public/wasm_exec.js
fi

echo "Build complete."