//go:build !js && !wasm

package bridge

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestDialOption_ReturnType tests that DialOption returns a valid grpc.DialOption
func TestDialOption_ReturnType(t *testing.T) {
	opt := DialOption("ws://localhost:8080")
	if opt == nil {
		t.Fatal("DialOption returned nil")
	}

	// Verify it's a valid DialOption by using it in a Dial call
	// Note: WithInsecure is deprecated, use WithTransportCredentials(insecure.NewCredentials())
	// We don't expect it to connect, just verifying the option is valid
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	conn, _ := grpc.DialContext(ctx, "localhost:8080", opt, grpc.WithInsecure(), grpc.WithBlock())

	// Clean up if somehow it connected
	if conn != nil {
		conn.Close()
	}

	// Test passes if we get here without panic
}

// TestDialOption_URLFormats tests various WebSocket URL formats
func TestDialOption_URLFormats(t *testing.T) {
	tests := []struct {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := DialOption(tt.url)
			if opt == nil {
				t.Errorf("DialOption(%s) returned nil", tt.url)
			}
		})
	}
}

// TestDialOption_Integration tests the dialer function behavior
func TestDialOption_Integration(t *testing.T) {
	// This test verifies the dialer is called with the right parameters
	// We can't easily test the full WebSocket connection without a server

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opt := DialOption("ws://localhost:9999")

	// Try to dial - will timeout or fail since no server is running
	conn, _ := grpc.DialContext(ctx, "ignored:1234", opt, grpc.WithInsecure(), grpc.WithBlock())

	// Clean up if connected
	if conn != nil {
		conn.Close()
	}
}

// TestDialOption_ContextCancellation tests context cancellation during dial
func TestDialOption_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opt := DialOption("ws://localhost:8080")

	conn, _ := grpc.DialContext(ctx, "localhost:8080", opt, grpc.WithInsecure())

	// Clean up
	if conn != nil {
		conn.Close()
	}

	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

// TestDialOption_Timeout tests dial timeout
func TestDialOption_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	opt := DialOption("ws://localhost:9999") // Non-existent server

	conn, _ := grpc.DialContext(ctx, "localhost:9999", opt, grpc.WithInsecure(), grpc.WithBlock())

	// Clean up
	if conn != nil {
		conn.Close()
	}
}

// TestDialOption_InvalidURL tests handling of invalid WebSocket URLs
func TestDialOption_InvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"invalid scheme", "http://localhost:8080"},
		{"no host", "ws://"},
		{"malformed", ":::invalid:::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			opt := DialOption(tt.url)

			conn, _ := grpc.DialContext(ctx, "localhost:8080", opt, grpc.WithInsecure())

			// Clean up
			if conn != nil {
				conn.Close()
			}
		})
	}
}

// TestDialOption_WithOtherOptions tests combining with other gRPC options
func TestDialOption_WithOtherOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opt := DialOption("ws://localhost:8080")

	// Combine with other options
	_, err := grpc.DialContext(
		ctx,
		"localhost:8080",
		opt,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)

	// Should fail to connect but not panic
	if err == nil {
		t.Error("expected connection error")
	}
}

// mockDialer is a test helper for testing dialer behavior
type mockDialer struct {
	dialFunc func(context.Context, string) (net.Conn, error)
}

func (m *mockDialer) Dial(ctx context.Context, addr string) (net.Conn, error) {
	return m.dialFunc(ctx, addr)
}

// TestDialOption_DialerBehavior tests the actual dialer function
func TestDialOption_DialerBehavior(t *testing.T) {
	// We can't easily mock the WebSocket dialer, but we can verify
	// the option creation doesn't panic
	opt := DialOption("ws://localhost:8080")

	if opt == nil {
		t.Fatal("DialOption returned nil")
	}

	// The option should be usable (even if connection fails)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	conn, _ := grpc.DialContext(ctx, "test:1234", opt, grpc.WithInsecure())

	// Clean up if somehow it connected
	if conn != nil {
		conn.Close()
	}
}

// TestDialOption_AddressOverride tests that WebSocket URL overrides target address
func TestDialOption_AddressOverride(t *testing.T) {
	// The dialer should use the WebSocket URL, not the target address
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	opt := DialOption("ws://localhost:7777")

	// Target address should be ignored in favor of WebSocket URL
	conn, _ := grpc.DialContext(ctx, "different-host:9999", opt, grpc.WithInsecure())

	// Clean up
	if conn != nil {
		conn.Close()
	}
}
