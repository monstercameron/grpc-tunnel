package bridge

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// getBridgeTestConn opens a live WebSocket connection and wraps it with NewWebSocketConn.
func getBridgeTestConn(parseT *testing.T, handleServer func(*websocket.Conn)) (*webSocketConn, func()) {
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

	parseBridgeConn := NewWebSocketConn(parseClientSocket).(*webSocketConn)
	return parseBridgeConn, func() {
		_ = parseBridgeConn.Close()
		parseServer.Close()
	}
}

// TestHandleBridgeConnReadFrames verifies binary frames stream correctly and empty frames are skipped.
func TestHandleBridgeConnReadFrames(parseT *testing.T) {
	parseBridgeConn, clearConn := getBridgeTestConn(parseT, func(parseServerSocket *websocket.Conn) {
		_ = parseServerSocket.WriteMessage(websocket.BinaryMessage, []byte("hello"))
		_ = parseServerSocket.WriteMessage(websocket.BinaryMessage, []byte{})
		_ = parseServerSocket.WriteMessage(websocket.BinaryMessage, []byte("world"))
		time.Sleep(20 * time.Millisecond)
	})
	defer clearConn()

	parseBuffer := make([]byte, 5)

	parseBytesRead, parseErr := parseBridgeConn.Read(parseBuffer)
	if parseErr != nil {
		parseT.Fatalf("Read() first error: %v", parseErr)
	}
	if string(parseBuffer[:parseBytesRead]) != "hello" {
		parseT.Fatalf("Read() first payload = %q, want %q", string(parseBuffer[:parseBytesRead]), "hello")
	}

	parseBytesRead, parseErr = parseBridgeConn.Read(parseBuffer)
	if parseErr != nil {
		parseT.Fatalf("Read() second error: %v", parseErr)
	}
	if string(parseBuffer[:parseBytesRead]) != "world" {
		parseT.Fatalf("Read() second payload = %q, want %q", string(parseBuffer[:parseBytesRead]), "world")
	}
}

// TestHandleBridgeConnRejectsTextFrames verifies non-binary frames terminate the stream.
func TestHandleBridgeConnRejectsTextFrames(parseT *testing.T) {
	parseBridgeConn, clearConn := getBridgeTestConn(parseT, func(parseServerSocket *websocket.Conn) {
		_ = parseServerSocket.WriteMessage(websocket.TextMessage, []byte("text-frame"))
		time.Sleep(20 * time.Millisecond)
	})
	defer clearConn()

	_, parseErr := parseBridgeConn.Read(make([]byte, 16))
	if !errors.Is(parseErr, net.ErrClosed) {
		parseT.Fatalf("Read() error = %v, want %v", parseErr, net.ErrClosed)
	}
}

// TestHandleBridgeConnTracksAddressesAndDeadlines verifies live connections expose addresses and deadline setters.
func TestHandleBridgeConnTracksAddressesAndDeadlines(parseT *testing.T) {
	parseBridgeConn, clearConn := getBridgeTestConn(parseT, func(parseServerSocket *websocket.Conn) {
		time.Sleep(50 * time.Millisecond)
	})
	defer clearConn()

	if parseBridgeConn.LocalAddr() == nil {
		parseT.Fatal("LocalAddr() returned nil")
	}
	if parseBridgeConn.RemoteAddr() == nil {
		parseT.Fatal("RemoteAddr() returned nil")
	}

	parseDeadline := time.Now().Add(250 * time.Millisecond)
	if parseErr := parseBridgeConn.SetDeadline(parseDeadline); parseErr != nil {
		parseT.Fatalf("SetDeadline() error: %v", parseErr)
	}
	if !parseBridgeConn.readDeadline.Equal(parseDeadline) || !parseBridgeConn.writeDeadline.Equal(parseDeadline) {
		parseT.Fatal("SetDeadline() did not store both deadlines")
	}

	parseReadDeadline := parseDeadline.Add(250 * time.Millisecond)
	if parseErr := parseBridgeConn.SetReadDeadline(parseReadDeadline); parseErr != nil {
		parseT.Fatalf("SetReadDeadline() error: %v", parseErr)
	}
	if !parseBridgeConn.readDeadline.Equal(parseReadDeadline) {
		parseT.Fatal("SetReadDeadline() did not store read deadline")
	}

	parseWriteDeadline := parseReadDeadline.Add(250 * time.Millisecond)
	if parseErr := parseBridgeConn.SetWriteDeadline(parseWriteDeadline); parseErr != nil {
		parseT.Fatalf("SetWriteDeadline() error: %v", parseErr)
	}
	if !parseBridgeConn.writeDeadline.Equal(parseWriteDeadline) {
		parseT.Fatal("SetWriteDeadline() did not store write deadline")
	}
}

// TestHandleBridgeConnRejectsIOAfterClose verifies reads and writes fail after Close.
func TestHandleBridgeConnRejectsIOAfterClose(parseT *testing.T) {
	parseBridgeConn, clearConn := getBridgeTestConn(parseT, func(parseServerSocket *websocket.Conn) {
		time.Sleep(50 * time.Millisecond)
	})
	defer clearConn()

	if parseErr := parseBridgeConn.Close(); parseErr != nil {
		parseT.Fatalf("Close() error: %v", parseErr)
	}

	_, parseReadErr := parseBridgeConn.Read(make([]byte, 1))
	if !errors.Is(parseReadErr, net.ErrClosed) {
		parseT.Fatalf("Read() after close error = %v, want %v", parseReadErr, net.ErrClosed)
	}

	_, parseWriteErr := parseBridgeConn.Write([]byte("x"))
	if !errors.Is(parseWriteErr, net.ErrClosed) {
		parseT.Fatalf("Write() after close error = %v, want %v", parseWriteErr, net.ErrClosed)
	}
}
