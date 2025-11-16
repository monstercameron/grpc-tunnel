package bridge

import (
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// bufferPool reduces memory allocations by reusing byte slices
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Default buffer size matches typical gRPC frame size
		buf := make([]byte, 4096)
		return &buf
	},
}

// webSocketConn adapts a gorilla/websocket.Conn to implement net.Conn.
// This allows gRPC to use WebSocket as its transport by providing a standard
// network connection interface that gRPC expects.
//
// The adapter handles buffering of partial WebSocket messages since WebSocket
// messages are discrete frames while net.Conn expects a continuous byte stream.
type webSocketConn struct {
	// websocket is the underlying WebSocket connection from gorilla/websocket
	websocket *websocket.Conn

	// readBuf stores any leftover bytes from a WebSocket message that didn't
	// fit into the caller's buffer during the last Read() call
	readBuf []byte

	// readDeadline is the deadline for read operations
	// Note: WebSocket deadlines are not fully implemented in this adapter
	readDeadline time.Time

	// writeDeadline is the deadline for write operations
	// Note: WebSocket deadlines are not fully implemented in this adapter
	writeDeadline time.Time

	// closeOnce ensures Close() is called only once
	closeOnce sync.Once

	// closed tracks whether the connection has been closed
	closed bool

	// closedMu protects the closed flag
	closedMu sync.RWMutex
}

// NewWebSocketConn wraps a WebSocket connection as a net.Conn.
// This is the core primitive that enables gRPC over WebSocket.
//
// The returned net.Conn implements the full interface required by gRPC,
// translating between WebSocket's message-oriented protocol and the
// stream-oriented protocol expected by net.Conn.
//
// Parameters:
//   - websocketConnection: An established WebSocket connection from gorilla/websocket
//
// Returns:
//   - A net.Conn that wraps the WebSocket connection
//
// Example:
//
//	websocketConnection, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	conn := bridge.NewWebSocketConn(websocketConnection)
//	// Use conn with gRPC or any code expecting net.Conn
func NewWebSocketConn(websocketConnection *websocket.Conn) net.Conn {
	return &webSocketConn{websocket: websocketConnection}
}

// Read reads data from the WebSocket connection into p.
// It implements the net.Conn Read method.
//
// This method bridges between WebSocket's message-oriented protocol and
// net.Conn's stream-oriented protocol. WebSocket messages are read as
// complete frames, but if a message is larger than the provided buffer p,
// the excess bytes are buffered internally and returned on subsequent Read calls.
//
// Parameters:
//   - destinationBuffer: Buffer to read data into
//
// Returns:
//   - bytesRead: Number of bytes read into destinationBuffer
//   - err: Any error that occurred during reading
//
// Behavior:
//   - Returns buffered data from previous reads if available
//   - Reads the next WebSocket message if no buffered data exists
//   - Only accepts binary WebSocket messages (returns net.ErrClosed for text messages)
//   - Buffers any data that doesn't fit in destinationBuffer for subsequent reads
func (c *webSocketConn) Read(destinationBuffer []byte) (int, error) {
	// Check if connection is closed
	c.closedMu.RLock()
	isClosed := c.closed
	c.closedMu.RUnlock()

	if isClosed {
		return 0, net.ErrClosed
	}

	// If we have buffered data from a previous read, return that first.
	// This happens when a WebSocket message was larger than the caller's buffer.
	if len(c.readBuf) > 0 {
		bytesRead := copy(destinationBuffer, c.readBuf)
		// Keep any remaining buffered data for the next Read() call
		c.readBuf = c.readBuf[bytesRead:]
		return bytesRead, nil
	}

	// Read the next complete WebSocket message frame
	messageType, messageData, err := c.websocket.ReadMessage()
	if err != nil {
		// WebSocket errors (connection closed, network errors, etc.)
		return 0, err
	}

	// gRPC sends data as binary, so we only accept binary WebSocket messages.
	// Text messages indicate a protocol violation.
	if messageType != websocket.BinaryMessage {
		return 0, net.ErrClosed
	}

	// Copy as much data as possible into the caller's buffer
	bytesRead := copy(destinationBuffer, messageData)
	// If the WebSocket message doesn't fit entirely in destinationBuffer,
	// save the remainder for the next Read() call
	if bytesRead < len(messageData) {
		c.readBuf = messageData[bytesRead:]
	}
	return bytesRead, nil
}

