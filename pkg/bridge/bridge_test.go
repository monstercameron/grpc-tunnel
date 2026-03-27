package bridge

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	if parseH.config.CheckOrigin != nil {
		parseT.Error("Expected CheckOrigin to use websocket default policy when nil")
	}
	if parseH.config.BackendDialTimeout != parseDefaultBackendDialTimeout {
		parseT.Errorf("Expected BackendDialTimeout %s, got %s", parseDefaultBackendDialTimeout, parseH.config.BackendDialTimeout)
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
		TargetAddress:      "localhost:9999",
		ReadBufferSize:     8192,
		WriteBufferSize:    16384,
		BackendDialTimeout: 2 * time.Second,
		CheckOrigin:        parseCustomOrigin,
		Logger:             parseCustomLogger,
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
	if parseH.config.BackendDialTimeout != 2*time.Second {
		parseT.Errorf("Expected BackendDialTimeout 2s, got %s", parseH.config.BackendDialTimeout)
	}
	if parseH.logger != parseCustomLogger {
		parseT.Error("Expected custom logger to be used")
	}

	// Verify callbacks are stored (can't test execution without WebSocket upgrade)
	if parseH.config.OnConnect == nil || parseH.config.OnDisconnect == nil {
		parseT.Error("Expected callbacks to be stored")
	}
}

// TestNewHandler_InvalidTargetGuard verifies invalid target config returns HTTP errors instead of panicking.
func TestNewHandler_InvalidTargetGuard(parseT *testing.T) {
	parseLogger := &testLogger{}
	parseH := NewHandler(Config{
		TargetAddress: "%",
		Logger:        parseLogger,
	})
	if parseH.initErr == nil {
		parseT.Fatal("Expected handler initialization error for invalid target")
	}

	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)

	if parseW.Code != http.StatusInternalServerError {
		parseT.Fatalf("Expected status %d, got %d", http.StatusInternalServerError, parseW.Code)
	}
	if !strings.Contains(parseW.Body.String(), "invalid target address") {
		parseT.Fatalf("Expected invalid target response body, got %q", parseW.Body.String())
	}

	isFoundConfigWarning := false
	isFoundRequestReject := false
	for _, parseMsg := range parseLogger.messages {
		if strings.Contains(parseMsg, "Bridge configuration warning") {
			isFoundConfigWarning = true
		}
		if strings.Contains(parseMsg, "Bridge request rejected due to configuration error") {
			isFoundRequestReject = true
		}
	}
	if !isFoundConfigWarning {
		parseT.Fatal("Expected config warning log for invalid target address")
	}
	if !isFoundRequestReject {
		parseT.Fatal("Expected request rejection log for invalid target address")
	}
}

// TestNewHandler_RequireLoopbackBackendRejectsNonLoopback verifies strict backend policy rejects non-loopback plaintext targets.
func TestNewHandler_RequireLoopbackBackendRejectsNonLoopback(parseT *testing.T) {
	parseLogger := &testLogger{}
	parseH := NewHandler(Config{
		TargetAddress:                "203.0.113.10:50051",
		ShouldRequireLoopbackBackend: true,
		Logger:                       parseLogger,
	})
	if parseH.initErr == nil {
		parseT.Fatal("Expected handler initialization error for non-loopback backend target")
	}
	if !strings.Contains(parseH.initErr.Error(), "violates backend transport policy") {
		parseT.Fatalf("initErr = %v, want backend transport policy violation", parseH.initErr)
	}

	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)

	if parseW.Code != http.StatusInternalServerError {
		parseT.Fatalf("Expected status %d, got %d", http.StatusInternalServerError, parseW.Code)
	}
	if !strings.Contains(parseW.Body.String(), "violates backend transport policy") {
		parseT.Fatalf("Expected policy-violation response body, got %q", parseW.Body.String())
	}

	isFoundPolicyLog := false
	for _, parseMsg := range parseLogger.messages {
		if strings.Contains(parseMsg, "backend_transport_policy_violation") {
			isFoundPolicyLog = true
			break
		}
	}
	if !isFoundPolicyLog {
		parseT.Fatal("Expected backend transport policy violation log entry")
	}
}

