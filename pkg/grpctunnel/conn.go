//go:build !js && !wasm

package grpctunnel

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// webSocketConn adapts a WebSocket connection to net.Conn interface.
// This is needed because gRPC expects a net.Conn but browsers only have WebSocket.
type webSocketConn struct {
	ws         *websocket.Conn
	reader     io.Reader
	closeOnce  sync.Once
	isClosed   atomic.Bool
	writeMu    sync.Mutex
	deadlineMu sync.Mutex // Protects deadline operations
}

func newWebSocketConn(parseWs *websocket.Conn) net.Conn {
	return &webSocketConn{ws: parseWs}
}

// Read reads binary payload bytes from the underlying WebSocket stream.
func (parseC *webSocketConn) Read(parseP []byte) (int, error) {
	if parseC.isClosed.Load() {
		return 0, io.EOF
	}
	if parseC.ws == nil {
		return 0, io.EOF
	}

	for {
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
			if parseN > 0 {
				return parseN, nil
			}
			// Empty frame: continue draining subsequent frames until payload arrives.
			continue
		}
		return parseN, parseErr2
	}
}

// Write writes one binary WebSocket message from the provided byte slice.
func (parseC *webSocketConn) Write(parseP []byte) (int, error) {
	parseC.writeMu.Lock()
	defer parseC.writeMu.Unlock()

	if parseC.isClosed.Load() {
		return 0, io.ErrClosedPipe
	}
	if parseC.ws == nil {
		return 0, io.ErrClosedPipe
	}

	if parseErr := parseC.ws.WriteMessage(websocket.BinaryMessage, parseP); parseErr != nil {
		return 0, parseErr
	}
	return len(parseP), nil
}

// Close closes the adapted connection and underlying WebSocket exactly once.
func (parseC *webSocketConn) Close() error {
	var parseErr error
	parseC.closeOnce.Do(func() {
		parseC.isClosed.Store(true)
		if parseC.ws == nil {
			return
		}
		parseErr = parseC.ws.Close()
	})
	return parseErr
}

// LocalAddr returns the local network address for this connection.
func (parseC *webSocketConn) LocalAddr() net.Addr {
	if parseC.ws == nil {
		return nil
	}
	return parseC.ws.LocalAddr()
}

// RemoteAddr returns the remote network address for this connection.
func (parseC *webSocketConn) RemoteAddr() net.Addr {
	if parseC.ws == nil {
		return nil
	}
	return parseC.ws.RemoteAddr()
}

// SetDeadline sets read and write deadlines on the underlying WebSocket.
func (parseC *webSocketConn) SetDeadline(parseT time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()
	if parseC.ws == nil {
		return net.ErrClosed
	}
	if parseErr := parseC.ws.SetReadDeadline(parseT); parseErr != nil {
		return parseErr
	}
	return parseC.ws.SetWriteDeadline(parseT)
}

// SetReadDeadline sets the read deadline on the underlying WebSocket.
func (parseC *webSocketConn) SetReadDeadline(parseT time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()
	if parseC.ws == nil {
		return net.ErrClosed
	}
	return parseC.ws.SetReadDeadline(parseT)
}

// SetWriteDeadline sets the write deadline on the underlying WebSocket.
func (parseC *webSocketConn) SetWriteDeadline(parseT time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()
	if parseC.ws == nil {
		return net.ErrClosed
	}
	return parseC.ws.SetWriteDeadline(parseT)
}