// Write writes data from p to the WebSocket connection.
// It implements the net.Conn Write method.
//
// The entire buffer p is sent as a single binary WebSocket message frame.
// WebSocket's message-oriented nature means each Write() becomes one WebSocket message.
//
// Parameters:
//   - sourceData: Data to write to the WebSocket
//
// Returns:
//   - bytesWritten: Number of bytes written (always len(sourceData) if err is nil)
//   - err: Any error that occurred during writing
//
// Note: Unlike traditional TCP sockets, WebSocket writes are atomic -
// either the entire message is sent or none of it is.
func (c *webSocketConn) Write(sourceData []byte) (int, error) {
	// Check if connection is closed
	c.closedMu.RLock()
	isClosed := c.closed
	c.closedMu.RUnlock()

	if isClosed {
		return 0, net.ErrClosed
	}

	// Send the entire buffer as a single binary WebSocket message
	err := c.websocket.WriteMessage(websocket.BinaryMessage, sourceData)
	if err != nil {
		// WebSocket write errors (connection closed, network errors, etc.)
		return 0, err
	}
	// WebSocket writes are all-or-nothing, so we always write len(sourceData) bytes
	return len(sourceData), nil
}

// Close closes the WebSocket connection.
// It implements the net.Conn Close method.
//
// This sends a WebSocket close frame and cleans up resources.
// After Close() is called, all future Read() and Write() operations will fail.
//
// Returns:
//   - err: Any error that occurred during closing
func (c *webSocketConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		c.closedMu.Lock()
		c.closed = true
		c.closedMu.Unlock()

		closeErr = c.websocket.Close()
	})
	return closeErr
}

// LocalAddr returns the local network address.
// It implements the net.Conn LocalAddr method.
//
// Returns:
//   - The local address of the underlying WebSocket connection
func (c *webSocketConn) LocalAddr() net.Addr {
	return c.websocket.LocalAddr()
}

// RemoteAddr returns the remote network address.
// It implements the net.Conn RemoteAddr method.
//
// Returns:
//   - The remote address of the underlying WebSocket connection
func (c *webSocketConn) RemoteAddr() net.Addr {
	return c.websocket.RemoteAddr()
}

// SetDeadline sets the read and write deadlines for the connection.
// It implements the net.Conn SetDeadline method.
//
// Note: This implementation stores the deadline values but does not
// currently enforce them. WebSocket deadline enforcement would require
// additional complexity with goroutines and timers.
//
// Parameters:
//   - deadline: The deadline time for both read and write operations
//
// Returns:
//   - Always returns nil (no errors)
func (c *webSocketConn) SetDeadline(deadline time.Time) error {
	c.readDeadline = deadline
	c.writeDeadline = deadline
	return nil
}

// SetReadDeadline sets the deadline for read operations.
// It implements the net.Conn SetReadDeadline method.
//
// Note: This implementation stores the deadline value but does not
// currently enforce it. WebSocket deadline enforcement would require
// additional complexity with goroutines and timers.
//
// Parameters:
//   - deadline: The deadline time for read operations
//
// Returns:
//   - Always returns nil (no errors)
func (c *webSocketConn) SetReadDeadline(deadline time.Time) error {
	c.readDeadline = deadline
	return nil
}

// SetWriteDeadline sets the deadline for write operations.
// It implements the net.Conn SetWriteDeadline method.
//
// Note: This implementation stores the deadline value but does not
// currently enforce it. WebSocket deadline enforcement would require
// additional complexity with goroutines and timers.
//
// Parameters:
//   - deadline: The deadline time for write operations
//
// Returns:
//   - Always returns nil (no errors)
func (c *webSocketConn) SetWriteDeadline(deadline time.Time) error {
	c.writeDeadline = deadline
	return nil
}
