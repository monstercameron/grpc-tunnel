package bridge

import (
	"testing"
)

// Package-level tests are in separate files:
// - conn_test.go: Tests for webSocketConn
// - server_test.go: Tests for ServeHandler and ServerConfig  
// - client_test.go: Tests for DialOption (non-WASM builds)

// TestPackage is a placeholder to ensure the test package compiles
func TestPackage(t *testing.T) {
	// This package has comprehensive tests in dedicated test files
	t.Log("Bridge package tests are split across multiple files for better organization")
}
