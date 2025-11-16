// Simple bridge server example - minimal configuration
package main

import (
	"log"
	"net/http"

	"grpc-tunnel/examples/_shared/helpers"
)

func main() {
	// Create bridge with minimal config
	handler := helpers.NewHandler(helpers.Config{
		TargetAddress: "localhost:50051",
	})

	log.Println("Bridge listening on :8080")
	log.Println("Proxying to gRPC server at localhost:50051")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
