// Production bridge example - full configuration
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"grpc-tunnel/pkg/bridge"
)

type customLogger struct{}

func (l *customLogger) Printf(format string, v ...interface{}) {
	log.Printf("[Bridge] "+format, v...)
}

func main() {
	addr := flag.String("addr", ":8443", "Bridge listen address")
	target := flag.String("target", "localhost:50051", "gRPC server address")
	cert := flag.String("cert", "cert.pem", "TLS certificate")
	key := flag.String("key", "key.pem", "TLS private key")
	allowedOrigins := flag.String("origins", "https://example.com", "Comma-separated allowed origins")
	flag.Parse()

	origins := strings.Split(*allowedOrigins, ",")

	// Create bridge with production config
	handler := bridge.NewHandler(bridge.Config{
		TargetAddress: *target,

		// Custom origin validation
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			for _, allowed := range origins {
				if origin == strings.TrimSpace(allowed) {
					return true
				}
			}
			log.Printf("Rejected connection from origin: %s", origin)
			return false
		},

		// Larger buffers for production
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,

		// Custom logger
		Logger: &customLogger{},

		// Connection lifecycle hooks
		OnConnect: func(r *http.Request) {
			log.Printf("Client connected: %s (Origin: %s, User-Agent: %s)",
				r.RemoteAddr,
				r.Header.Get("Origin"),
				r.Header.Get("User-Agent"))
		},

		OnDisconnect: func(r *http.Request) {
			log.Printf("Client disconnected: %s", r.RemoteAddr)
		},
	})

	log.Printf("Production bridge starting on %s", *addr)
	log.Printf("Proxying to gRPC server at %s", *target)
	log.Printf("Allowed origins: %v", origins)
	log.Fatal(http.ListenAndServeTLS(*addr, *cert, *key, handler))
}
