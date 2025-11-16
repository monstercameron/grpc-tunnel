package bridge

import (
	"net/http"
	"testing"
)

// TestNewHandler_Interface ensures that NewHandler returns an http.Handler.
func TestNewHandler_Interface(t *testing.T) {
	// NewHandler should return an http.Handler.
	// We don't need a real gRPC server for this interface check.
	var handler http.Handler
	handler = NewHandler("localhost:50051") // Dummy target address

	if handler == nil {
		t.Fatal("NewHandler returned nil, expected an http.Handler implementation")
	}
}
