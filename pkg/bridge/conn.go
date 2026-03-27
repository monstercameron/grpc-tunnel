package bridge

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
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

	// readStream stores the active WebSocket frame reader so Read can stream
	// bytes directly without allocating whole-frame message buffers.
	readStream io.Reader

	// readBuf stores any leftover bytes from a WebSocket message that didn't
	// fit into the caller's buffer during the last Read() call
	readBuf []byte

	// readDeadline is the deadline for read operations and mirrors the
	// deadline set on the underlying WebSocket connection.
	readDeadline time.Time

	// writeDeadline is the deadline for write operations
	// and mirrors the deadline set on the underlying WebSocket connection.
	writeDeadline time.Time

	// deadlineMu serializes deadline updates to the underlying websocket.
	deadlineMu sync.Mutex

	// closeOnce ensures Close() is called only once
	closeOnce sync.Once

	// isClosed tracks whether the connection has been closed.
	isClosed atomic.Bool

	// writeMu serializes websocket writes because gorilla/websocket
	// panics when multiple goroutines write concurrently.
	writeMu sync.Mutex
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
	parseConnection := &webSocketConn{websocket: parseWebsocketConnection}
	if parseWebsocketConnection == nil {
		parseConnection.isClosed.Store(true)
	}
	return parseConnection
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
	if parseC.isClosed.Load() {
		return 0, net.ErrClosed
	}
	if parseC.websocket == nil {
		return 0, net.ErrClosed
	}

	for {
		if parseC.readStream == nil {
			// Stream the next frame directly instead of materializing the full
			// message in memory with ReadMessage().
			parseMessageType, parseFrameReader, parseErr := parseC.websocket.NextReader()
			if parseErr != nil {
				// WebSocket errors (connection closed, network errors, etc.)
				return 0, parseErr
			}

			// gRPC sends data as binary, so we only accept binary WebSocket messages.
			// Text messages indicate a protocol violation.
			if parseMessageType != websocket.BinaryMessage {
				return 0, net.ErrClosed
			}
			parseC.readStream = parseFrameReader
		}

		parseBytesRead, parseErr := parseC.readStream.Read(parseDestinationBuffer)
		if parseErr == io.EOF {
			parseC.readStream = nil
			if parseBytesRead > 0 {
				return parseBytesRead, nil
			}
			// Empty frame: continue to the next frame.
			continue
		}
		return parseBytesRead, parseErr
	}
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
	parseC.writeMu.Lock()
	defer parseC.writeMu.Unlock()

	if parseC.isClosed.Load() {
		return 0, net.ErrClosed
	}
	if parseC.websocket == nil {
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
		parseC.isClosed.Store(true)
		if parseC.websocket == nil {
			return
		}
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
	if parseC.websocket == nil {
		return nil
	}
	return parseC.websocket.LocalAddr()
}

// RemoteAddr returns the remote network address.
// It implements the net.Conn RemoteAddr method.
//
// Returns:
//   - The remote address of the underlying WebSocket connection
func (parseC *webSocketConn) RemoteAddr() net.Addr {
	if parseC.websocket == nil {
		return nil
	}
	return parseC.websocket.RemoteAddr()
}

// SetDeadline sets the read and write deadlines for the connection.
// It implements the net.Conn SetDeadline method.
//
// Parameters:
//   - deadline: The deadline time for both read and write operations
//
// Returns:
//   - Any error from the underlying WebSocket deadline setters
func (parseC *webSocketConn) SetDeadline(parseDeadline time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()

	parseC.readDeadline = parseDeadline
	parseC.writeDeadline = parseDeadline

	if parseC.websocket == nil {
		return net.ErrClosed
	}

	if parseErr := parseC.websocket.SetReadDeadline(parseDeadline); parseErr != nil {
		return parseErr
	}
	return parseC.websocket.SetWriteDeadline(parseDeadline)
}

// SetReadDeadline sets the deadline for read operations.
// It implements the net.Conn SetReadDeadline method.
//
// Parameters:
//   - deadline: The deadline time for read operations
//
// Returns:
//   - Any error from the underlying WebSocket read deadline setter
func (parseC *webSocketConn) SetReadDeadline(parseDeadline time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()

	parseC.readDeadline = parseDeadline
	if parseC.websocket == nil {
		return net.ErrClosed
	}
	return parseC.websocket.SetReadDeadline(parseDeadline)
}

// SetWriteDeadline sets the deadline for write operations.
// It implements the net.Conn SetWriteDeadline method.
//
// Parameters:
//   - deadline: The deadline time for write operations
//
// Returns:
//   - Any error from the underlying WebSocket write deadline setter
func (parseC *webSocketConn) SetWriteDeadline(parseDeadline time.Time) error {
	parseC.deadlineMu.Lock()
	defer parseC.deadlineMu.Unlock()

	parseC.writeDeadline = parseDeadline
	if parseC.websocket == nil {
		return net.ErrClosed
	}
	return parseC.websocket.SetWriteDeadline(parseDeadline)
}
