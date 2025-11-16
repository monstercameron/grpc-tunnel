// Simple bridge server example - minimal configuration
package main

import (
	"log"
	"net/http"

	"grpc-tunnel/pkg/bridge"
)

func main() {
	// Create bridge with minimal config
	handler := bridge.NewHandler(bridge.Config{
		TargetAddress: "localhost:50051",
	})

	log.Println("Bridge listening on :8080")
	log.Println("Proxying to gRPC server at localhost:50051")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
