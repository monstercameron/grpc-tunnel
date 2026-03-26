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
func TestNewHandler_Defaults(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress: "localhost:50051",
	})

	if parseH.config.ReadBufferSize != 4096 {
		parseT.Errorf("Expected ReadBufferSize 4096, got %d", parseH.config.ReadBufferSize)
	}
	if parseH.config.WriteBufferSize != 4096 {
		parseT.Errorf("Expected WriteBufferSize 4096, got %d", parseH.config.WriteBufferSize)
	}
	if parseH.config.CheckOrigin == nil {
		parseT.Error("Expected CheckOrigin to be set to default")
	}
	if parseH.logger == nil {
		parseT.Error("Expected logger to be set")
	}
}

// TestNewHandler_CustomConfig verifies that NewHandler respects custom configuration
func TestNewHandler_CustomConfig(parseT *testing.T) {
	parseCustomOrigin := func(parseR *http.Request) bool { return false }
	parseCustomLogger := &testLogger{}

	parseH := NewHandler(Config{
		TargetAddress:   "localhost:9999",
		ReadBufferSize:  8192,
		WriteBufferSize: 16384,
		CheckOrigin:     parseCustomOrigin,
		Logger:          parseCustomLogger,
		OnConnect: func(parseR2 *http.Request) {
			// Connect callback
		},
		OnDisconnect: func(parseR3 *http.Request) {
			// Disconnect callback
		},
	})

	if parseH.config.ReadBufferSize != 8192 {
		parseT.Errorf("Expected ReadBufferSize 8192, got %d", parseH.config.ReadBufferSize)
	}
	if parseH.config.WriteBufferSize != 16384 {
		parseT.Errorf("Expected WriteBufferSize 16384, got %d", parseH.config.WriteBufferSize)
	}
	if parseH.logger != parseCustomLogger {
		parseT.Error("Expected custom logger to be used")
	}

	// Verify callbacks are stored (can't test execution without WebSocket upgrade)
	if parseH.config.OnConnect == nil || parseH.config.OnDisconnect == nil {
		parseT.Error("Expected callbacks to be stored")
	}
}

// TestNewHandler_UpgraderConfig verifies that upgrader is configured correctly
func TestNewHandler_UpgraderConfig(parseT *testing.T) {
	parseCustomOrigin := func(parseR *http.Request) bool {
		return parseR.Header.Get("Origin") == "https://trusted.com"
	}

	parseH := NewHandler(Config{
		TargetAddress:   "localhost:50051",
		ReadBufferSize:  8192,
		WriteBufferSize: 16384,
		CheckOrigin:     parseCustomOrigin,
	})

	if parseH.upgrader.ReadBufferSize != 8192 {
		parseT.Errorf("Expected upgrader ReadBufferSize 8192, got %d", parseH.upgrader.ReadBufferSize)
	}
	if parseH.upgrader.WriteBufferSize != 16384 {
		parseT.Errorf("Expected upgrader WriteBufferSize 16384, got %d", parseH.upgrader.WriteBufferSize)
	}

	// Test CheckOrigin function
	parseReq := httptest.NewRequest("GET", "/", nil)
	parseReq.Header.Set("Origin", "https://trusted.com")
	if !parseH.upgrader.CheckOrigin(parseReq) {
		parseT.Error("Expected CheckOrigin to allow trusted origin")
	}

	parseReq.Header.Set("Origin", "https://untrusted.com")
	if parseH.upgrader.CheckOrigin(parseReq) {
		parseT.Error("Expected CheckOrigin to reject untrusted origin")
	}
}

// TestDefaultLogger_Printf verifies that the default logger works
func TestDefaultLogger_Printf(parseT *testing.T) {
	var parseBuf bytes.Buffer
	log.SetOutput(&parseBuf)
	defer log.SetOutput(log.Writer())

	parseLogger := defaultLogger{}
	parseLogger.Printf("test message: %s", "hello")

	parseOutput := parseBuf.String()
	if !strings.Contains(parseOutput, "test message: hello") {
		parseT.Errorf("Expected log output to contain 'test message: hello', got: %s", parseOutput)
	}
}