// TestNewHandler_RequireLoopbackBackendAllowsLoopback verifies strict backend policy allows loopback targets.
func TestNewHandler_RequireLoopbackBackendAllowsLoopback(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress:                "127.0.0.1:50051",
		ShouldRequireLoopbackBackend: true,
	})
	if parseH.initErr != nil {
		parseT.Fatalf("Expected no init error for loopback backend target, got %v", parseH.initErr)
	}
}

// TestNewHandler_RejectsNegativeAbuseControlLimits verifies negative abuse-control limits are rejected.
func TestNewHandler_RejectsNegativeAbuseControlLimits(parseT *testing.T) {
	parseTests := []Config{
		{TargetAddress: "localhost:50051", MaxActiveConnections: -1},
		{TargetAddress: "localhost:50051", MaxConnectionsPerClient: -1},
		{TargetAddress: "localhost:50051", MaxUpgradesPerClientPerMinute: -1},
	}

	for _, parseConfig := range parseTests {
		parseHandler := NewHandler(parseConfig)
		if parseHandler.initErr == nil {
			parseT.Fatalf("NewHandler(%#v) expected init error for invalid abuse-control limit", parseConfig)
		}
	}
}

// TestServeHTTP_RejectsUpgradeWhenRateLimitExceeded verifies bridge handler returns 429 when per-client upgrade rate is exceeded.
func TestServeHTTP_RejectsUpgradeWhenRateLimitExceeded(parseT *testing.T) {
	parseHandler := NewHandler(Config{
		TargetAddress:                 "localhost:50051",
		MaxUpgradesPerClientPerMinute: 1,
	})

	parseReqOne := httptest.NewRequest(http.MethodGet, "/", nil)
	parseReqOne.RemoteAddr = "203.0.113.50:51000"
	parseWOne := httptest.NewRecorder()
	parseHandler.ServeHTTP(parseWOne, parseReqOne)

	parseReqTwo := httptest.NewRequest(http.MethodGet, "/", nil)
	parseReqTwo.RemoteAddr = "203.0.113.50:51001"
	parseWTwo := httptest.NewRecorder()
	parseHandler.ServeHTTP(parseWTwo, parseReqTwo)

	if parseWTwo.Code != http.StatusTooManyRequests {
		parseT.Fatalf("second ServeHTTP() status = %d, want %d", parseWTwo.Code, http.StatusTooManyRequests)
	}
}

// TestNewHandler_NegativeBackendDialTimeoutGuard verifies invalid backend dial timeout config is rejected.
func TestNewHandler_NegativeBackendDialTimeoutGuard(parseT *testing.T) {
	parseLogger := &testLogger{}
	parseH := NewHandler(Config{
		TargetAddress:      "localhost:50051",
		BackendDialTimeout: -time.Second,
		Logger:             parseLogger,
	})
	if parseH.initErr == nil {
		parseT.Fatal("Expected handler initialization error for negative BackendDialTimeout")
	}

	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)

	if parseW.Code != http.StatusInternalServerError {
		parseT.Fatalf("Expected status %d, got %d", http.StatusInternalServerError, parseW.Code)
	}
	if !strings.Contains(parseW.Body.String(), "BackendDialTimeout must be >= 0") {
		parseT.Fatalf("Expected BackendDialTimeout response body, got %q", parseW.Body.String())
	}

	isFoundConfigWarning := false
	isFoundRequestReject := false
	for _, parseMsg := range parseLogger.messages {
		if strings.Contains(parseMsg, "Bridge configuration warning") {
			isFoundConfigWarning = true
		}
		if strings.Contains(parseMsg, "Bridge request rejected due to configuration error") {
			isFoundRequestReject = true
		}
	}
	if !isFoundConfigWarning {
		parseT.Fatal("Expected config warning log for negative BackendDialTimeout")
	}
	if !isFoundRequestReject {
		parseT.Fatal("Expected request rejection log for negative BackendDialTimeout")
	}
}

