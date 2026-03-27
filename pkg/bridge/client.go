//go:build !js && !wasm

package bridge

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// ClientConfig configures low-level websocket dialing behavior.
type ClientConfig struct {
	// Headers configures optional HTTP headers for the websocket handshake.
	Headers http.Header
	// Subprotocols configures optional websocket subprotocol negotiation.
	Subprotocols []string
	// Proxy configures an optional proxy selector.
	Proxy func(*http.Request) (*url.URL, error)
	// HandshakeTimeout limits websocket handshake duration.
	HandshakeTimeout time.Duration
	// TLSConfig configures TLS behavior for secure websocket dialing.
	TLSConfig *tls.Config
	// ShouldEnableCompression enables websocket per-message compression where supported.
	ShouldEnableCompression bool
}

// DialOption creates a gRPC dial option that connects via WebSocket.
// Use this on the client side to establish gRPC connections over WebSocket.
//
// This function returns a grpc.DialOption that can be passed to grpc.Dial() or
// grpc.DialContext() to make the gRPC client connect through a WebSocket instead
// of a regular TCP connection. This is essential for environments where direct TCP
// connections are not available (e.g., browsers) or when you need to tunnel through
// firewalls that only allow HTTP/WebSocket traffic.
//
// Parameters:
//   - websocketURL: The WebSocket URL to connect to (e.g., "ws://localhost:8080/grpc" or "wss://example.com/grpc")
//
// Returns:
//   - grpc.DialOption: A dial option that configures gRPC to use WebSocket transport
//
// Example:
//
//	conn, err := grpc.Dial("localhost:8080",
//	    bridge.DialOption("ws://localhost:8080/grpc"),
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
// Note: The target address parameter in grpc.Dial() is ignored when using this
// DialOption - the connection is made to the WebSocket URL instead.
func DialOption(parseWebsocketURL string) grpc.DialOption {
	return DialOptionWithConfig(parseWebsocketURL, ClientConfig{})
}

// DialOptionWithConfig creates a websocket gRPC dial option with additive client settings.
func DialOptionWithConfig(parseWebsocketURL string, parseConfig ClientConfig) grpc.DialOption {
	parseDialer := websocket.Dialer{
		TLSClientConfig:   parseConfig.TLSConfig,
		Subprotocols:      append([]string{}, parseConfig.Subprotocols...),
		Proxy:             parseConfig.Proxy,
		HandshakeTimeout:  parseConfig.HandshakeTimeout,
		WriteBufferPool:   buildWebSocketWriteBufferPool(parseDefaultWebSocketBufferSize),
		EnableCompression: parseConfig.ShouldEnableCompression,
	}
	parseHeadersTemplate := http.Header(nil)
	if parseConfig.Headers != nil {
		parseHeadersTemplate = parseConfig.Headers.Clone()
	}

	return grpc.WithContextDialer(func(parseCtx context.Context, parseGrpcTargetAddress string) (net.Conn, error) {
		// Dial the WebSocket connection using the provided URL.
		// The grpcTargetAddress parameter (from grpc.Dial) is ignored because the WebSocket
		// URL contains the complete target address.
		parseDialerCopy := parseDialer
		parseWebsocketConnection, _, parseErr := parseDialerCopy.DialContext(parseCtx, parseWebsocketURL, parseHeadersTemplate)
		if parseErr != nil {
			// WebSocket connection failed (network error, DNS resolution, etc.)
			return nil, parseErr
		}

		// Wrap the WebSocket as a net.Conn so gRPC can use it.
		// This allows gRPC to send HTTP/2 frames over the WebSocket.
		return NewWebSocketConn(parseWebsocketConnection), nil
	})
}
