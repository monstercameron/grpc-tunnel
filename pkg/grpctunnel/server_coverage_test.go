//go:build !js && !wasm

package grpctunnel

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// getGrpctunnelTestConn opens a live WebSocket pair for grpctunnel conn coverage tests.
func getGrpctunnelTestConn(parseT *testing.T, handleServer func(*websocket.Conn)) (*webSocketConn, func()) {
	parseT.Helper()

	parseUpgrader := websocket.Upgrader{
		CheckOrigin: func(parseR *http.Request) bool { return true },
	}

	parseServer := httptest.NewServer(http.HandlerFunc(func(parseW http.ResponseWriter, parseR *http.Request) {
		parseServerSocket, parseErr := parseUpgrader.Upgrade(parseW, parseR, nil)
		if parseErr != nil {
			return
		}

		go func() {
			defer parseServerSocket.Close()
			handleServer(parseServerSocket)
		}()
	}))

	parseSocketURL := "ws" + strings.TrimPrefix(parseServer.URL, "http")
	parseClientSocket, _, parseErr := websocket.DefaultDialer.Dial(parseSocketURL, nil)
	if parseErr != nil {
		parseServer.Close()
		parseT.Fatalf("Dial() error: %v", parseErr)
	}

	parseTunnelConn := newWebSocketConn(parseClientSocket).(*webSocketConn)
	return parseTunnelConn, func() {
		_ = parseTunnelConn.Close()
		parseServer.Close()
	}
}

// TestServe_ClosedListener verifies Serve returns listener errors directly.
func TestServe_ClosedListener(parseT *testing.T) {
	parseListener, parseErr := net.Listen("tcp", "127.0.0.1:0")
	if parseErr != nil {
		parseT.Fatalf("Listen() error: %v", parseErr)
	}
	_ = parseListener.Close()

	parseErr = Serve(parseListener, grpc.NewServer())
	if parseErr == nil {
		parseT.Fatal("Serve() expected error for closed listener, got nil")
	}
}

// TestListenAndServe_InvalidAddress verifies invalid listen addresses fail immediately.
func TestListenAndServe_InvalidAddress(parseT *testing.T) {
	parseErr := ListenAndServe("127.0.0.1:invalid", grpc.NewServer())
	if parseErr == nil {
		parseT.Fatal("ListenAndServe() expected address error, got nil")
	}
}

// TestWebSocketConn_DeadlineMethods verifies deadline setters execute against a live socket.
func TestWebSocketConn_DeadlineMethods(parseT *testing.T) {
	parseTunnelConn, clearConn := getGrpctunnelTestConn(parseT, func(parseServerSocket *websocket.Conn) {
		time.Sleep(50 * time.Millisecond)
	})
	defer clearConn()

	parseDeadline := time.Now().Add(250 * time.Millisecond)
	if parseErr := parseTunnelConn.SetDeadline(parseDeadline); parseErr != nil {
		parseT.Fatalf("SetDeadline() error: %v", parseErr)
	}
	if parseErr := parseTunnelConn.SetReadDeadline(parseDeadline.Add(250 * time.Millisecond)); parseErr != nil {
		parseT.Fatalf("SetReadDeadline() error: %v", parseErr)
	}
	if parseErr := parseTunnelConn.SetWriteDeadline(parseDeadline.Add(500 * time.Millisecond)); parseErr != nil {
		parseT.Fatalf("SetWriteDeadline() error: %v", parseErr)
	}
}
