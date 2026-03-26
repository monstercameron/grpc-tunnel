//go:build !js && !wasm

package grpctunnel

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// webSocketConn adapts a WebSocket connection to net.Conn interface.
// This is needed because gRPC expects a net.Conn but browsers only have WebSocket.
type webSocketConn struct {
	ws         *websocket.Conn
	reader     io.Reader
	closeOnce  sync.Once
	closed     bool
	closedMu   sync.RWMutex
	deadlineMu sync.Mutex // Protects deadline operations
}

func newWebSocketConn(parseWs *websocket.Conn) net.Conn {
	return &webSocketConn{ws: parseWs}
}

func (parseC *webSocketConn) Read(parseP []byte) (int, error) {
	parseC.closedMu.RLock()
	if parseC.closed {
		parseC.closedMu.RUnlock()
		return 0, io.EOF
	}
	parseC.closedMu.RUnlock()

	if parseC.reader == nil {
		parseMessageType, parseReader, parseErr := parseC.ws.NextReader()
		if parseErr != nil {
			return 0, parseErr
		}
		if parseMessageType != websocket.BinaryMessage {
			return 0, io.EOF
		}
		parseC.reader = parseReader
	}

	parseN, parseErr2 := parseC.reader.Read(parseP)
	if parseErr2 == io.EOF {
		parseC.reader = nil
		parseErr2 = nil
	}
	return parseN, parseErr2
}

func (parseC *webSocketConn) Write(parseP []byte) (int, error) {
	parseC.closedMu.RLock()
	if parseC.closed {
		parseC.closedMu.RUnlock()
		return 0, io.ErrClosedPipe
	}
	parseC.closedMu.RUnlock()

	if parseErr := parseC.ws.WriteMessage(websocket.BinaryMessage, parseP); parseErr != nil {
		return 0, parseErr
	}
	return len(parseP), nil
}

func (parseC *webSocketConn) Close() error {
	var parseErr error
	parseC.closeOnce.Do(func() {
		parseC.closedMu.Lock()
		parseC.closed = true
		parseC.closedMu.Unlock()
		parseErr = parseC.ws.Close()
	})
	return parseErr
}

func (parseC *webSocketConn) LocalAddr() net.Addr {
	return parseC.ws.LocalAddr()
}

func (parseC *webSocketConn) RemoteAddr() net.Addr {
	return parseC.ws.RemoteAddr()
}

func (parseC *webSocketConn) SetDeadline(parseT time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()
	if parseErr := parseC.ws.SetReadDeadline(parseT); parseErr != nil {
		return parseErr
	}
	return parseC.ws.SetWriteDeadline(parseT)
}

func (parseC *webSocketConn) SetReadDeadline(parseT time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()
	return parseC.ws.SetReadDeadline(parseT)
}

func (parseC *webSocketConn) SetWriteDeadline(parseT time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()
	return parseC.ws.SetWriteDeadline(parseT)
}
