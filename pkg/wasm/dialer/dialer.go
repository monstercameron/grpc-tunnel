//go:build js && wasm

package dialer

import (
	"context"
	"log"
	"net"
	"syscall/js"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// WebSocket ready states (https://developer.mozilla.org/en-US/docs/Web/API/WebSocket/readyState)
	webSocketStateConnecting = 0 // Connection not yet open
	webSocketStateOpen       = 1 // Connection open and ready to communicate
	webSocketStateClosing    = 2 // Connection in the process of closing
	webSocketStateClosed     = 3 // Connection closed or couldn't be opened

	// JavaScript WebSocket event handlers
	jsEventOnOpen = "onopen"

	// JavaScript WebSocket properties
	jsPropertyReadyState = "readyState"
)

// newBrowserWebSocketDialer creates a custom gRPC dialer that establishes a WebSocket
// connection in the browser environment and prepares it for gRPC communication.
//
// This function returns a dialer function that:
// 1. Creates a browser WebSocket connection to the specified URL
// 2. Configures it for binary communication (required for gRPC)
// 3. Waits for the connection to establish or fail
// 4. Returns a net.Conn adapter that gRPC can use
//
// The dialer handles the asynchronous nature of browser WebSocket connections
// by using channels to synchronize the connection establishment.
//
// Parameters:
//   - webSocketURL: The WebSocket URL to connect to (e.g., "ws://localhost:8080/grpc")
//
// Returns:
//   - A dialer function compatible with grpc.WithContextDialer
func newBrowserWebSocketDialer(webSocketURL string) func(context.Context, string) (net.Conn, error) {
	return func(dialContext context.Context, grpcTargetAddress string) (net.Conn, error) {
		// Access the browser's WebSocket constructor from the JavaScript global scope.
		// This is the standard browser WebSocket API.
		browserWebSocketConstructor := js.Global().Get(jsGlobalWebSocket)
		if !browserWebSocketConstructor.Truthy() {
			// WebSocket API not available - this shouldn't happen in modern browsers
			// but could occur in non-browser WASM environments
			return nil, status.Errorf(codes.Unavailable, "WASM: WebSocket not available in this environment")
		}

		// Create a new browser WebSocket instance with the provided URL.
		// This initiates the WebSocket handshake in the background.
		browserWebSocket := browserWebSocketConstructor.New(webSocketURL)

		// Configure the WebSocket to use ArrayBuffer for binary data.
		// gRPC requires binary communication, so we must set binaryType to 'arraybuffer'.
		// The alternative 'blob' type would be incompatible with our data handling.
		browserWebSocket.Set(jsPropertyBinaryType, jsBinaryTypeArrayBuffer)

		// Create our net.Conn adapter that wraps this browser WebSocket.
		// This adapter translates between the event-driven WebSocket API
		// and the synchronous Read/Write interface that gRPC expects.
		webSocketNetworkConnection := NewWebSocketConn(browserWebSocket)

		// Set up error handling for the WebSocket connection.
		// Browser WebSocket errors are asynchronous events, so we use a channel
		// to communicate them back to this synchronous function.
		connectionErrorChannel := make(chan error, 1)
		browserWebSocket.Set("onerror", js.FuncOf(func(this js.Value, eventArgs []js.Value) interface{} {
			// Log the error event for debugging purposes
			log.Printf("WASM: WebSocket error: %v", eventArgs[0])
			// Send the error to our channel so we can return it
			connectionErrorChannel <- status.Errorf(codes.Unavailable, "WASM: WebSocket error during connection setup")
			return nil
		}))

		// Set up a listener for the 'onopen' event to know when the connection is ready.
		// Browser WebSocket connections are asynchronous, so we must wait for this event
		// before we can start using the connection.
		connectionOpenChannel := make(chan struct{}, 1)
		browserWebSocket.Set(jsEventOnOpen, js.FuncOf(func(this js.Value, eventArgs []js.Value) interface{} {
			// Signal that the connection is now open and ready to use
			connectionOpenChannel <- struct{}{}
			return nil
		}))

		// Wait for one of three outcomes:
		// 1. Connection opens successfully
		// 2. Connection fails with an error
		// 3. Context is cancelled (timeout or explicit cancellation)
		select {
		case <-connectionOpenChannel:
			// Success! The WebSocket is now connected and ready.
			log.Println("WASM: WebSocket connection opened.")
		case err := <-connectionErrorChannel:
			// Connection failed during the handshake
			return nil, err
		case <-dialContext.Done():
			// The dialing context was cancelled or timed out before connection completed.
			// Clean up by closing the WebSocket if it's still in the CONNECTING state.
			if browserWebSocket.Get(jsPropertyReadyState).Int() == webSocketStateConnecting {
				browserWebSocket.Call(jsMethodClose)
			}
			// Return the context error (DeadlineExceeded or Canceled)
			return nil, dialContext.Err()
		}

		// The WebSocket is now open and ready to use.
		// Return the net.Conn adapter so gRPC can send HTTP/2 frames over it.
		return webSocketNetworkConnection, nil
	}
}

// New creates a grpc.DialOption that can be used to dial a gRPC server over a WebSocket
// from a WebAssembly environment (browser).
//
// This is the WASM equivalent of bridge.DialOption for browser-based clients.
// It configures gRPC to use browser WebSocket APIs instead of traditional TCP sockets,
// which are not available in browser environments.
//
// The returned DialOption should be passed to grpc.Dial() or grpc.DialContext() along
// with other required options like credentials.
//
// Parameters:
//   - webSocketURL: The full WebSocket URL to connect to, including scheme (ws:// or wss://),
//     host, port, and path (e.g., "ws://localhost:8080/grpc" or "wss://api.example.com/grpc")
//
// Returns:
//   - grpc.DialOption: A dial option that configures gRPC to use browser WebSocket transport
//
// Example:
//
//	ctx := context.Background()
//	conn, err := grpc.DialContext(
//	    ctx,
//	    "localhost:8080", // This target is ignored; WebSocket URL is used instead
//	    dialer.New("ws://localhost:8080/grpc"),
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//
//	client := proto.NewYourServiceClient(conn)
//	// Use the client normally
//
// Note: This function is only available in WASM builds (//go:build js && wasm).
// For non-WASM Go code, use bridge.DialOption instead.
func New(webSocketURL string) grpc.DialOption {
	return grpc.WithContextDialer(newBrowserWebSocketDialer(webSocketURL))
}
