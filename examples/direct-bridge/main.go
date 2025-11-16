// Direct gRPC-over-WebSocket example - no proxy needed!
package main

import (
	"log"
	"net/http"

	"google.golang.org/grpc"
	"grpc-tunnel/pkg/bridge"
	"grpc-tunnel/proto"
	"grpc-tunnel/cmd/server" // Reuse the TodoService implementation
)

func main() {
	// Create gRPC server with your service
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, server.NewTodoServer())

	// Serve gRPC directly over WebSocket
	http.Handle("/grpc", bridge.ServeHandler(bridge.ServerConfig{
		GRPCServer: grpcServer,
		OnConnect: func(r *http.Request) {
			log.Printf("Client connected: %s", r.RemoteAddr)
		},
		OnDisconnect: func(r *http.Request) {
			log.Printf("Client disconnected: %s", r.RemoteAddr)
		},
	}))

	log.Println("gRPC server listening on :8080 (WebSocket)")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
