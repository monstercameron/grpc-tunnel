#!/bin/bash
set -e

SERVER_DIR="./server"
CLIENT_DIR="./client"
BIN_DIR="./bin"
PUBLIC_DIR="./public"

mkdir -p "$BIN_DIR"
mkdir -p "$PUBLIC_DIR"

echo "Building server for ARM macOS (darwin/arm64)..."
GOOS=darwin GOARCH=arm64 go build -o "$BIN_DIR/server" "$SERVER_DIR/main.go"
echo "Server built: $BIN_DIR/server"

echo "Building client for WebAssembly (WASM)..."
GOOS=js GOARCH=wasm go build -o "$PUBLIC_DIR/client.wasm" "$CLIENT_DIR/main.go"
echo "Client built: $PUBLIC_DIR/client.wasm"

echo "Copying wasm_exec.js to public..."
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$PUBLIC_DIR/"
echo "Build completed."
