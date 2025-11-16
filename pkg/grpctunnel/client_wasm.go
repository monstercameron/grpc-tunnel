//go:build js && wasm

package grpctunnel

import (
	"context"
	"fmt"
	"syscall/js"

	"grpc-tunnel/pkg/wasm/dialer"

	"google.golang.org/grpc"
)

// ClientOption is a no-op in WASM (TLS handled by browser).
type ClientOption func(*clientOptions)

type clientOptions struct{}

// WithTLS is a no-op in WASM builds since browsers handle TLS automatically.
func WithTLS(_ interface{}) ClientOption {
	return func(o *clientOptions) {}
}

// inferBrowserWebSocketURL infers the WebSocket URL from the browser's current location.
// If target is empty or just a path, it uses window.location to build the URL.
func inferBrowserWebSocketURL(target string) string {
	// If already a full WebSocket URL, use it
	if len(target) >= 5 && target[:5] == "ws://" {
		return target
	}
	if len(target) >= 6 && target[:6] == "wss://" {
		return target
	}

	// Access window.location
	location := js.Global().Get("location")
	if !location.Truthy() {
		// Fallback if window.location not available (shouldn't happen in browser)
		if target == "" {
			return "ws://localhost:8080"
		}
		// If target looks like host:port, add ws://
		return "ws://" + target
	}

	// Determine scheme (ws or wss based on current page)
	protocol := location.Get("protocol").String()
	scheme := "ws"
	if protocol == "https:" {
		scheme = "wss"
	}

	// Get host (includes port if present)
	host := location.Get("host").String()

	// If target is empty, connect to same host
	if target == "" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}

	// If target starts with "/", it's a path - use current host
	if len(target) > 0 && target[0] == '/' {
		return fmt.Sprintf("%s://%s%s", scheme, host, target)
	}

	// If target is just "host:port", add scheme
	return fmt.Sprintf("%s://%s", scheme, target)
}

// Dial creates a gRPC client connection over WebSocket in the browser.
//
// The target can be:
//   - Empty "" - automatically uses current page's host (ws://current-host or wss://current-host)
//   - A path "/grpc" - uses current host + path (ws://current-host/grpc)
//   - A WebSocket URL "ws://localhost:8080" or "wss://api.example.com"
//   - A host:port "localhost:8080" - adds ws:// or wss:// based on current page protocol
//
// Example (automatic):
//
//	// On page https://example.com, automatically connects to wss://example.com
//	conn, err := grpctunnel.Dial("",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//
// Example (explicit):
//
//	conn, err := grpctunnel.Dial("ws://localhost:8080",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
func Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), target, opts...)
}

// DialContext creates a gRPC client connection over WebSocket in the browser with context.
func DialContext(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Infer WebSocket URL from browser location if needed
	wsURL := inferBrowserWebSocketURL(target)

	// Filter out grpctunnel options (not needed in WASM)
	var grpcOpts []grpc.DialOption
	for _, opt := range opts {
		grpcOpts = append(grpcOpts, opt)
	}

	// Use the existing WASM dialer with inferred URL
	grpcOpts = append(grpcOpts, dialer.New(wsURL))

	// Use inferred URL as target for gRPC
	if target == "" {
		target = wsURL
	}

	return grpc.DialContext(ctx, target, grpcOpts...)
}
