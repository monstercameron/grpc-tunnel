package bridge

import (
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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
func NewWebSocketConn(parseWebsocketConnection *websocket.Conn) net.Conn {
	return &webSocketConn{websocket: parseWebsocketConnection}
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
func (parseC *webSocketConn) Read(parseDestinationBuffer []byte) (int, error) {
	// Check if connection is closed
	parseC.closedMu.RLock()
	isClosed := parseC.closed
	parseC.closedMu.RUnlock()

	if isClosed {
		return 0, net.ErrClosed
	}

	// If we have buffered data from a previous read, return that first.
	// This happens when a WebSocket message was larger than the caller's buffer.
	if len(parseC.readBuf) > 0 {
		parseBytesRead := copy(parseDestinationBuffer, parseC.readBuf)
		// Keep any remaining buffered data for the next Read() call
		parseC.readBuf = parseC.readBuf[parseBytesRead:]
		return parseBytesRead, nil
	}

	// Read the next complete WebSocket message frame
	parseMessageType, parseMessageData, parseErr := parseC.websocket.ReadMessage()
	if parseErr != nil {
		// WebSocket errors (connection closed, network errors, etc.)
		return 0, parseErr
	}

	// gRPC sends data as binary, so we only accept binary WebSocket messages.
	// Text messages indicate a protocol violation.
	if parseMessageType != websocket.BinaryMessage {
		return 0, net.ErrClosed
	}

	// Copy as much data as possible into the caller's buffer
	parseBytesRead2 := copy(parseDestinationBuffer, parseMessageData)
	// If the WebSocket message doesn't fit entirely in destinationBuffer,
	// save the remainder for the next Read() call
	if parseBytesRead2 < len(parseMessageData) {
		parseC.readBuf = parseMessageData[parseBytesRead2:]
	}
	return parseBytesRead2, nil
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
func (parseC *webSocketConn) Write(parseSourceData []byte) (int, error) {
	// Check if connection is closed
	parseC.closedMu.RLock()
	isClosed := parseC.closed
	parseC.closedMu.RUnlock()

	if isClosed {
		return 0, net.ErrClosed
	}

	// Send the entire buffer as a single binary WebSocket message
	parseErr := parseC.websocket.WriteMessage(websocket.BinaryMessage, parseSourceData)
	if parseErr != nil {
		// WebSocket write errors (connection closed, network errors, etc.)
		return 0, parseErr
	}
	// WebSocket writes are all-or-nothing, so we always write len(sourceData) bytes
	return len(parseSourceData), nil
}

// Close closes the WebSocket connection.
// It implements the net.Conn Close method.
//
// This sends a WebSocket close frame and cleans up resources.
// After Close() is called, all future Read() and Write() operations will fail.
//
// Returns:
//   - err: Any error that occurred during closing
func (parseC *webSocketConn) Close() error {
	var parseCloseErr error
	parseC.closeOnce.Do(func() {
		parseC.closedMu.Lock()
		parseC.closed = true
		parseC.closedMu.Unlock()

		parseCloseErr = parseC.websocket.Close()
	})
	return parseCloseErr
}

// LocalAddr returns the local network address.
// It implements the net.Conn LocalAddr method.
//
// Returns:
//   - The local address of the underlying WebSocket connection
func (parseC *webSocketConn) LocalAddr() net.Addr {
	return parseC.websocket.LocalAddr()
}

// RemoteAddr returns the remote network address.
// It implements the net.Conn RemoteAddr method.
//
// Returns:
//   - The remote address of the underlying WebSocket connection
func (parseC *webSocketConn) RemoteAddr() net.Addr {
	return parseC.websocket.RemoteAddr()
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
func (parseC *webSocketConn) SetDeadline(parseDeadline time.Time) error {
	parseC.readDeadline = parseDeadline
	parseC.writeDeadline = parseDeadline
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
func (parseC *webSocketConn) SetReadDeadline(parseDeadline time.Time) error {
	parseC.readDeadline = parseDeadline
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
func (parseC *webSocketConn) SetWriteDeadline(parseDeadline time.Time) error {
	parseC.writeDeadline = parseDeadline
	return nil
}
