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
func TestNew_ReturnType(parseT *testing.T) {
	parseDialOption := New("ws://localhost:8080")
	if parseDialOption == nil {
		parseT.Fatal("New returned nil")
	}
}

// TestNew_URLFormats tests various WebSocket URL formats
func TestNew_URLFormats(parseT *testing.T) {
	parseTests := []struct {
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

	for _, parseTestCase := range parseTests {
		parseT.Run(parseTestCase.testName, func(parseT2 *testing.T) {
			parseDialOption := New(parseTestCase.websocketURL)
			if parseDialOption == nil {
				parseT2.Errorf("New(%s) returned nil", parseTestCase.websocketURL)
			}
		})
	}
}

func TestNewWithConfig_ReturnType(parseT *testing.T) {
	parseDialOption := NewWithConfig("ws://localhost:8080", Config{
		Subprotocols: []string{"proto.v1"},
	})
	if parseDialOption == nil {
		parseT.Fatal("NewWithConfig returned nil")
	}
}

// TestNewWebSocketDialer_NoWebSocketGlobal tests behavior when WebSocket is not available
func TestNewWebSocketDialer_NoWebSocketGlobal(parseT *testing.T) {
	// In a real WASM environment, WebSocket should always be available
	// This tests the error path if it's not

	// We can't easily remove the global WebSocket, so this is a documentation test
	parseT.Skip("Cannot test missing WebSocket in real WASM environment")
}

// TestBrowserWebSocketAddr_Interface tests the browserWebSocketAddr implementation
func TestBrowserWebSocketAddr_Interface(parseT *testing.T) {
	parseWebsocketAddress := &browserWebSocketAddr{
		networkType:   networkTypeWebSocket,
		addressString: "test-address",
	}

	if parseWebsocketAddress.Network() != networkTypeWebSocket {
		parseT.Errorf("Network() = %s, want websocket", parseWebsocketAddress.Network())
	}

	if parseWebsocketAddress.String() != "test-address" {
		parseT.Errorf("String() = %s, want test-address", parseWebsocketAddress.String())
	}
}

// TestBrowserWebSocketAddr_EmptyValues tests browserWebSocketAddr with empty values
func TestBrowserWebSocketAddr_EmptyValues(parseT *testing.T) {
	parseWebsocketAddress := &browserWebSocketAddr{}

	if parseWebsocketAddress.Network() != "" {
		parseT.Errorf("Network() = %s, want empty string", parseWebsocketAddress.Network())
	}

	if parseWebsocketAddress.String() != "" {
		parseT.Errorf("String() = %s, want empty string", parseWebsocketAddress.String())
	}
}

// TestNewBrowserWebSocketDialer_ContextCancellation tests context cancellation
func TestNewBrowserWebSocketDialer_ContextCancellation(parseT *testing.T) {
	parseDialContext, cancelFunc := context.WithCancel(context.Background())
	cancelFunc() // Cancel immediately

	parseWebsocketDialer := newBrowserWebSocketDialer("ws://localhost:8080")

	_, parseErr := parseWebsocketDialer(parseDialContext, "test:1234")

	if parseErr == nil {
		parseT.Error("expected error with cancelled context")
	}

	if parseDialContext.Err() == nil {
		parseT.Error("expected context to be cancelled")
	}
}

// TestNewBrowserWebSocketDialer_Timeout tests dial timeout
func TestNewBrowserWebSocketDialer_Timeout(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	parseDialContext, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelFunc()

	parseWebsocketDialer := newBrowserWebSocketDialer("ws://localhost:9999") // Non-existent server

	_, parseErr := parseWebsocketDialer(parseDialContext, "test:1234")

	// Should timeout or fail to connect
	if parseErr == nil {
		parseT.Error("expected timeout or connection error")
	}
}

// TestNew_Integration tests the option can be used with gRPC
func TestNew_Integration(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	parseDialContext, cancelFunc := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelFunc()

	parseDialOption := New("ws://localhost:9999")

	// Try to dial - should fail since no server is running.
	// Use WithBlock so the timeout surfaces as a dial error rather than an async
	// background connection failure.
	parseConnection, parseErr := grpc.DialContext(
		parseDialContext,
		"ignored:1234",
		parseDialOption,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if parseConnection != nil {
		defer parseConnection.Close()
	}

	if parseErr == nil {
		parseT.Error("expected connection error with no server running")
	}
}

// TestBrowserWebSocketConnection_Channels tests channel initialization
func TestBrowserWebSocketConnection_Channels(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	// Create a mock WebSocket value
	parseMockBrowserWebSocket := js.Global().Get(jsGlobalObject).New()
	parseMockBrowserWebSocket.Set(jsPropertyReadyState, 1) // OPEN

	parseNetworkConnection := NewWebSocketConn(parseMockBrowserWebSocket).(*browserWebSocketConnection)
	defer parseNetworkConnection.closeChannels()

	if parseNetworkConnection.incomingMessagesChannel == nil {
		parseT.Error("incomingMessagesChannel not initialized")
	}

	if parseNetworkConnection.incomingErrorsChannel == nil {
		parseT.Error("incomingErrorsChannel not initialized")
	}
}

// TestBrowserWebSocketConnection_LocalAddr tests LocalAddr method
func TestBrowserWebSocketConnection_LocalAddr(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	parseMockBrowserWebSocket := js.Global().Get(jsGlobalObject).New()
	parseNetworkConnection := NewWebSocketConn(parseMockBrowserWebSocket)
	defer parseNetworkConnection.(*browserWebSocketConnection).closeChannels()

	parseLocalAddress := parseNetworkConnection.LocalAddr()
	if parseLocalAddress == nil {
		parseT.Error("LocalAddr() returned nil")
	}

	if parseLocalAddress.Network() != networkTypeWebSocket {
		parseT.Errorf("LocalAddr().Network() = %s, want websocket", parseLocalAddress.Network())
	}
}

// TestBrowserWebSocketConnection_RemoteAddr tests RemoteAddr method
func TestBrowserWebSocketConnection_RemoteAddr(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	parseMockBrowserWebSocket := js.Global().Get(jsGlobalObject).New()
	parseNetworkConnection := NewWebSocketConn(parseMockBrowserWebSocket)
	defer parseNetworkConnection.(*browserWebSocketConnection).closeChannels()

	parseRemoteAddress := parseNetworkConnection.RemoteAddr()
	if parseRemoteAddress == nil {
		parseT.Error("RemoteAddr() returned nil")
	}

	if parseRemoteAddress.Network() != networkTypeWebSocket {
		parseT.Errorf("RemoteAddr().Network() = %s, want websocket", parseRemoteAddress.Network())
	}
}

// TestBrowserWebSocketConnection_Deadlines tests deadline methods
func TestBrowserWebSocketConnection_Deadlines(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	parseMockBrowserWebSocket := js.Global().Get(jsGlobalObject).New()
	parseNetworkConnection := NewWebSocketConn(parseMockBrowserWebSocket)
	defer parseNetworkConnection.(*browserWebSocketConnection).closeChannels()

	parseCurrentTime := time.Now()
	parseFutureDeadline := parseCurrentTime.Add(time.Second)

	// All deadline methods should return nil (no error)
	if parseErr := parseNetworkConnection.SetDeadline(parseFutureDeadline); parseErr != nil {
		parseT.Errorf("SetDeadline() error = %v, want nil", parseErr)
	}

	if parseErr2 := parseNetworkConnection.SetReadDeadline(parseFutureDeadline); parseErr2 != nil {
		parseT.Errorf("SetReadDeadline() error = %v, want nil", parseErr2)
	}

	if parseErr3 := parseNetworkConnection.SetWriteDeadline(parseFutureDeadline); parseErr3 != nil {
		parseT.Errorf("SetWriteDeadline() error = %v, want nil", parseErr3)
	}
}

// TestBrowserWebSocketConnection_Close tests Close method
func TestBrowserWebSocketConnection_Close(parseT *testing.T) {
	if !js.Global().Get(jsGlobalWebSocket).Truthy() {
		parseT.Skip("WebSocket not available in test environment")
	}

	isCloseMethodCalled := false
	parseMockBrowserWebSocket := js.Global().Get(jsGlobalObject).New()
	parseCloseMethod := js.FuncOf(func(parseThis js.Value, parseFunctionArgs []js.Value) interface{} {
		isCloseMethodCalled = true
		return nil
	})
	defer parseCloseMethod.Release()
	parseMockBrowserWebSocket.Set(jsMethodClose, parseCloseMethod)

	parseNetworkConnection := NewWebSocketConn(parseMockBrowserWebSocket)
	parseErr := parseNetworkConnection.Close()

	if parseErr != nil {
		parseT.Errorf("Close() error = %v, want nil", parseErr)
	}

	if !isCloseMethodCalled {
		parseT.Error("WebSocket.close() was not called")
	}
}
