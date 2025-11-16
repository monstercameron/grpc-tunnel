// Custom router example - integrate bridge with existing HTTP server
package main

import (
	"log"
	"net/http"
	"time"

	"grpc-tunnel/examples/_shared/helpers"
)

func main() {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write health response: %v", err)
		}
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// Your metrics implementation
		if _, err := w.Write([]byte("# Metrics here\n")); err != nil {
			log.Printf("Failed to write metrics response: %v", err)
		}
	})

	// gRPC bridge on specific path
	mux.Handle("/grpc", helpers.NewHandler(helpers.Config{
		TargetAddress: "localhost:50051",
	}))

	// Another bridge for a different service
	mux.Handle("/api/v2/grpc", helpers.NewHandler(helpers.Config{
		TargetAddress: "localhost:50052",
	}))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Server with multiple endpoints listening on :8080")
	log.Println("  /health       - Health check")
	log.Println("  /metrics      - Prometheus metrics")
	log.Println("  /grpc         - gRPC bridge to :50051")
	log.Println("  /api/v2/grpc  - gRPC bridge to :50052")
	log.Fatal(server.ListenAndServe())
}
