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
	ws        *websocket.Conn
	reader    io.Reader
	closeOnce sync.Once
	closed    bool
	closedMu  sync.RWMutex
}

func newWebSocketConn(ws *websocket.Conn) net.Conn {
	return &webSocketConn{ws: ws}
}

func (c *webSocketConn) Read(p []byte) (int, error) {
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return 0, io.EOF
	}
	c.closedMu.RUnlock()

	if c.reader == nil {
		messageType, reader, err := c.ws.NextReader()
		if err != nil {
			return 0, err
		}
		if messageType != websocket.BinaryMessage {
			return 0, io.EOF
		}
		c.reader = reader
	}

	n, err := c.reader.Read(p)
	if err == io.EOF {
		c.reader = nil
		err = nil
	}
	return n, err
}

func (c *webSocketConn) Write(p []byte) (int, error) {
	c.closedMu.RLock()
	if c.closed {
		c.closedMu.RUnlock()
		return 0, io.ErrClosedPipe
	}
	c.closedMu.RUnlock()

	if err := c.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *webSocketConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.closedMu.Lock()
		c.closed = true
		c.closedMu.Unlock()
		err = c.ws.Close()
	})
	return err
}

func (c *webSocketConn) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

func (c *webSocketConn) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

func (c *webSocketConn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}

func (c *webSocketConn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

func (c *webSocketConn) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}
