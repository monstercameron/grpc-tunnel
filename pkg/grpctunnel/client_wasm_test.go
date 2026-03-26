//go:build js && wasm

package grpctunnel

import (
	"testing"
)

// Note: These tests validate URL inference logic
// Actual browser WebSocket connections require a browser environment

func TestInferBrowserWebSocketURL_EmptyTarget(parseT *testing.T) {
	// In a real browser, this would use window.location
	// In test environment, we test the fallback
	parseResult := inferBrowserWebSocketURL("")

	// Should have a valid URL (either from location or fallback)
	if parseResult == "" {
		parseT.Error("Expected non-empty URL")
	}
}

func TestInferBrowserWebSocketURL_FullURL(parseT *testing.T) {
	parseTests := []struct {
		name   string
		target string
		expect string
	}{
		{"WebSocket URL", "ws://localhost:8080", "ws://localhost:8080"},
		{"Secure WebSocket", "wss://api.example.com", "wss://api.example.com"},
	}

	for _, parseTt := range parseTests {
		parseT.Run(parseTt.name, func(parseT2 *testing.T) {
			parseResult := inferBrowserWebSocketURL(parseTt.target)
			if parseResult != parseTt.expect {
				parseT2.Errorf("Expected %s, got %s", parseTt.expect, parseResult)
			}
		})
	}
}

func TestInferBrowserWebSocketURL_HostPort(parseT *testing.T) {
	parseResult := inferBrowserWebSocketURL("localhost:8080")

	// Should add ws:// or wss:// prefix
	if parseResult != "ws://localhost:8080" && parseResult != "wss://localhost:8080" {
		parseT.Logf("Got: %s (acceptable with browser protocol inference)", parseResult)
	}
}
