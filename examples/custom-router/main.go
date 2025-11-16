// Custom router example - integrate bridge with existing HTTP server
package main

import (
	"log"
	"net/http"

	"grpc-tunnel/pkg/bridge"
)

func main() {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// Your metrics implementation
		w.Write([]byte("# Metrics here\n"))
	})

	// gRPC bridge on specific path
	mux.Handle("/grpc", bridge.NewHandler(bridge.Config{
		TargetAddress: "localhost:50051",
	}))

	// Another bridge for a different service
	mux.Handle("/api/v2/grpc", bridge.NewHandler(bridge.Config{
		TargetAddress: "localhost:50052",
	}))

	log.Println("Server with multiple endpoints listening on :8080")
	log.Println("  /health       - Health check")
	log.Println("  /metrics      - Prometheus metrics")
	log.Println("  /grpc         - gRPC bridge to :50051")
	log.Println("  /api/v2/grpc  - gRPC bridge to :50052")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
