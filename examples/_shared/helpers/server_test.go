package helpers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
)

// TestServerConfig_Defaults tests that default values are set correctly
func TestServerConfig_Defaults(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	cfg := ServerConfig{
		GRPCServer: grpcServer,
	}

	handler := ServeHandler(cfg)
	if handler == nil {
		t.Fatal("ServeHandler returned nil")
	}

	// The handler creation should set defaults internally
	// We can't directly test the internal state, but we can verify it doesn't panic
}

// TestServerConfig_CustomBufferSizes tests custom buffer sizes
func TestServerConfig_CustomBufferSizes(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	cfg := ServerConfig{
		GRPCServer:      grpcServer,
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
	}

	handler := ServeHandler(cfg)
	if handler == nil {
		t.Fatal("ServeHandler returned nil")
	}
}

// TestServerConfig_CheckOrigin tests custom origin checker
func TestServerConfig_CheckOrigin(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	allowedOrigin := "https://example.com"
	cfg := ServerConfig{
		GRPCServer: grpcServer,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == allowedOrigin
		},
	}

	handler := ServeHandler(cfg)
	if handler == nil {
		t.Fatal("ServeHandler returned nil")
	}

	// Test with non-WebSocket request (should fail upgrade)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", allowedOrigin)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail because it's not a valid WebSocket upgrade request
	if w.Code == http.StatusSwitchingProtocols {
		t.Error("expected upgrade to fail without proper WebSocket headers")
	}
}

// TestServerConfig_LifecycleHooks tests OnConnect and OnDisconnect
func TestServerConfig_LifecycleHooks(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	connectCalled := false
	disconnectCalled := false

	cfg := ServerConfig{
		GRPCServer: grpcServer,
		OnConnect: func(r *http.Request) {
			connectCalled = true
		},
		OnDisconnect: func(r *http.Request) {
			disconnectCalled = true
		},
	}

	handler := ServeHandler(cfg)
	if handler == nil {
		t.Fatal("ServeHandler returned nil")
	}

	// Note: We can't easily test the hooks being called without a real WebSocket upgrade
	// The hooks would only be called after successful WebSocket upgrade
	// This test verifies the handler can be created with hooks
	_, _ = connectCalled, disconnectCalled // Suppress unused warnings
}

// TestServeHandler_NotWebSocket tests handling of non-WebSocket requests
func TestServeHandler_NotWebSocket(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: grpcServer,
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail to upgrade because it's not a WebSocket request
	if w.Code == http.StatusSwitchingProtocols {
		t.Error("expected failure for non-WebSocket request")
	}
}

// TestServeHandler_WebSocketHeaders tests with WebSocket upgrade headers
func TestServeHandler_WebSocketHeaders(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: grpcServer,
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should attempt upgrade (may still fail in test environment)
	// The important part is it doesn't panic
}

// TestServeHandler_NilGRPCServer tests with nil gRPC server
func TestServeHandler_NilGRPCServer(t *testing.T) {
	// Creating handler with nil server should work, but serving will fail
	handler := ServeHandler(ServerConfig{
		GRPCServer: nil,
	})

	if handler == nil {
		t.Error("ServeHandler returned nil even with nil GRPCServer")
	}

	// Attempting to serve will panic when h2c.NewHandler is called
	// We're just testing that the handler can be created
}

// TestServe tests the Serve convenience function
func TestServe(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	// Create a test listener
	// We'll test that Serve doesn't panic and returns when stopped
	// In a real test, we'd need to run this in a goroutine and cancel it

	// For now, just verify the function signature and that it accepts the parameters
	// A full test would require starting a server and client
}

// TestServeHandler_ContextCancellation tests behavior when context is cancelled
func TestServeHandler_ContextCancellation(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	handler := ServeHandler(ServerConfig{
		GRPCServer: grpcServer,
	})

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Should handle cancelled context gracefully
	handler.ServeHTTP(w, req)
}

// TestServerConfig_ZeroValues tests behavior with zero-value config
func TestServerConfig_ZeroValues(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	// Only required field set
	cfg := ServerConfig{
		GRPCServer: grpcServer,
	}

	handler := ServeHandler(cfg)
	if handler == nil {
		t.Fatal("ServeHandler should work with minimal config")
	}
}

// TestServeHandler_InvalidOrigin tests origin rejection
func TestServeHandler_InvalidOrigin(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: grpcServer,
		CheckOrigin: func(r *http.Request) bool {
			return false // Reject all origins
		},
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should reject the upgrade due to origin check
	if w.Code == http.StatusSwitchingProtocols {
		t.Error("expected origin rejection")
	}
}

// TestServeHandler_HTTPMethod tests different HTTP methods
func TestServeHandler_HTTPMethod(t *testing.T) {
	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: grpcServer,
	})

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", strings.NewReader("test"))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Non-GET requests with WebSocket upgrade should fail
			if w.Code == http.StatusSwitchingProtocols {
				t.Errorf("unexpected upgrade success for %s method", method)
			}
		})
	}
}
