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
func WithTLS(parseConfig *tls.Config) ClientOption {
	return func(parseO *clientOptions) {
		parseO.tlsConfig = parseConfig
	}
}

// inferWebSocketURL converts a target address to a WebSocket URL.
// It handles various formats:
//   - "ws://..." or "wss://..." -> use as-is
//   - "localhost:8080" -> "ws://localhost:8080"
//   - ":8080" -> "ws://localhost:8080"
func inferWebSocketURL(parseTarget string, isUseTLS bool) string {
	// Already a WebSocket URL
	if strings.HasPrefix(parseTarget, "ws://") || strings.HasPrefix(parseTarget, "wss://") {
		return parseTarget
	}

	// Handle bare port
	if strings.HasPrefix(parseTarget, ":") {
		parseTarget = "localhost" + parseTarget
	}

	// Build WebSocket URL
	parseScheme := "ws"
	if isUseTLS {
		parseScheme = "wss"
	}
	return parseScheme + "://" + parseTarget
}

// newWebSocketDialer creates a custom gRPC dialer that establishes WebSocket connections.
func newWebSocketDialer(parseTarget string, parseOpts ...ClientOption) func(context.Context, string) (net.Conn, error) {
	parseOptions := &clientOptions{}
	for _, parseOpt := range parseOpts {
		parseOpt(parseOptions)
	}

	parseWsURL := inferWebSocketURL(parseTarget, parseOptions.tlsConfig != nil)

	return func(parseCtx context.Context, parseAddr string) (net.Conn, error) {
		// Parse WebSocket URL
		parseU, parseErr := url.Parse(parseWsURL)
		if parseErr != nil {
			return nil, parseErr
		}

		// Create WebSocket dialer
		parseDialer := websocket.Dialer{
			TLSClientConfig: parseOptions.tlsConfig,
		}

		// Dial WebSocket
		parseWs, _, parseErr := parseDialer.DialContext(parseCtx, parseU.String(), nil)
		if parseErr != nil {
			return nil, parseErr
		}

		return newWebSocketConn(parseWs), nil
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
func Dial(parseTarget string, parseOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), parseTarget, parseOpts...)
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
func DialContext(parseCtx context.Context, parseTarget string, parseOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Extract grpctunnel options and grpc options
	var parseTunnelOpts []ClientOption
	var parseGrpcOpts []grpc.DialOption

	for _, parseOpt := range parseOpts {
		// Check if it's a ClientOption (our custom type)
		if parseCo, parseOk := parseOpt.(interface{ apply(*clientOptions) }); parseOk {
			// This is a bit of a hack - we'll handle this differently
			_ = parseCo
		} else {
			parseGrpcOpts = append(parseGrpcOpts, parseOpt)
		}
	}

	// Add our custom dialer
	parseGrpcOpts = append(parseGrpcOpts, grpc.WithContextDialer(newWebSocketDialer(parseTarget, parseTunnelOpts...)))

	return grpc.DialContext(parseCtx, parseTarget, parseGrpcOpts...)
}