// TestServeHTTP_NonWebSocket verifies that non-WebSocket requests are rejected
func TestServeHTTP_NonWebSocket(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress: "localhost:50051",
	})

	// Create a regular HTTP request (not a WebSocket upgrade)
	parseReq := httptest.NewRequest("GET", "/", nil)
	parseW := httptest.NewRecorder()

	parseH.ServeHTTP(parseW, parseReq)

	// WebSocket upgrade should fail for non-WebSocket requests
	// The upgrader will return an error which is logged
	if parseW.Code == http.StatusOK {
		parseT.Error("Expected non-WebSocket request to fail upgrade")
	}
}

// TestServeHTTP_OriginCheck verifies that origin checking works
func TestServeHTTP_OriginCheck(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress: "localhost:50051",
		CheckOrigin: func(parseR *http.Request) bool {
			return parseR.Header.Get("Origin") == "https://allowed.com"
		},
	})

	// Create a WebSocket upgrade request with wrong origin
	parseReq := httptest.NewRequest("GET", "/", nil)
	parseReq.Header.Set("Connection", "Upgrade")
	parseReq.Header.Set("Upgrade", "websocket")
	parseReq.Header.Set("Sec-WebSocket-Version", "13")
	parseReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	parseReq.Header.Set("Origin", "https://blocked.com")

	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)

	// Should fail due to origin check
	if parseW.Code == http.StatusSwitchingProtocols {
		parseT.Error("Expected origin check to reject request")
	}
}

// testLogger is a test implementation of Logger
type testLogger struct {
	messages []string
}

func (parseL *testLogger) Printf(format string, parseV ...interface{}) {
	// Store messages for testing
	parseL.messages = append(parseL.messages, format)
}

// TestCustomLogger verifies that custom logger is used
func TestCustomLogger(parseT *testing.T) {
	parseLogger := &testLogger{}

	parseH := NewHandler(Config{
		TargetAddress: "localhost:50051",
		Logger:        parseLogger,
	})

	// Trigger a log by attempting WebSocket upgrade on regular HTTP request
	parseReq := httptest.NewRequest("GET", "/", nil)
	parseW := httptest.NewRecorder()

	parseH.ServeHTTP(parseW, parseReq)

	// Should have logged the upgrade failure
	if len(parseLogger.messages) == 0 {
		parseT.Error("Expected custom logger to be called")
	}

	isFound := false
	for _, parseMsg := range parseLogger.messages {
		if strings.Contains(parseMsg, "WebSocket upgrade failed") {
			isFound = true
			break
		}
	}
	if !isFound {
		parseT.Error("Expected logger to log WebSocket upgrade failure")
	}
}

// TestNewHandler_ProxyConfiguration verifies that reverse proxy is configured
func TestNewHandler_ProxyConfiguration(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress: "localhost:9999",
	})

	if parseH.proxy == nil {
		parseT.Fatal("Expected proxy to be initialized")
	}

	if parseH.proxy.Transport == nil {
		parseT.Error("Expected proxy transport to be configured")
	}

	if parseH.proxy.Director == nil {
		parseT.Error("Expected proxy director to be configured")
	}

	if parseH.proxy.ErrorHandler == nil {
		parseT.Error("Expected proxy error handler to be configured")
	}
}

// TestDefaultCheckOrigin verifies that default CheckOrigin allows all origins
func TestDefaultCheckOrigin(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress: "localhost:50051",
		// No CheckOrigin specified - should default to allow all
	})

	parseTestCases := []string{
		"https://example.com",
		"http://localhost:3000",
		"https://untrusted.com",
		"",
	}

	for _, parseOrigin := range parseTestCases {
		parseReq := httptest.NewRequest("GET", "/", nil)
		if parseOrigin != "" {
			parseReq.Header.Set("Origin", parseOrigin)
		}

		if !parseH.config.CheckOrigin(parseReq) {
			parseT.Errorf("Default CheckOrigin should allow all origins, rejected: %s", parseOrigin)
		}
	}
}
