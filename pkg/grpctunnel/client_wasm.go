//go:build js && wasm

package grpctunnel

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/monstercameron/GoGRPCBridge/pkg/wasm/dialer"

	"google.golang.org/grpc"
)

// ClientOption is a no-op in WASM (TLS handled by browser).
type ClientOption func(*clientOptions)

type clientOptions struct{}

// WithTLS is a no-op in WASM builds since browsers handle TLS automatically.
func WithTLS(_ interface{}) ClientOption {
	return func(parseO *clientOptions) {}
}

// inferBrowserWebSocketURL infers the WebSocket URL from the browser's current location.
// If target is empty or just a path, it uses window.location to build the URL.
func inferBrowserWebSocketURL(parseTarget string) string {
	// If already a full WebSocket URL, use it
	if len(parseTarget) >= 5 && parseTarget[:5] == "ws://" {
		return parseTarget
	}
	if len(parseTarget) >= 6 && parseTarget[:6] == "wss://" {
		return parseTarget
	}

	// Access window.location
	parseLocation := js.Global().Get("location")
	if !parseLocation.Truthy() {
		// Fallback if window.location not available (shouldn't happen in browser)
		if parseTarget == "" {
			return "ws://localhost:8080"
		}
		// If target looks like host:port, add ws://
		return "ws://" + parseTarget
	}

	// Determine scheme (ws or wss based on current page)
	parseProtocol := parseLocation.Get("protocol").String()
	parseScheme := "ws"
	if parseProtocol == "https:" {
		parseScheme = "wss"
	}

	// Get host (includes port if present)
	parseHost := parseLocation.Get("host").String()

	// If target is empty, connect to same host
	if parseTarget == "" {
		return fmt.Sprintf("%s://%s", parseScheme, parseHost)
	}

	// If target starts with "/", it's a path - use current host
	if len(parseTarget) > 0 && parseTarget[0] == '/' {
		return fmt.Sprintf("%s://%s%s", parseScheme, parseHost, parseTarget)
	}

	// If target is just "host:port", add scheme
	return fmt.Sprintf("%s://%s", parseScheme, parseTarget)
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
func Dial(parseTarget string, parseOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), parseTarget, parseOpts...)
}

// DialContext creates a gRPC client connection over WebSocket in the browser with context.
func DialContext(parseCtx context.Context, parseTarget string, parseOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Infer WebSocket URL from browser location if needed
	parseWsURL := inferBrowserWebSocketURL(parseTarget)

	// Filter out grpctunnel options (not needed in WASM)
	var parseGrpcOpts []grpc.DialOption
	for _, parseOpt := range parseOpts {
		parseGrpcOpts = append(parseGrpcOpts, parseOpt)
	}

	// Use the existing WASM dialer with inferred URL
	parseGrpcOpts = append(parseGrpcOpts, dialer.New(parseWsURL))

	// Use inferred URL as target for gRPC
	if parseTarget == "" {
		parseTarget = parseWsURL
	}

	return grpc.DialContext(parseCtx, parseTarget, parseGrpcOpts...)
}
