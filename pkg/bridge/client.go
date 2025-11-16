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
// Example:
//
//	conn, err := grpc.Dial("localhost:8080",
//	    bridge.DialOption("ws://localhost:8080/grpc"),
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
func DialOption(websocketURL string) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		// Dial WebSocket
		ws, _, err := websocket.DefaultDialer.DialContext(ctx, websocketURL, nil)
		if err != nil {
			return nil, err
		}
		// Return WebSocket wrapped as net.Conn
		return NewWebSocketConn(ws), nil
	})
}
