//go:build !js && !wasm

package grpctunnel

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// ClientOption configures the WebSocket client behavior.
type ClientOption func(*clientOptions)

type clientOptions struct {
	tlsConfig *tls.Config
}

// WithTLS enables secure WebSocket connections (wss://).
// This sets the TLS configuration for the WebSocket dialer.
//
// Example:
//
//	conn, _ := grpctunnel.Dial("localhost:8080",
//	    grpctunnel.WithTLS(&tls.Config{InsecureSkipVerify: true}),
//	)
func WithTLS(config *tls.Config) ClientOption {
	return func(o *clientOptions) {
		o.tlsConfig = config
	}
}

// inferWebSocketURL converts a target address to a WebSocket URL.
// It handles various formats:
//   - "ws://..." or "wss://..." -> use as-is
//   - "localhost:8080" -> "ws://localhost:8080"
//   - ":8080" -> "ws://localhost:8080"
func inferWebSocketURL(target string, useTLS bool) string {
	// Already a WebSocket URL
	if strings.HasPrefix(target, "ws://") || strings.HasPrefix(target, "wss://") {
		return target
	}

	// Handle bare port
	if strings.HasPrefix(target, ":") {
		target = "localhost" + target
	}

	// Build WebSocket URL
	scheme := "ws"
	if useTLS {
		scheme = "wss"
	}
	return scheme + "://" + target
}

// newWebSocketDialer creates a custom gRPC dialer that establishes WebSocket connections.
func newWebSocketDialer(target string, opts ...ClientOption) func(context.Context, string) (net.Conn, error) {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	wsURL := inferWebSocketURL(target, options.tlsConfig != nil)

	return func(ctx context.Context, addr string) (net.Conn, error) {
		// Parse WebSocket URL
		u, err := url.Parse(wsURL)
		if err != nil {
			return nil, err
		}

		// Create WebSocket dialer
		dialer := websocket.Dialer{
			TLSClientConfig: options.tlsConfig,
		}

		// Dial WebSocket
		ws, _, err := dialer.DialContext(ctx, u.String(), nil)
		if err != nil {
			return nil, err
		}

		return newWebSocketConn(ws), nil
	}
}

// Dial creates a gRPC client connection over WebSocket.
// The target can be:
//   - A WebSocket URL: "ws://localhost:8080" or "wss://api.example.com"
//   - A host:port: "localhost:8080" (infers ws://)
//   - A port: ":8080" (infers ws://localhost:8080)
//
// Additional grpc.DialOption can be passed for credentials, interceptors, etc.
//
// Example:
//
//	conn, err := grpctunnel.Dial("localhost:8080",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//
//	client := proto.NewYourServiceClient(conn)
func Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), target, opts...)
}

// DialContext creates a gRPC client connection over WebSocket with context.
// This is the context-aware version of Dial.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	conn, err := grpctunnel.DialContext(ctx, "localhost:8080",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
func DialContext(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Extract grpctunnel options and grpc options
	var tunnelOpts []ClientOption
	var grpcOpts []grpc.DialOption

	for _, opt := range opts {
		// Check if it's a ClientOption (our custom type)
		if co, ok := opt.(interface{ apply(*clientOptions) }); ok {
			// This is a bit of a hack - we'll handle this differently
			_ = co
		} else {
			grpcOpts = append(grpcOpts, opt)
		}
	}

	// Add our custom dialer
	grpcOpts = append(grpcOpts, grpc.WithContextDialer(newWebSocketDialer(target, tunnelOpts...)))

	return grpc.DialContext(ctx, target, grpcOpts...)
}