// TestNewHandler_NegativeReadBufferSizeGuard verifies invalid read buffer config is rejected.
func TestNewHandler_NegativeReadBufferSizeGuard(parseT *testing.T) {
	parseLogger := &testLogger{}
	parseH := NewHandler(Config{
		TargetAddress:  "localhost:50051",
		ReadBufferSize: -1,
		Logger:         parseLogger,
	})
	if parseH.initErr == nil {
		parseT.Fatal("Expected handler initialization error for negative ReadBufferSize")
	}

	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)

	if parseW.Code != http.StatusInternalServerError {
		parseT.Fatalf("Expected status %d, got %d", http.StatusInternalServerError, parseW.Code)
	}
	if !strings.Contains(parseW.Body.String(), "ReadBufferSize must be >= 0") {
		parseT.Fatalf("Expected ReadBufferSize response body, got %q", parseW.Body.String())
	}

	isFoundConfigWarning := false
	isFoundRequestReject := false
	for _, parseMsg := range parseLogger.messages {
		if strings.Contains(parseMsg, "Bridge configuration warning") {
			isFoundConfigWarning = true
		}
		if strings.Contains(parseMsg, "Bridge request rejected due to configuration error") {
			isFoundRequestReject = true
		}
	}
	if !isFoundConfigWarning {
		parseT.Fatal("Expected config warning log for negative ReadBufferSize")
	}
	if !isFoundRequestReject {
		parseT.Fatal("Expected request rejection log for negative ReadBufferSize")
	}
}

// TestNewHandler_NegativeWriteBufferSizeGuard verifies invalid write buffer config is rejected.
func TestNewHandler_NegativeWriteBufferSizeGuard(parseT *testing.T) {
	parseLogger := &testLogger{}
	parseH := NewHandler(Config{
		TargetAddress:   "localhost:50051",
		WriteBufferSize: -1,
		Logger:          parseLogger,
	})
	if parseH.initErr == nil {
		parseT.Fatal("Expected handler initialization error for negative WriteBufferSize")
	}

	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)

	if parseW.Code != http.StatusInternalServerError {
		parseT.Fatalf("Expected status %d, got %d", http.StatusInternalServerError, parseW.Code)
	}
	if !strings.Contains(parseW.Body.String(), "WriteBufferSize must be >= 0") {
		parseT.Fatalf("Expected WriteBufferSize response body, got %q", parseW.Body.String())
	}

	isFoundConfigWarning := false
	isFoundRequestReject := false
	for _, parseMsg := range parseLogger.messages {
		if strings.Contains(parseMsg, "Bridge configuration warning") {
			isFoundConfigWarning = true
		}
		if strings.Contains(parseMsg, "Bridge request rejected due to configuration error") {
			isFoundRequestReject = true
		}
	}
	if !isFoundConfigWarning {
		parseT.Fatal("Expected config warning log for negative WriteBufferSize")
	}
	if !isFoundRequestReject {
		parseT.Fatal("Expected request rejection log for negative WriteBufferSize")
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
	parseL.messages = append(parseL.messages, fmt.Sprintf(format, parseV...))
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

// TestDefaultCheckOrigin verifies that default CheckOrigin uses websocket same-origin behavior.
func TestDefaultCheckOrigin(parseT *testing.T) {
	parseH := NewHandler(Config{
		TargetAddress: "localhost:50051",
	})

	if parseH.config.CheckOrigin != nil {
		parseT.Fatal("Expected nil CheckOrigin so websocket default same-origin checks apply")
	}

	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseReq.Header.Set("Connection", "Upgrade")
	parseReq.Header.Set("Upgrade", "websocket")
	parseReq.Header.Set("Sec-WebSocket-Version", "13")
	parseReq.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	parseReq.Header.Set("Origin", "https://untrusted.example")

	parseW := httptest.NewRecorder()
	parseH.ServeHTTP(parseW, parseReq)
	if parseW.Code == http.StatusSwitchingProtocols {
		parseT.Fatal("Expected websocket default same-origin check to reject cross-origin upgrade")
	}
}
