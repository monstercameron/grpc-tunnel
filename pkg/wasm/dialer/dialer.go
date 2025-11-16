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

// newWebSocketDialer creates a custom gRPC dialer that establishes a WebSocket
// connection and then layers HTTP/2 on top of it.
func newWebSocketDialer(webSocketURL string) func(context.Context, string) (net.Conn, error) {
	return func(dialContext context.Context, _ string) (net.Conn, error) {
		// Create a new WebSocket object in the browser's JavaScript environment.
		jsWebSocketConstructor := js.Global().Get("WebSocket")
		if !jsWebSocketConstructor.Truthy() {
			return nil, status.Errorf(codes.Unavailable, "WASM: WebSocket not available in this environment")
		}

		// Create a new WebSocket instance.
		jsWebSocket := jsWebSocketConstructor.New(webSocketURL)

		// Set binary type to 'arraybuffer' for binary data
		jsWebSocket.Set("binaryType", "arraybuffer")

		// Create our net.Conn adapter for this WebSocket.
		webSocketNetworkConnection := NewWebSocketConn(jsWebSocket)

		// Set up an error listener for the WebSocket to ensure the connection context is cancelled if the WebSocket errors.
		errorChannel := make(chan error, 1)
		jsWebSocket.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			log.Printf("WASM: WebSocket error: %v", args[0]) // Log the JS error event
			errorChannel <- status.Errorf(codes.Unavailable, "WASM: WebSocket error during connection setup")
			return nil
		}))
		// We'll also specifically listen for the 'onopen' event to know when the WebSocket is ready.
		openChannel := make(chan struct{}, 1)
		jsWebSocket.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			openChannel <- struct{}{}
			return nil
		}))

		// Wait for the WebSocket to open or an error to occur.
		select {
		case <-openChannel:
			// WebSocket connected successfully!
			log.Println("WASM: WebSocket connection opened.")
		case err := <-errorChannel:
			return nil, err
		case <-dialContext.Done():
			// The dialing context was cancelled or timed out before the WebSocket opened.
			// Close the WebSocket if it's still connecting.
			if jsWebSocket.Get("readyState").Int() == 0 { // CONNECTING
				jsWebSocket.Call("close")
			}
			return nil, dialContext.Err()
		}

		// The WebSocket is now open and ready to use.
		// Return the WebSocketConn directly - gRPC will handle HTTP/2 framing.
		return webSocketNetworkConnection, nil
	}
}

// New creates a grpc.DialOption that can be used to dial a gRPC server over a WebSocket.
// The webSocketURL should be the full URL to the WebSocket endpoint (e.g., "ws://localhost:8080/ws").
func New(webSocketURL string) grpc.DialOption {
	return grpc.WithContextDialer(newWebSocketDialer(webSocketURL))
}
