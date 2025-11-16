package bridge

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// webSocketConn adapts a gorilla/websocket.Conn to implement net.Conn.
// This allows gRPC to use WebSocket as its transport.
type webSocketConn struct {
	ws            *websocket.Conn
	readBuf       []byte
	readDeadline  time.Time
	writeDeadline time.Time
}

func newWebSocketConn(ws *websocket.Conn) net.Conn {
	return &webSocketConn{ws: ws}
}

func (c *webSocketConn) Read(p []byte) (int, error) {
	// If we have buffered data, use it
	if len(c.readBuf) > 0 {
		n := copy(p, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// Read next WebSocket message
	msgType, data, err := c.ws.ReadMessage()
	if err != nil {
		return 0, err
	}

	if msgType != websocket.BinaryMessage {
		return 0, net.ErrClosed
	}

	// Copy to output buffer and buffer remainder
	n := copy(p, data)
	if n < len(data) {
		c.readBuf = data[n:]
	}
	return n, nil
}

func (c *webSocketConn) Write(p []byte) (int, error) {
	err := c.ws.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *webSocketConn) Close() error {
	return c.ws.Close()
}

func (c *webSocketConn) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

func (c *webSocketConn) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

func (c *webSocketConn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

func (c *webSocketConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *webSocketConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}
