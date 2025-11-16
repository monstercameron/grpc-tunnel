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
	dialOption := New("ws://localhost:8080")
	if dialOption == nil {
		t.Fatal("New returned nil")
	}
}

// TestNew_URLFormats tests various WebSocket URL formats
func TestNew_URLFormats(t *testing.T) {
	tests := []struct {
		testName     string
		websocketURL string
	}{
		{"ws scheme", "ws://localhost:8080"},
		{"wss scheme", "wss://localhost:8080"},
		{"with path", "ws://localhost:8080/grpc"},
		{"with query", "ws://localhost:8080?param=value"},
		{"localhost", "ws://localhost:8080"},
		{"IP address", "ws://127.0.0.1:8080"},
		{"domain", "ws://example.com:8080"},
	}

	for _, testCase := range tests {
		t.Run(testCase.testName, func(t *testing.T) {
			dialOption := New(testCase.websocketURL)
			if dialOption == nil {
				t.Errorf("New(%s) returned nil", testCase.websocketURL)
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

// TestBrowserWebSocketAddr_Interface tests the browserWebSocketAddr implementation
func TestBrowserWebSocketAddr_Interface(t *testing.T) {
	websocketAddress := &browserWebSocketAddr{
		networkType:   "websocket",
		addressString: "test-address",
	}

	if websocketAddress.Network() != "websocket" {
		t.Errorf("Network() = %s, want websocket", websocketAddress.Network())
	}

	if websocketAddress.String() != "test-address" {
		t.Errorf("String() = %s, want test-address", websocketAddress.String())
	}
}

// TestBrowserWebSocketAddr_EmptyValues tests browserWebSocketAddr with empty values
func TestBrowserWebSocketAddr_EmptyValues(t *testing.T) {
	websocketAddress := &browserWebSocketAddr{}

	if websocketAddress.Network() != "" {
		t.Errorf("Network() = %s, want empty string", websocketAddress.Network())
	}

	if websocketAddress.String() != "" {
		t.Errorf("String() = %s, want empty string", websocketAddress.String())
	}
}

// TestNewBrowserWebSocketDialer_ContextCancellation tests context cancellation
func TestNewBrowserWebSocketDialer_ContextCancellation(t *testing.T) {
	dialContext, cancelFunc := context.WithCancel(context.Background())
	cancelFunc() // Cancel immediately

	websocketDialer := newBrowserWebSocketDialer("ws://localhost:8080")

	_, err := websocketDialer(dialContext, "test:1234")

	if err == nil {
		t.Error("expected error with cancelled context")
	}

	if dialContext.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

// TestNewBrowserWebSocketDialer_Timeout tests dial timeout
func TestNewBrowserWebSocketDialer_Timeout(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	dialContext, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelFunc()

	websocketDialer := newBrowserWebSocketDialer("ws://localhost:9999") // Non-existent server

	_, err := websocketDialer(dialContext, "test:1234")

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

	dialContext, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelFunc()

	dialOption := New("ws://localhost:9999")

	// Try to dial - should fail since no server is running
	_, err := grpc.DialContext(dialContext, "ignored:1234", dialOption, grpc.WithInsecure())

	if err == nil {
		t.Error("expected connection error with no server running")
	}
}

// TestBrowserWebSocketConnection_Channels tests channel initialization
func TestBrowserWebSocketConnection_Channels(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	// Create a mock WebSocket value
	mockBrowserWebSocket := js.Global().Get("Object").New()
	mockBrowserWebSocket.Set("readyState", 1) // OPEN

	networkConnection := NewWebSocketConn(mockBrowserWebSocket).(*browserWebSocketConnection)

	if networkConnection.incomingMessagesChannel == nil {
		t.Error("incomingMessagesChannel not initialized")
	}

	if networkConnection.incomingErrorsChannel == nil {
		t.Error("incomingErrorsChannel not initialized")
	}

	if networkConnection.outgoingMessagesChannel == nil {
		t.Error("outgoingMessagesChannel not initialized")
	}
}

// TestBrowserWebSocketConnection_LocalAddr tests LocalAddr method
func TestBrowserWebSocketConnection_LocalAddr(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	mockBrowserWebSocket := js.Global().Get("Object").New()
	networkConnection := NewWebSocketConn(mockBrowserWebSocket)

	localAddress := networkConnection.LocalAddr()
	if localAddress == nil {
		t.Error("LocalAddr() returned nil")
	}

	if localAddress.Network() != "websocket" {
		t.Errorf("LocalAddr().Network() = %s, want websocket", localAddress.Network())
	}
}

// TestBrowserWebSocketConnection_RemoteAddr tests RemoteAddr method
func TestBrowserWebSocketConnection_RemoteAddr(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	mockBrowserWebSocket := js.Global().Get("Object").New()
	networkConnection := NewWebSocketConn(mockBrowserWebSocket)

	remoteAddress := networkConnection.RemoteAddr()
	if remoteAddress == nil {
		t.Error("RemoteAddr() returned nil")
	}

	if remoteAddress.Network() != "websocket" {
		t.Errorf("RemoteAddr().Network() = %s, want websocket", remoteAddress.Network())
	}
}

// TestBrowserWebSocketConnection_Deadlines tests deadline methods
func TestBrowserWebSocketConnection_Deadlines(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	mockBrowserWebSocket := js.Global().Get("Object").New()
	networkConnection := NewWebSocketConn(mockBrowserWebSocket)

	currentTime := time.Now()
	futureDeadline := currentTime.Add(time.Second)

	// All deadline methods should return nil (no error)
	if err := networkConnection.SetDeadline(futureDeadline); err != nil {
		t.Errorf("SetDeadline() error = %v, want nil", err)
	}

	if err := networkConnection.SetReadDeadline(futureDeadline); err != nil {
		t.Errorf("SetReadDeadline() error = %v, want nil", err)
	}

	if err := networkConnection.SetWriteDeadline(futureDeadline); err != nil {
		t.Errorf("SetWriteDeadline() error = %v, want nil", err)
	}
}

// TestBrowserWebSocketConnection_Close tests Close method
func TestBrowserWebSocketConnection_Close(t *testing.T) {
	if !js.Global().Get("WebSocket").Truthy() {
		t.Skip("WebSocket not available in test environment")
	}

	closeMethodCalled := false
	mockBrowserWebSocket := js.Global().Get("Object").New()
	mockBrowserWebSocket.Set("close", js.FuncOf(func(this js.Value, functionArgs []js.Value) interface{} {
		closeMethodCalled = true
		return nil
	}))

	networkConnection := NewWebSocketConn(mockBrowserWebSocket)
	err := networkConnection.Close()

	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	if !closeMethodCalled {
		t.Error("WebSocket.close() was not called")
	}
}
