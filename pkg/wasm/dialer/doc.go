//go:build js && wasm

// Package dialer provides browser-specific gRPC dial integration for WebAssembly.
//
// The package adapts browser WebSocket APIs to net.Conn so gRPC can run over
// WebSocket transport from Go/WASM clients.
package dialer
