//go:build js && wasm

package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"grpc-tunnel/examples/_shared/proto" // Our generated protobuf code
	"grpc-tunnel/pkg/wasm/dialer"        // Our new WASM dialer library
)

func main() {
	log.Println("WASM: Starting new gRPC client...")

	// Establish a gRPC connection via the WebSocket tunnel
	dialContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcConnection, dialError := grpc.DialContext(
		dialContext,
		"localhost:5000", // Target will be ignored, dialer handles connection
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		dialer.New("ws://localhost:5000/"), // Use our custom WASM dialer
	)
	if dialError != nil {
		log.Fatalf("WASM: Failed to connect to gRPC server via WebSocket: %v", dialError)
	}
	defer func() {
		if closeError := grpcConnection.Close(); closeError != nil {
			log.Printf("WASM: Error closing gRPC connection: %v", closeError)
		}
	}()

	todoServiceClient := proto.NewTodoServiceClient(grpcConnection)

	// Make a simple CreateTodo RPC call
	createRequest := &proto.CreateTodoRequest{Text: "Learn gRPC-over-WebSocket"}
	contextWithTimeout, contextCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer contextCancel()

	createResponse, createError := todoServiceClient.CreateTodo(contextWithTimeout, createRequest)
	if createError != nil {
		log.Fatalf("WASM: Failed to create todo: %v", createError)
	}
	log.Printf("WASM: Created new todo: ID=%s, Text=%s, Done=%t",
		createResponse.GetTodo().GetId(),
		createResponse.GetTodo().GetText(),
		createResponse.GetTodo().GetDone(),
	)

	// Keep the WASM runtime alive
	select {}
}
