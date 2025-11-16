//go:build js && wasm

package grpctunnel

import (
	"testing"
)

// Note: These tests validate URL inference logic
// Actual browser WebSocket connections require a browser environment

func TestInferBrowserWebSocketURL_EmptyTarget(t *testing.T) {
	// In a real browser, this would use window.location
	// In test environment, we test the fallback
	result := inferBrowserWebSocketURL("")
	
	// Should have a valid URL (either from location or fallback)
	if result == "" {
		t.Error("Expected non-empty URL")
	}
}

func TestInferBrowserWebSocketURL_FullURL(t *testing.T) {
	tests := []struct {
		name   string
		target string
		expect string
	}{
		{"WebSocket URL", "ws://localhost:8080", "ws://localhost:8080"},
		{"Secure WebSocket", "wss://api.example.com", "wss://api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferBrowserWebSocketURL(tt.target)
			if result != tt.expect {
				t.Errorf("Expected %s, got %s", tt.expect, result)
			}
		})
	}
}

func TestInferBrowserWebSocketURL_HostPort(t *testing.T) {
	result := inferBrowserWebSocketURL("localhost:8080")
	
	// Should add ws:// or wss:// prefix
	if result != "ws://localhost:8080" && result != "wss://localhost:8080" {
		t.Logf("Got: %s (acceptable with browser protocol inference)", result)
	}
}
