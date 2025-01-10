#!/bin/bash
set -e

SERVER_DIR="./server"
CLIENT_DIR="./client"
BIN_DIR="./bin"
PUBLIC_DIR="./public"

DEBUG_MODE=false

# Check for the -debug flag
for arg in "$@"; do
  if [ "$arg" == "-debug" ]; then
    DEBUG_MODE=true
    echo "Debug mode enabled. Keeping metadata in WebAssembly build."
    break
  fi
done

mkdir -p "$BIN_DIR"
mkdir -p "$PUBLIC_DIR"
mkdir -p "./data"

echo "Building server for ARM macOS (darwin/arm64)..."
GOOS=darwin GOARCH=arm64 go build \
    -o "$BIN_DIR/server" \
    "$SERVER_DIR/main.go"

echo "Server built: $BIN_DIR/server"

echo "Building client for WebAssembly (WASM)..."
TEMP_WASM="$PUBLIC_DIR/client_temp.wasm"

if [ "$DEBUG_MODE" = true ]; then
  # Build without stripping metadata
  GOOS=js GOARCH=wasm go build \
      -o "$PUBLIC_DIR/client.wasm" \
      "$CLIENT_DIR/main.go"
  echo "Client built with metadata: $PUBLIC_DIR/client.wasm"
else
  # Build with stripped debug symbols
  GOOS=js GOARCH=wasm go build \
      -ldflags="-s -w" \
      -o "$TEMP_WASM" \
      "$CLIENT_DIR/main.go"

  # Optimize with wasm-opt if available
  if command -v wasm-opt &> /dev/null; then
    echo "Optimizing client.wasm with wasm-opt -Os..."
    wasm-opt -Os "$TEMP_WASM" -o "$PUBLIC_DIR/client.wasm"
    rm "$TEMP_WASM"
  else
    echo "wasm-opt not found, skipping additional optimization."
    mv "$TEMP_WASM" "$PUBLIC_DIR/client.wasm"
  fi

  echo "Client built: $PUBLIC_DIR/client.wasm"
fi

echo "Copying wasm_exec.js to public..."
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$PUBLIC_DIR/"

# Create an empty data/todos.json if not existing
if [ ! -f "./data/todos.json" ]; then
  echo "[]" > "./data/todos.json"
fi

echo "Build complete."
