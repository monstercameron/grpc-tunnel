package main

import (
	"flag"
	"log"
	"net/http"

	"grpc-tunnel/pkg/bridge"
)

func main() {
	// Parse command-line flags
	listenAddress := flag.String("listen", ":8080", "Address to listen on for WebSocket connections (e.g., :8080)")
	targetGRPCServerAddress := flag.String("target", "localhost:50051", "Address of the backend gRPC server (e.g., localhost:50051)")
	tlsCertPath := flag.String("tls-cert", "", "Path to the TLS certificate file (for WSS)")
	tlsKeyPath := flag.String("tls-key", "", "Path to the TLS private key file (for WSS)")
	flag.Parse()

	log.Printf("Starting gRPC-over-WebSocket bridge...")
	log.Printf("Listening on: %s", *listenAddress)
	log.Printf("Targeting gRPC server: %s", *targetGRPCServerAddress)

	// Create the bridge handler
	bridgeHTTPHandler := bridge.NewHandler(*targetGRPCServerAddress)

	// Set up the HTTP server
	httpServer := &http.Server{
		Addr:    *listenAddress,
		Handler: bridgeHTTPHandler, // Our bridge handler will handle all requests
	}

	// Start the server
	var serveError error
	if *tlsCertPath != "" && *tlsKeyPath != "" {
		log.Printf("Serving with TLS (WSS) using cert: %s and key: %s", *tlsCertPath, *tlsKeyPath)
		serveError = httpServer.ListenAndServeTLS(*tlsCertPath, *tlsKeyPath)
	} else {
		log.Println("Serving without TLS (WS)")
		serveError = httpServer.ListenAndServe()
	}

	if serveError != nil && serveError != http.ErrServerClosed {
		log.Fatalf("Bridge server failed: %v", serveError)
	}
	log.Println("Bridge server gracefully shut down.")
}
