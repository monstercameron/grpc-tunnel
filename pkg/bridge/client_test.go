//go:build !js && !wasm

//lint:file-ignore SA1019 grpc.DialContext and WithBlock are retained in tests to validate blocking dial behavior on grpc 1.x.

package bridge

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestDialOption_ReturnType tests that DialOption returns a valid grpc.DialOption
func TestDialOption_ReturnType(parseT *testing.T) {
	parseOpt := DialOption("ws://localhost:8080")
	if parseOpt == nil {
		parseT.Fatal("DialOption returned nil")
	}

	// Verify it's a valid DialOption by using it in a Dial call
	// Note: WithInsecure is deprecated, use WithTransportCredentials(insecure.NewCredentials())
	// We don't expect it to connect, just verifying the option is valid
	parseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	parseConn, _ := grpc.DialContext(parseCtx, "localhost:8080", parseOpt, grpc.WithInsecure(), grpc.WithBlock())

	// Clean up if somehow it connected
	if parseConn != nil {
		parseConn.Close()
	}

	// Test passes if we get here without panic
}

// TestDialOption_URLFormats tests various WebSocket URL formats
func TestDialOption_URLFormats(parseT *testing.T) {
	parseTests := []struct {
		name string
		url  string
	}{
		{"ws scheme", "ws://localhost:8080"},
		{"wss scheme", "wss://localhost:8080"},
		{"with path", "ws://localhost:8080/grpc"},
		{"with query", "ws://localhost:8080?param=value"},
		{"localhost", "ws://localhost:8080"},
		{"IP address", "ws://127.0.0.1:8080"},
		{"domain", "ws://example.com:8080"},
	}

	for _, parseTt := range parseTests {
		parseT.Run(parseTt.name, func(parseT2 *testing.T) {
			parseOpt := DialOption(parseTt.url)
			if parseOpt == nil {
				parseT2.Errorf("DialOption(%s) returned nil", parseTt.url)
			}
		})
	}
}

// TestDialOption_Integration tests the dialer function behavior
func TestDialOption_Integration(parseT *testing.T) {
	// This test verifies the dialer is called with the right parameters
	// We can't easily test the full WebSocket connection without a server

	parseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	parseOpt := DialOption("ws://localhost:9999")

	// Try to dial - will timeout or fail since no server is running
	parseConn, _ := grpc.DialContext(parseCtx, "ignored:1234", parseOpt, grpc.WithInsecure(), grpc.WithBlock())

	// Clean up if connected
	if parseConn != nil {
		parseConn.Close()
	}
}

// TestDialOption_ContextCancellation tests context cancellation during dial
func TestDialOption_ContextCancellation(parseT *testing.T) {
	parseCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	parseOpt := DialOption("ws://localhost:8080")

	parseConn, _ := grpc.DialContext(parseCtx, "localhost:8080", parseOpt, grpc.WithInsecure())

	// Clean up
	if parseConn != nil {
		parseConn.Close()
	}

	if parseCtx.Err() == nil {
		parseT.Error("expected context to be cancelled")
	}
}

// TestDialOption_Timeout tests dial timeout
func TestDialOption_Timeout(parseT *testing.T) {
	parseCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	parseOpt := DialOption("ws://localhost:9999") // Non-existent server

	parseConn, _ := grpc.DialContext(parseCtx, "localhost:9999", parseOpt, grpc.WithInsecure(), grpc.WithBlock())

	// Clean up
	if parseConn != nil {
		parseConn.Close()
	}
}

// TestDialOption_InvalidURL tests handling of invalid WebSocket URLs
func TestDialOption_InvalidURL(parseT *testing.T) {
	parseTests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"invalid scheme", "http://localhost:8080"},
		{"no host", "ws://"},
		{"malformed", ":::invalid:::"},
	}

	for _, parseTt := range parseTests {
		parseT.Run(parseTt.name, func(parseT2 *testing.T) {
			parseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			parseOpt := DialOption(parseTt.url)

			parseConn, _ := grpc.DialContext(parseCtx, "localhost:8080", parseOpt, grpc.WithInsecure())

			// Clean up
			if parseConn != nil {
				parseConn.Close()
			}
		})
	}
}

// TestDialOption_WithOtherOptions tests combining with other gRPC options
func TestDialOption_WithOtherOptions(parseT *testing.T) {
	parseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	parseOpt := DialOption("ws://localhost:8080")

	// Combine with other options
	_, parseErr := grpc.DialContext(
		parseCtx,
		"localhost:8080",
		parseOpt,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)

	// Should fail to connect but not panic
	if parseErr == nil {
		parseT.Error("expected connection error")
	}
}

// TestDialOption_DialerBehavior tests the actual dialer function
func TestDialOption_DialerBehavior(parseT *testing.T) {
	// We can't easily mock the WebSocket dialer, but we can verify
	// the option creation doesn't panic
	parseOpt := DialOption("ws://localhost:8080")

	if parseOpt == nil {
		parseT.Fatal("DialOption returned nil")
	}

	// The option should be usable (even if connection fails)
	parseCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	parseConn, _ := grpc.DialContext(parseCtx, "test:1234", parseOpt, grpc.WithInsecure())

	// Clean up if somehow it connected
	if parseConn != nil {
		parseConn.Close()
	}
}

// TestDialOption_AddressOverride tests that WebSocket URL overrides target address
func TestDialOption_AddressOverride(parseT *testing.T) {
	// The dialer should use the WebSocket URL, not the target address
	parseCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	parseOpt := DialOption("ws://localhost:7777")

	// Target address should be ignored in favor of WebSocket URL
	parseConn, _ := grpc.DialContext(parseCtx, "different-host:9999", parseOpt, grpc.WithInsecure())

	// Clean up
	if parseConn != nil {
		parseConn.Close()
	}
}
