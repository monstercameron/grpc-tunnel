//go:build js && wasm

package grpctunnel

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"syscall/js"
)

// ClientOption is a no-op in WASM (TLS handled by browser).
type ClientOption func(*clientOptions)

type clientOptions struct{}

// WithTLS is a no-op in WASM builds since browsers handle TLS automatically.
func WithTLS(_ interface{}) ClientOption {
	return func(o *clientOptions) {}
}

// Dial creates a gRPC client connection over WebSocket in the browser.
// The target must be a WebSocket URL (ws:// or wss://).
//
// Example:
//
//	conn, err := grpctunnel.Dial("ws://localhost:8080",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
func Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), target, opts...)
}

// DialContext creates a gRPC client connection over WebSocket in the browser with context.
func DialContext(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Filter out grpctunnel options (not needed in WASM)
	var grpcOpts []grpc.DialOption
	for _, opt := range opts {
		grpcOpts = append(grpcOpts, opt)
	}

	// Add browser WebSocket dialer
	grpcOpts = append(grpcOpts, grpc.WithContextDialer(newBrowserWebSocketDialer(target)))

	return grpc.DialContext(ctx, target, grpcOpts...)
}

// newBrowserWebSocketDialer creates a dialer for browser WebSocket connections.
// This is a simplified version that uses the browser's native WebSocket API.
func newBrowserWebSocketDialer(wsURL string) func(context.Context, string) (interface{}, error) {
	return func(dialCtx context.Context, _ string) (interface{}, error) {
		// Check for WebSocket API
		wsConstructor := js.Global().Get("WebSocket")
		if !wsConstructor.Truthy() {
			return nil, status.Errorf(codes.Unavailable, "WebSocket not available in browser")
		}

		// Create WebSocket
		ws := wsConstructor.New(wsURL)
		ws.Set("binaryType", "arraybuffer")

		// Wait for connection
		openChan := make(chan struct{}, 1)
		errChan := make(chan error, 1)

		ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			openChan <- struct{}{}
			return nil
		}))

		ws.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			errChan <- status.Errorf(codes.Unavailable, "WebSocket connection failed")
			return nil
		}))

		select {
		case <-openChan:
			return newBrowserWebSocketConn(ws), nil
		case err := <-errChan:
			return nil, err
		case <-dialCtx.Done():
			if ws.Get("readyState").Int() == 0 { // CONNECTING
				ws.Call("close")
			}
			return nil, dialCtx.Err()
		}
	}
}

// Placeholder for browser WebSocket conn - actual implementation would go in conn_wasm.go
func newBrowserWebSocketConn(ws js.Value) interface{} {
	// This would return a net.Conn-like wrapper around the browser WebSocket
	// For now, returning the js.Value directly
	return ws
}
