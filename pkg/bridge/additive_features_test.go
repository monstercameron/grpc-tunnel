//go:build !js && !wasm

//lint:file-ignore SA1019 grpc.DialContext and WithInsecure/WithBlock are retained in tests to validate legacy dial options on grpc 1.x.

package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

func TestDialOptionWithConfig_HandshakeOptions(parseT *testing.T) {
	parseHandshakeChannel := make(chan struct{}, 1)
	var parseHeaderValue string
	var parseExtensionValue string
	var parseSubprotocolValue string
	isProxyCalled := false

	parseUpgrader := websocket.Upgrader{
		CheckOrigin:       func(parseR *http.Request) bool { return true },
		EnableCompression: true,
		Subprotocols:      []string{"proto.v1"},
	}

	parseServer := httptest.NewServer(http.HandlerFunc(func(parseW http.ResponseWriter, parseR *http.Request) {
		parseHeaderValue = parseR.Header.Get("X-Bridge-Test")
		parseExtensionValue = parseR.Header.Get("Sec-WebSocket-Extensions")

		parseSocket, parseErr := parseUpgrader.Upgrade(parseW, parseR, nil)
		if parseErr != nil {
			return
		}
		defer parseSocket.Close()

		parseSubprotocolValue = parseSocket.Subprotocol()
		select {
		case parseHandshakeChannel <- struct{}{}:
		default:
		}
		time.Sleep(100 * time.Millisecond)
	}))
	defer parseServer.Close()

	parseWebSocketURL := "ws" + strings.TrimPrefix(parseServer.URL, "http")
	parseDialContext, clearDial := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer clearDial()

	_, _ = grpc.DialContext(
		parseDialContext,
		"ignored:1234",
		DialOptionWithConfig(parseWebSocketURL, ClientConfig{
			Headers: http.Header{
				"X-Bridge-Test": []string{"present"},
			},
			Subprotocols:            []string{"proto.v1"},
			HandshakeTimeout:        100 * time.Millisecond,
			ShouldEnableCompression: true,
			Proxy: func(parseRequest *http.Request) (*url.URL, error) {
				isProxyCalled = true
				return nil, nil
			},
		}),
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)

	select {
	case <-parseHandshakeChannel:
	case <-time.After(time.Second):
		parseT.Fatal("expected websocket handshake")
	}

	if !isProxyCalled {
		parseT.Fatal("expected proxy selector to be called")
	}
	if parseHeaderValue != "present" {
		parseT.Fatalf("X-Bridge-Test header = %q, want %q", parseHeaderValue, "present")
	}
	if parseSubprotocolValue != "proto.v1" {
		parseT.Fatalf("websocket subprotocol = %q, want %q", parseSubprotocolValue, "proto.v1")
	}
	if !strings.Contains(parseExtensionValue, "permessage-deflate") {
		parseT.Fatalf("Sec-WebSocket-Extensions = %q, want compression negotiation", parseExtensionValue)
	}
}

func TestGetHandlerConfigError_Validation(parseT *testing.T) {
	parseTests := []struct {
		parseName   string
		parseConfig Config
	}{
		{
			parseName: "negative read limit",
			parseConfig: Config{
				ReadLimitBytes: -1,
			},
		},
		{
			parseName: "negative ping interval",
			parseConfig: Config{
				PingInterval: -time.Second,
			},
		},
		{
			parseName: "negative idle timeout",
			parseConfig: Config{
				IdleTimeout: -time.Second,
			},
		},
		{
			parseName: "idle timeout without ping interval",
			parseConfig: Config{
				IdleTimeout: time.Second,
			},
		},
		{
			parseName: "ping interval not less than idle timeout",
			parseConfig: Config{
				PingInterval: time.Second,
				IdleTimeout:  time.Second,
			},
		},
		{
			parseName: "read limit with disable flag",
			parseConfig: Config{
				ReadLimitBytes:         1024,
				ShouldDisableReadLimit: true,
			},
		},
	}

	for _, parseTestCase := range parseTests {
		parseT.Run(parseTestCase.parseName, func(parseT2 *testing.T) {
			parseErr := getHandlerConfigError(parseTestCase.parseConfig)
			if parseErr == nil {
				parseT2.Fatal("getHandlerConfigError() expected validation error, got nil")
			}
		})
	}
}

func TestGetHandlerReadLimitBytes(parseT *testing.T) {
	parseTests := []struct {
		parseName     string
		parseConfig   Config
		parseExpected int64
	}{
		{
			parseName: "default secure limit",
			parseConfig: Config{
				ReadLimitBytes: 0,
			},
			parseExpected: parseDefaultReadLimitBytes,
		},
		{
			parseName: "custom read limit",
			parseConfig: Config{
				ReadLimitBytes: 1024,
			},
			parseExpected: 1024,
		},
		{
			parseName: "read limit disabled",
			parseConfig: Config{
				ShouldDisableReadLimit: true,
			},
			parseExpected: 0,
		},
	}

	for _, parseTestCase := range parseTests {
		parseT.Run(parseTestCase.parseName, func(parseT2 *testing.T) {
			parseResult := getHandlerReadLimitBytes(parseTestCase.parseConfig)
			if parseResult != parseTestCase.parseExpected {
				parseT2.Fatalf("getHandlerReadLimitBytes() = %d, want %d", parseResult, parseTestCase.parseExpected)
			}
		})
	}
}

