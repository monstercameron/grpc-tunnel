//go:build !js && !wasm

package bridge

import (
	"context"
	"net"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

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
func DialOption(websocketURL string) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, grpcTargetAddress string) (net.Conn, error) {
		// Dial the WebSocket connection using the provided URL.
		// The grpcTargetAddress parameter (from grpc.Dial) is ignored because the WebSocket
		// URL contains the complete target address.
		websocketConnection, _, err := websocket.DefaultDialer.DialContext(ctx, websocketURL, nil)
		if err != nil {
			// WebSocket connection failed (network error, DNS resolution, etc.)
			return nil, err
		}

		// Wrap the WebSocket as a net.Conn so gRPC can use it.
		// This allows gRPC to send HTTP/2 frames over the WebSocket.
		return NewWebSocketConn(websocketConnection), nil
	})
}
