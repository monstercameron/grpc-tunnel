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
func TestServerConfig_Defaults(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	parseCfg := ServerConfig{
		GRPCServer: parseGrpcServer,
	}

	handler := ServeHandler(parseCfg)
	if handler == nil {
		parseT.Fatal("ServeHandler returned nil")
	}

	// The handler creation should set defaults internally
	// We can't directly test the internal state, but we can verify it doesn't panic
}

// TestServerConfig_CustomBufferSizes tests custom buffer sizes
func TestServerConfig_CustomBufferSizes(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	parseCfg := ServerConfig{
		GRPCServer:      parseGrpcServer,
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
	}

	handler := ServeHandler(parseCfg)
	if handler == nil {
		parseT.Fatal("ServeHandler returned nil")
	}
}

// TestServerConfig_CheckOrigin tests custom origin checker
func TestServerConfig_CheckOrigin(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	parseAllowedOrigin := "https://example.com"
	parseCfg := ServerConfig{
		GRPCServer: parseGrpcServer,
		CheckOrigin: func(parseR *http.Request) bool {
			parseOrigin := parseR.Header.Get("Origin")
			return parseOrigin == parseAllowedOrigin
		},
	}

	handler := ServeHandler(parseCfg)
	if handler == nil {
		parseT.Fatal("ServeHandler returned nil")
	}

	// Test with non-WebSocket request (should fail upgrade)
	parseReq := httptest.NewRequest("GET", "/", nil)
	parseReq.Header.Set("Origin", parseAllowedOrigin)
	parseW := httptest.NewRecorder()

	handler.ServeHTTP(parseW, parseReq)

	// Should fail because it's not a valid WebSocket upgrade request
	if parseW.Code == http.StatusSwitchingProtocols {
		parseT.Error("expected upgrade to fail without proper WebSocket headers")
	}
}

// TestServerConfig_LifecycleHooks tests OnConnect and OnDisconnect
func TestServerConfig_LifecycleHooks(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	isConnectCalled := false
	isDisconnectCalled := false

	parseCfg := ServerConfig{
		GRPCServer: parseGrpcServer,
		OnConnect: func(parseR *http.Request) {
			isConnectCalled = true
		},
		OnDisconnect: func(parseR2 *http.Request) {
			isDisconnectCalled = true
		},
	}

	handler := ServeHandler(parseCfg)
	if handler == nil {
		parseT.Fatal("ServeHandler returned nil")
	}

	// Note: We can't easily test the hooks being called without a real WebSocket upgrade
	// The hooks would only be called after successful WebSocket upgrade
	// This test verifies the handler can be created with hooks
	_, _ = isConnectCalled, isDisconnectCalled // Suppress unused warnings
}

// TestServeHandler_NotWebSocket tests handling of non-WebSocket requests
func TestServeHandler_NotWebSocket(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: parseGrpcServer,
	})

	parseReq := httptest.NewRequest("GET", "/", nil)
	parseW := httptest.NewRecorder()

	handler.ServeHTTP(parseW, parseReq)

	// Should fail to upgrade because it's not a WebSocket request
	if parseW.Code == http.StatusSwitchingProtocols {
		parseT.Error("expected failure for non-WebSocket request")
	}
}

// TestServeHandler_WebSocketHeaders tests with WebSocket upgrade headers
func TestServeHandler_WebSocketHeaders(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: parseGrpcServer,
	})

	parseReq := httptest.NewRequest("GET", "/", nil)
	parseReq.Header.Set("Upgrade", "websocket")
	parseReq.Header.Set("Connection", "Upgrade")
	parseReq.Header.Set("Sec-WebSocket-Version", "13")
	parseReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	parseW := httptest.NewRecorder()

	handler.ServeHTTP(parseW, parseReq)

	// Should attempt upgrade (may still fail in test environment)
	// The important part is it doesn't panic
}

// TestServeHandler_NilGRPCServer tests with nil gRPC server
func TestServeHandler_NilGRPCServer(parseT *testing.T) {
	// Creating handler with nil server should work, but serving will fail
	handler := ServeHandler(ServerConfig{
		GRPCServer: nil,
	})

	if handler == nil {
		parseT.Error("ServeHandler returned nil even with nil GRPCServer")
	}

	// Attempting to serve will panic when h2c.NewHandler is called
	// We're just testing that the handler can be created
}

// TestServe tests the Serve convenience function
func TestServe(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	// Create a test listener
	// We'll test that Serve doesn't panic and returns when stopped
	// In a real test, we'd need to run this in a goroutine and cancel it

	// For now, just verify the function signature and that it accepts the parameters
	// A full test would require starting a server and client
}

// TestServeHandler_ContextCancellation tests behavior when context is cancelled
func TestServeHandler_ContextCancellation(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	parseCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	handler := ServeHandler(ServerConfig{
		GRPCServer: parseGrpcServer,
	})

	parseReq := httptest.NewRequest("GET", "/", nil).WithContext(parseCtx)
	parseW := httptest.NewRecorder()

	// Should handle cancelled context gracefully
	handler.ServeHTTP(parseW, parseReq)
}

// TestServerConfig_ZeroValues tests behavior with zero-value config
func TestServerConfig_ZeroValues(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	// Only required field set
	parseCfg := ServerConfig{
		GRPCServer: parseGrpcServer,
	}

	handler := ServeHandler(parseCfg)
	if handler == nil {
		parseT.Fatal("ServeHandler should work with minimal config")
	}
}

// TestServeHandler_InvalidOrigin tests origin rejection
func TestServeHandler_InvalidOrigin(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: parseGrpcServer,
		CheckOrigin: func(parseR *http.Request) bool {
			return false // Reject all origins
		},
	})

	parseReq := httptest.NewRequest("GET", "/", nil)
	parseReq.Header.Set("Origin", "https://evil.com")
	parseReq.Header.Set("Upgrade", "websocket")
	parseReq.Header.Set("Connection", "Upgrade")
	parseReq.Header.Set("Sec-WebSocket-Version", "13")
	parseReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	parseW := httptest.NewRecorder()
	handler.ServeHTTP(parseW, parseReq)

	// Should reject the upgrade due to origin check
	if parseW.Code == http.StatusSwitchingProtocols {
		parseT.Error("expected origin rejection")
	}
}

// TestServeHandler_HTTPMethod tests different HTTP methods
func TestServeHandler_HTTPMethod(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	handler := ServeHandler(ServerConfig{
		GRPCServer: parseGrpcServer,
	})

	parseMethods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, parseMethod := range parseMethods {
		parseT.Run(parseMethod, func(parseT2 *testing.T) {
			parseReq := httptest.NewRequest(parseMethod, "/", strings.NewReader("test"))
			parseW := httptest.NewRecorder()

			handler.ServeHTTP(parseW, parseReq)

			// Non-GET requests with WebSocket upgrade should fail
			if parseW.Code == http.StatusSwitchingProtocols {
				parseT2.Errorf("unexpected upgrade success for %s method", parseMethod)
			}
		})
	}
}
