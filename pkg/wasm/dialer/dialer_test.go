//go:build js && wasm

package dialer

import (
	"context"
	"syscall/js"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestNew_ReturnType tests that New returns a valid grpc.DialOption
func TestNew_ReturnType(t *testing.T) {
	opt := New("ws://localhost:8080")
	if opt == nil {
		t.Fatal("New returned nil")
	}
}

// TestNew_URLFormats tests various WebSocket URL formats
func TestNew_URLFormats(t *testing.T) {
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
			opt := New(tt.url)
			if opt == nil {
				t.Errorf("New(%s) returned nil", tt.url)
			}
		})
	}
}

// TestNewWebSocketDialer_NoWebSocketGlobal tests behavior when WebSocket is not available
func TestNewWebSocketDialer_NoWebSocketGlobal(t *testing.T) {
	// In a real WASM environment, WebSocket should always be available
	// This tests the error path if it's not
	
	// We can't easily remove the global WebSocket, so this is a documentation test
	t.Skip("Cannot test missing WebSocket in real WASM environment")
}

// TestWebSocketAddr_Interface tests the webSocketAddr implementation
func TestWebSocketAddr_Interface(t *testing.T) {
	addr := &webSocketAddr{
		network: "websocket",
		address: "test-address",
	}

	if addr.Network() != "websocket" {
		t.Errorf("Network() = %s, want websocket", addr.Network())
	}

	if addr.String() != "test-address" {
		t.Errorf("String() = %s, want test-address", addr.String())
	}
}

// TestWebSocketAddr_EmptyValues tests webSocketAddr with empty values
func TestWebSocketAddr_EmptyValues(t *testing.T) {
	addr := &webSocketAddr{}

	if addr.Network() != "" {
		t.Errorf("Network() = %s, want empty string", addr.Network())
	}

	if addr.String() != "" {
		t.Errorf("String() = %s, want empty string", addr.String())
	}
}

// TestNewWebSocketDialer_ContextCancellation tests context cancellation
func TestNewWebSocketDialer_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	dialer := newWebSocketDialer("ws://localhost:8080")
	
	_, err := dialer(ctx, "test:1234")

	if err == nil {
		t.Error("expected error with cancelled context")
	}

	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

// TestNewWebSocketDialer_Timeout tests dial timeout
func TestNewWebSocketDialer_Timeout(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dialer := newWebSocketDialer("ws://localhost:9999") // Non-existent server
	
	_, err := dialer(ctx, "test:1234")

	// Should timeout or fail to connect
	if err == nil {
		t.Error("expected timeout or connection error")
	}
}

// TestNew_Integration tests the option can be used with gRPC
func TestNew_Integration(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opt := New("ws://localhost:9999")

	// Try to dial - should fail since no server is running
	_, err := grpc.DialContext(ctx, "ignored:1234", opt, grpc.WithInsecure())

	if err == nil {
		t.Error("expected connection error with no server running")
	}
}

// TestWebSocketConnection_Channels tests channel initialization
func TestWebSocketConnection_Channels(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	// Create a mock WebSocket value
	mockWS := js.Global().Get("Object").New()
	mockWS.Set("readyState", 1) // OPEN

	conn := NewWebSocketConn(mockWS).(*webSocketConnection)

	if conn.readMessageChannel == nil {
		t.Error("readMessageChannel not initialized")
	}

	if conn.readErrorChannel == nil {
		t.Error("readErrorChannel not initialized")
	}

	if conn.writeMessageChannel == nil {
		t.Error("writeMessageChannel not initialized")
	}
}

// TestWebSocketConnection_LocalAddr tests LocalAddr method
func TestWebSocketConnection_LocalAddr(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	mockWS := js.Global().Get("Object").New()
	conn := NewWebSocketConn(mockWS)

	addr := conn.LocalAddr()
	if addr == nil {
		t.Error("LocalAddr() returned nil")
	}

	if addr.Network() != "websocket" {
		t.Errorf("LocalAddr().Network() = %s, want websocket", addr.Network())
	}
}

// TestWebSocketConnection_RemoteAddr tests RemoteAddr method
func TestWebSocketConnection_RemoteAddr(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	mockWS := js.Global().Get("Object").New()
	conn := NewWebSocketConn(mockWS)

	addr := conn.RemoteAddr()
	if addr == nil {
		t.Error("RemoteAddr() returned nil")
	}

	if addr.Network() != "websocket" {
		t.Errorf("RemoteAddr().Network() = %s, want websocket", addr.Network())
	}
}

// TestWebSocketConnection_Deadlines tests deadline methods
func TestWebSocketConnection_Deadlines(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	mockWS := js.Global().Get("Object").New()
	conn := NewWebSocketConn(mockWS)

	now := time.Now()
	future := now.Add(time.Second)

	// All deadline methods should return nil (no error)
	if err := conn.SetDeadline(future); err != nil {
		t.Errorf("SetDeadline() error = %v, want nil", err)
	}

	if err := conn.SetReadDeadline(future); err != nil {
		t.Errorf("SetReadDeadline() error = %v, want nil", err)
	}

	if err := conn.SetWriteDeadline(future); err != nil {
		t.Errorf("SetWriteDeadline() error = %v, want nil", err)
	}
}

// TestWebSocketConnection_Close tests Close method
func TestWebSocketConnection_Close(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	closeCalled := false
	mockWS := js.Global().Get("Object").New()
	mockWS.Set("close", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		closeCalled = true
		return nil
	}))

	conn := NewWebSocketConn(mockWS)
	err := conn.Close()

	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	if !closeCalled {
		t.Error("WebSocket.close() was not called")
	}
}