func TestNewHandler_KeepaliveAndCompression(parseT *testing.T) {
	parseHandler := NewHandler(Config{
		TargetAddress:           "localhost:50051",
		PingInterval:            25 * time.Millisecond,
		IdleTimeout:             100 * time.Millisecond,
		ShouldEnableCompression: true,
	})

	parseServer := httptest.NewServer(parseHandler)
	defer parseServer.Close()

	parseWebSocketURL := "ws" + strings.TrimPrefix(parseServer.URL, "http")
	parseDialer := websocket.Dialer{
		EnableCompression: true,
	}
	parsePingChannel := make(chan struct{}, 1)
	parseClientSocket, parseResponse, parseErr := parseDialer.Dial(parseWebSocketURL, nil)
	if parseErr != nil {
		parseT.Fatalf("Dial() error: %v", parseErr)
	}
	defer parseClientSocket.Close()

	parseClientSocket.SetPingHandler(func(parseApplicationData string) error {
		select {
		case parsePingChannel <- struct{}{}:
		default:
		}
		return parseClientSocket.WriteControl(websocket.PongMessage, []byte(parseApplicationData), time.Now().Add(time.Second))
	})
	go func() {
		for {
			if _, _, parseErr := parseClientSocket.NextReader(); parseErr != nil {
				return
			}
		}
	}()

	select {
	case <-parsePingChannel:
	case <-time.After(time.Second):
		parseT.Fatal("expected handler keepalive ping")
	}

	if !strings.Contains(parseResponse.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate") {
		parseT.Fatalf("Sec-WebSocket-Extensions = %q, want compression negotiation", parseResponse.Header.Get("Sec-WebSocket-Extensions"))
	}
}

func TestNewHandler_InvalidKeepaliveConfig(parseT *testing.T) {
	parseHandler := NewHandler(Config{
		TargetAddress: "localhost:50051",
		IdleTimeout:   time.Second,
	})

	if parseHandler.initErr == nil {
		parseT.Fatal("expected invalid keepalive configuration error")
	}
	if !strings.Contains(parseHandler.initErr.Error(), "PingInterval") {
		parseT.Fatalf("initErr = %v, want PingInterval validation", parseHandler.initErr)
	}
}

func TestBuildHandlerPingWriteTimeout(parseT *testing.T) {
	parseTests := []struct {
		parseName     string
		parseConfig   Config
		parseExpected time.Duration
	}{
		{
			parseName: "use ping interval",
			parseConfig: Config{
				PingInterval: 25 * time.Millisecond,
				IdleTimeout:  100 * time.Millisecond,
			},
			parseExpected: 25 * time.Millisecond,
		},
		{
			parseName: "cap by idle timeout",
			parseConfig: Config{
				PingInterval: 2 * time.Second,
				IdleTimeout:  time.Second,
			},
			parseExpected: time.Second,
		},
		{
			parseName:     "fallback to one second",
			parseConfig:   Config{},
			parseExpected: time.Second,
		},
	}

	for _, parseTestCase := range parseTests {
		parseT.Run(parseTestCase.parseName, func(parseT2 *testing.T) {
			parseResult := buildHandlerPingWriteTimeout(parseTestCase.parseConfig)
			if parseResult != parseTestCase.parseExpected {
				parseT2.Fatalf("buildHandlerPingWriteTimeout() = %v, want %v", parseResult, parseTestCase.parseExpected)
			}
		})
	}
}

func TestBuildWebSocketWriteBufferPool_CacheableBufferSize(parseT *testing.T) {
	parsePoolOne := buildWebSocketWriteBufferPool(parseDefaultWebSocketBufferSize)
	parsePoolTwo := buildWebSocketWriteBufferPool(parseDefaultWebSocketBufferSize)
	if parsePoolOne != parsePoolTwo {
		parseT.Fatal("buildWebSocketWriteBufferPool() expected shared pool for cacheable size")
	}
}

func TestBuildWebSocketWriteBufferPool_UncacheableBufferSize(parseT *testing.T) {
	parsePoolOne := buildWebSocketWriteBufferPool(12345)
	parsePoolTwo := buildWebSocketWriteBufferPool(12345)
	if parsePoolOne == parsePoolTwo {
		parseT.Fatal("buildWebSocketWriteBufferPool() expected distinct pools for uncacheable size")
	}
}
