package bridge

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewHandler_Defaults verifies that NewHandler sets proper default values
func TestNewHandler_Defaults(t *testing.T) {
	h := NewHandler(Config{
		TargetAddress: "localhost:50051",
	})

	if h.config.ReadBufferSize != 4096 {
		t.Errorf("Expected ReadBufferSize 4096, got %d", h.config.ReadBufferSize)
	}
	if h.config.WriteBufferSize != 4096 {
		t.Errorf("Expected WriteBufferSize 4096, got %d", h.config.WriteBufferSize)
	}
	if h.config.CheckOrigin == nil {
		t.Error("Expected CheckOrigin to be set to default")
	}
	if h.logger == nil {
		t.Error("Expected logger to be set")
	}
}

// TestNewHandler_CustomConfig verifies that NewHandler respects custom configuration
func TestNewHandler_CustomConfig(t *testing.T) {
	customOrigin := func(r *http.Request) bool { return false }
	customLogger := &testLogger{}

	h := NewHandler(Config{
		TargetAddress:   "localhost:9999",
		ReadBufferSize:  8192,
		WriteBufferSize: 16384,
		CheckOrigin:     customOrigin,
		Logger:          customLogger,
		OnConnect: func(r *http.Request) {
			// Connect callback
		},
		OnDisconnect: func(r *http.Request) {
			// Disconnect callback
		},
	})

	if h.config.ReadBufferSize != 8192 {
		t.Errorf("Expected ReadBufferSize 8192, got %d", h.config.ReadBufferSize)
	}
	if h.config.WriteBufferSize != 16384 {
		t.Errorf("Expected WriteBufferSize 16384, got %d", h.config.WriteBufferSize)
	}
	if h.logger != customLogger {
		t.Error("Expected custom logger to be used")
	}

	// Verify callbacks are stored (can't test execution without WebSocket upgrade)
	if h.config.OnConnect == nil || h.config.OnDisconnect == nil {
		t.Error("Expected callbacks to be stored")
	}
}

// TestNewHandler_UpgraderConfig verifies that upgrader is configured correctly
func TestNewHandler_UpgraderConfig(t *testing.T) {
	customOrigin := func(r *http.Request) bool {
		return r.Header.Get("Origin") == "https://trusted.com"
	}

	h := NewHandler(Config{
		TargetAddress:   "localhost:50051",
		ReadBufferSize:  8192,
		WriteBufferSize: 16384,
		CheckOrigin:     customOrigin,
	})

	if h.upgrader.ReadBufferSize != 8192 {
		t.Errorf("Expected upgrader ReadBufferSize 8192, got %d", h.upgrader.ReadBufferSize)
	}
	if h.upgrader.WriteBufferSize != 16384 {
		t.Errorf("Expected upgrader WriteBufferSize 16384, got %d", h.upgrader.WriteBufferSize)
	}

	// Test CheckOrigin function
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://trusted.com")
	if !h.upgrader.CheckOrigin(req) {
		t.Error("Expected CheckOrigin to allow trusted origin")
	}

	req.Header.Set("Origin", "https://untrusted.com")
	if h.upgrader.CheckOrigin(req) {
		t.Error("Expected CheckOrigin to reject untrusted origin")
	}
}

// TestDefaultLogger_Printf verifies that the default logger works
func TestDefaultLogger_Printf(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	logger := defaultLogger{}
	logger.Printf("test message: %s", "hello")

	output := buf.String()
	if !strings.Contains(output, "test message: hello") {
		t.Errorf("Expected log output to contain 'test message: hello', got: %s", output)
	}
}

// TestServeHTTP_NonWebSocket verifies that non-WebSocket requests are rejected
func TestServeHTTP_NonWebSocket(t *testing.T) {
	h := NewHandler(Config{
		TargetAddress: "localhost:50051",
	})

	// Create a regular HTTP request (not a WebSocket upgrade)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// WebSocket upgrade should fail for non-WebSocket requests
	// The upgrader will return an error which is logged
	if w.Code == http.StatusOK {
		t.Error("Expected non-WebSocket request to fail upgrade")
	}
}

// TestServeHTTP_OriginCheck verifies that origin checking works
func TestServeHTTP_OriginCheck(t *testing.T) {
	h := NewHandler(Config{
		TargetAddress: "localhost:50051",
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == "https://allowed.com"
		},
	})

	// Create a WebSocket upgrade request with wrong origin
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Origin", "https://blocked.com")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Should fail due to origin check
	if w.Code == http.StatusSwitchingProtocols {
		t.Error("Expected origin check to reject request")
	}
}

// testLogger is a test implementation of Logger
type testLogger struct {
	messages []string
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	// Store messages for testing
	l.messages = append(l.messages, format)
}

// TestCustomLogger verifies that custom logger is used
func TestCustomLogger(t *testing.T) {
	logger := &testLogger{}

	h := NewHandler(Config{
		TargetAddress: "localhost:50051",
		Logger:        logger,
	})

	// Trigger a log by attempting WebSocket upgrade on regular HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// Should have logged the upgrade failure
	if len(logger.messages) == 0 {
		t.Error("Expected custom logger to be called")
	}

	found := false
	for _, msg := range logger.messages {
		if strings.Contains(msg, "WebSocket upgrade failed") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected logger to log WebSocket upgrade failure")
	}
}

// TestNewHandler_ProxyConfiguration verifies that reverse proxy is configured
func TestNewHandler_ProxyConfiguration(t *testing.T) {
	h := NewHandler(Config{
		TargetAddress: "localhost:9999",
	})

	if h.proxy == nil {
		t.Fatal("Expected proxy to be initialized")
	}

	if h.proxy.Transport == nil {
		t.Error("Expected proxy transport to be configured")
	}

	if h.proxy.Director == nil {
		t.Error("Expected proxy director to be configured")
	}

	if h.proxy.ErrorHandler == nil {
		t.Error("Expected proxy error handler to be configured")
	}
}

// TestDefaultCheckOrigin verifies that default CheckOrigin allows all origins
func TestDefaultCheckOrigin(t *testing.T) {
	h := NewHandler(Config{
		TargetAddress: "localhost:50051",
		// No CheckOrigin specified - should default to allow all
	})

	testCases := []string{
		"https://example.com",
		"http://localhost:3000",
		"https://untrusted.com",
		"",
	}

	for _, origin := range testCases {
		req := httptest.NewRequest("GET", "/", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}

		if !h.config.CheckOrigin(req) {
			t.Errorf("Default CheckOrigin should allow all origins, rejected: %s", origin)
		}
	}
}
