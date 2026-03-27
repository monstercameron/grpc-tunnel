package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// startPublicFileServer starts the static example file server on an ephemeral local port.
func startPublicFileServer(parseT *testing.T, parseProjectRoot string) string {
	parseT.Helper()

	parsePublicDir := http.Dir(filepath.Join(parseProjectRoot, "examples", "_shared", "public"))
	parseFileServer := &http.Server{Handler: http.FileServer(parsePublicDir)}
	parseListener, parseErr := net.Listen("tcp", "127.0.0.1:0")
	if parseErr != nil {
		parseT.Fatalf("Failed to reserve public file server port: %v", parseErr)
	}

	parseServerURL := fmt.Sprintf("http://%s", parseListener.Addr().String())
	go func() {
		if parseServeErr := parseFileServer.Serve(parseListener); parseServeErr != nil && parseServeErr != http.ErrServerClosed {
			parseT.Logf("File server error: %v", parseServeErr)
		}
	}()

	parseT.Cleanup(func() {
		parseShutdownContext, parseCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer parseCancel()
		_ = parseFileServer.Shutdown(parseShutdownContext)
	})

	waitPublicFileServerReady(parseT, parseServerURL)
	return parseServerURL
}

// waitPublicFileServerReady blocks until the static file server accepts HTTP requests.
func waitPublicFileServerReady(parseT *testing.T, parseServerURL string) {
	parseT.Helper()

	parseClient := &http.Client{Timeout: 500 * time.Millisecond}
	parseDeadline := time.Now().Add(4 * time.Second)

	for time.Now().Before(parseDeadline) {
		parseResponse, parseErr := parseClient.Get(parseServerURL)
		if parseErr == nil {
			_ = parseResponse.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	parseT.Fatalf("Public file server did not become ready at %s before timeout", parseServerURL)
}
