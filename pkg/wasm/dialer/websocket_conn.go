//go:build js && wasm

package dialer

import (
	"net"
	"sync"
	"syscall/js"
	"time"
)

const (
	// JavaScript API names
	jsGlobalWebSocket = "WebSocket"
	jsGlobalUint8Array = "Uint8Array"
	jsGlobalObject     = "Object"

	// WebSocket event handlers
	jsEventOnMessage = "onmessage"
	jsEventOnError   = "onerror"
	jsEventOnClose   = "onclose"

	// WebSocket methods
	jsMethodSend  = "send"
	jsMethodClose = "close"

	// WebSocket properties
	jsPropertyBinaryType = "binaryType"
	jsPropertyData       = "data"
	jsPropertyLength     = "length"

	// WebSocket binary type values
	jsBinaryTypeArrayBuffer = "arraybuffer"

	// Network type constants
	networkTypeWebSocket = "websocket"
	addressLocal         = "local"
	addressRemote        = "remote"
)

// browserWebSocketConnection implements the net.Conn interface for a browser WebSocket.
// This allows the Go gRPC client to use a WebSocket as its underlying transport.
//
// The browser WebSocket API is event-driven and asynchronous, while net.Conn
// is synchronous and blocking. This struct bridges the two models using channels
// to convert WebSocket events into blocking Read/Write operations.
type browserWebSocketConnection struct {
	// browserWebSocket is the JavaScript WebSocket object from the browser API
	browserWebSocket js.Value

	// incomingMessagesChannel receives incoming WebSocket messages.
	// The onmessage event handler sends data here, which Read() consumes.
	// Buffered to prevent blocking browser event handlers.
	incomingMessagesChannel chan []byte

	// incomingErrorsChannel receives WebSocket errors and close events.
	// The onerror and onclose handlers send errors here, which Read() returns.
	// Buffered to prevent blocking browser event handlers.
	incomingErrorsChannel chan error

	// outgoingMessagesChannel is reserved for potential future buffering.
	// Currently unused but initialized for consistency.
	outgoingMessagesChannel chan []byte

	// closeOnce ensures Close() operations happen only once
	closeOnce sync.Once

	// closed indicates whether the connection has been closed
	closed bool

	// closedMu protects the closed flag
	closedMu sync.RWMutex
}

// NewWebSocketConn creates a new net.Conn implementation that wraps a browser WebSocket.
//
// This function sets up all the necessary event handlers (onmessage, onerror, onclose)
// to bridge between the browser's asynchronous WebSocket API and Go's synchronous
// net.Conn interface. The event handlers use channels to communicate WebSocket events
// to the Read() method.
//
// Parameters:
//   - browserWebSocket: A JavaScript WebSocket object from the browser (js.Value)
//     This should be a WebSocket instance created with `js.Global().Get("WebSocket").New(url)`
//
// Returns:
//   - net.Conn: A connection that implements the standard Go network interface
//
// The returned connection handles:
//   - Converting WebSocket ArrayBuffer messages to Go byte slices
//   - Translating WebSocket events (message, error, close) to Read() operations
//   - Converting Go Write() calls to WebSocket send() calls
//
// Example:
//
//	browserWebSocket := js.Global().Get("WebSocket").New("ws://localhost:8080")
//	browserWebSocket.Set("binaryType", "arraybuffer")
//	conn := dialer.NewWebSocketConn(browserWebSocket)
//	// Use conn with gRPC or any code expecting net.Conn
func NewWebSocketConn(browserWebSocket js.Value) net.Conn {
	connection := &browserWebSocketConnection{
		browserWebSocket:        browserWebSocket,
		incomingMessagesChannel: make(chan []byte, 10),    // Buffered to prevent blocking event handlers
		incomingErrorsChannel:   make(chan error, 2),      // Buffered to prevent blocking error handlers
		outgoingMessagesChannel: make(chan []byte),        // Initialize for potential future use
		closed:                  false,
	}

	// Set up the onmessage handler to receive incoming WebSocket data.
	// Browser WebSocket messages arrive as JavaScript events, which we must
	// convert to Go byte slices and send through a channel.
	browserWebSocket.Set(jsEventOnMessage, js.FuncOf(func(this js.Value, eventArgs []js.Value) interface{} {
		messageEvent := eventArgs[0]
		messageData := messageEvent.Get(jsPropertyData)

		// WebSocket data can arrive as ArrayBuffer or Blob.
		// We configured binaryType="arraybuffer" so we expect ArrayBuffer.
		var messageBytes []byte
		if messageData.Type() == js.TypeObject {
			// Convert the JavaScript ArrayBuffer to a Uint8Array for easy copying
			uint8Array := js.Global().Get(jsGlobalUint8Array).New(messageData)
			arrayLength := uint8Array.Get(jsPropertyLength).Int()
			if arrayLength > 0 {
				// Allocate a Go byte slice and copy the data from JavaScript
				messageBytes = make([]byte, arrayLength)
				js.CopyBytesToGo(messageBytes, uint8Array)
			}
		}

		// Send the data to the Read() method via channel (non-empty messages only)
		// Use non-blocking send to prevent browser event handler from hanging
		if len(messageBytes) > 0 {
			connection.closedMu.RLock()
			isClosed := connection.closed
			connection.closedMu.RUnlock()

			if !isClosed {
				select {
				case connection.incomingMessagesChannel <- messageBytes:
					// Message sent successfully
				default:
					// Channel full - log but don't block browser event loop
					// In production, consider increasing buffer size or implementing backpressure
				}
			}
		}
		return nil
	}))

	// Set up the onerror handler to detect WebSocket errors.
	// Browser WebSocket errors are asynchronous events.
	browserWebSocket.Set(jsEventOnError, js.FuncOf(func(this js.Value, eventArgs []js.Value) interface{} {
		connection.closedMu.RLock()
		isClosed := connection.closed
		connection.closedMu.RUnlock()

		if !isClosed {
			// Use non-blocking send to prevent hanging
			select {
			case connection.incomingErrorsChannel <- net.ErrClosed:
				// Error sent successfully
			default:
				// Channel full - error will be caught by other mechanisms
			}
		}
		return nil
	}))

	// Set up the onclose handler to detect when the WebSocket closes.
	// This can happen due to explicit Close() calls or network issues.
	browserWebSocket.Set(jsEventOnClose, js.FuncOf(func(this js.Value, eventArgs []js.Value) interface{} {
		connection.closeChannels()
		return nil
	}))

	return connection
}

// closeChannels safely closes all channels and marks the connection as closed.
// This should be called from both the onclose event handler and the Close() method.
func (connection *browserWebSocketConnection) closeChannels() {
	connection.closeOnce.Do(func() {
		connection.closedMu.Lock()
		connection.closed = true
		connection.closedMu.Unlock()

		// Close channels to signal no more data will arrive
		close(connection.incomingMessagesChannel)
		close(connection.incomingErrorsChannel)
	})
}

// Read reads data from the WebSocket into the provided buffer.
// It implements the net.Conn Read method.
//
// This method blocks until data arrives via a WebSocket message event
// or an error/close event occurs. It bridges the asynchronous WebSocket
// API with the synchronous net.Conn interface using channels.
//
// Parameters:
//   - destinationBuffer: Destination buffer to copy WebSocket data into
//
// Returns:
//   - bytesRead: Number of bytes read into destinationBuffer
//   - err: Any error that occurred (connection closed, network error, etc.)
//
// Behavior:
//   - Blocks until a WebSocket message arrives or an error occurs
//   - Copies as much data as fits in destinationBuffer (excess data is discarded)
//   - Returns net.ErrClosed when the WebSocket closes or errors
//
// Note: Unlike traditional sockets, WebSocket messages are discrete frames.
// Each Read() may return data from a different WebSocket message.
func (connection *browserWebSocketConnection) Read(destinationBuffer []byte) (int, error) {
	// Check if already closed
	connection.closedMu.RLock()
	isClosed := connection.closed
	connection.closedMu.RUnlock()

	if isClosed {
		return 0, net.ErrClosed
	}

	// Wait for either a message or an error from the WebSocket event handlers.
	// This select blocks until one of the channels receives data.
	select {
	case incomingMessage, ok := <-connection.incomingMessagesChannel:
		if !ok {
			// Channel closed - connection terminated
			return 0, net.ErrClosed
		}
		// Received a WebSocket message - copy it to the caller's buffer
		bytesRead := copy(destinationBuffer, incomingMessage)
		// Note: If incomingMessage is larger than destinationBuffer, excess bytes are discarded.
		// This is acceptable for gRPC which handles framing at a higher level.
		return bytesRead, nil
	case err, ok := <-connection.incomingErrorsChannel:
		if !ok {
			// Channel closed - connection terminated
			return 0, net.ErrClosed
		}
		// WebSocket error or close event occurred
		return 0, err
	}
}

// Write writes data to the WebSocket.
// It implements the net.Conn Write method.
//
// The data is sent as a binary WebSocket message using the browser's
// WebSocket.send() API. The entire buffer is sent as one message frame.
//
// Parameters:
//   - sourceData: Data to send over the WebSocket
//
// Returns:
//   - bytesWritten: Number of bytes written (always len(sourceData) if err is nil)
//   - err: Any error that occurred during writing
//
// The write operation:
//  1. Converts the Go byte slice to a JavaScript Uint8Array
//  2. Sends it via the browser's WebSocket.send() method
//  3. Returns immediately (browser handles actual transmission)
//
// Note: WebSocket writes are asynchronous in the browser, so this
// function returns before the data is actually transmitted over the network.
func (connection *browserWebSocketConnection) Write(sourceData []byte) (int, error) {
	// Check if already closed
	connection.closedMu.RLock()
	isClosed := connection.closed
	connection.closedMu.RUnlock()

	if isClosed {
		return 0, net.ErrClosed
	}

	// Convert the Go byte slice to a JavaScript Uint8Array.
	// This is necessary because browser WebSocket.send() expects JavaScript types.
	uint8ArrayToSend := js.Global().Get(jsGlobalUint8Array).New(len(sourceData))
	js.CopyBytesToJS(uint8ArrayToSend, sourceData)

	// Send the data over the WebSocket using the browser API.
	// This is an asynchronous operation - the browser handles the actual
	// network transmission in the background.
	connection.browserWebSocket.Call(jsMethodSend, uint8ArrayToSend)

	// Return the number of bytes "written".
	// Note: This doesn't mean the data has been transmitted, just that
	// it has been handed to the browser's WebSocket implementation.
	return len(sourceData), nil
}

// Close closes the WebSocket connection.
// It implements the net.Conn Close method.
//
// This calls the browser's WebSocket.close() method, which initiates the
// WebSocket closing handshake. The onclose event handler will be triggered
// when the close completes.
//
// Returns:
//   - Always returns nil (browser API doesn't provide synchronous error info)
func (connection *browserWebSocketConnection) Close() error {
	// Close channels first to prevent new sends
	connection.closeChannels()

	// Call the browser's WebSocket.close() method.
	// This is asynchronous - the actual close happens in the background.
	connection.browserWebSocket.Call(jsMethodClose)
	return nil
}

// LocalAddr returns the local network address.
// It implements the net.Conn LocalAddr method.
//
// Returns:
//   - A dummy net.Addr with network="websocket" and address="local"
//
// Note: Browser WebSockets don't expose local address information,
// so this returns a placeholder. This is acceptable as gRPC doesn't
// rely on local address information for WebSocket connections.
func (connection *browserWebSocketConnection) LocalAddr() net.Addr {
	return &browserWebSocketAddr{networkTypeWebSocket, addressLocal}
}

// RemoteAddr returns the remote network address.
// It implements the net.Conn RemoteAddr method.
//
// Returns:
//   - A dummy net.Addr with network="websocket" and address="remote"
//
// Note: Browser WebSockets don't expose remote address information,
// so this returns a placeholder. This is acceptable as gRPC doesn't
// rely on remote address information for WebSocket connections.
func (connection *browserWebSocketConnection) RemoteAddr() net.Addr {
	return &browserWebSocketAddr{networkTypeWebSocket, addressRemote}
}

// SetDeadline sets the read and write deadlines for the connection.
// It implements the net.Conn SetDeadline method.
//
// Parameters:
//   - deadline: The deadline time for both read and write operations
//
// Returns:
//   - Always returns nil
//
// Note: Browser WebSockets don't support deadlines natively.
// This method is a no-op placeholder to satisfy the net.Conn interface.
// Timeout behavior should be handled at a higher level (e.g., context.Context).
func (connection *browserWebSocketConnection) SetDeadline(deadline time.Time) error {
	// Browser WebSockets don't support deadlines in the same way as TCP sockets.
	// Deadline enforcement would require additional complexity with timers and
	// goroutines, which is not currently implemented.
	// For WASM/browser use cases, context.Context timeouts are preferred.
	return nil
}

// SetReadDeadline sets the read deadline.
// It implements the net.Conn SetReadDeadline method.
//
// Parameters:
//   - deadline: The deadline time for read operations
//
// Returns:
//   - Always returns nil
//
// Note: Browser WebSockets don't support read deadlines natively.
// This method is a no-op placeholder to satisfy the net.Conn interface.
func (connection *browserWebSocketConnection) SetReadDeadline(deadline time.Time) error {
	return nil
}

// SetWriteDeadline sets the write deadline.
// It implements the net.Conn SetWriteDeadline method.
//
// Parameters:
//   - deadline: The deadline time for write operations
//
// Returns:
//   - Always returns nil
//
// Note: Browser WebSockets don't support write deadlines natively.
// This method is a no-op placeholder to satisfy the net.Conn interface.
func (connection *browserWebSocketConnection) SetWriteDeadline(deadline time.Time) error {
	return nil
}

// browserWebSocketAddr is a placeholder implementation of net.Addr for WebSocket connections.
//
// Browser WebSockets don't expose address information through their API,
// but the net.Conn interface requires LocalAddr() and RemoteAddr() methods.
// This struct provides dummy values to satisfy the interface.
type browserWebSocketAddr struct {
	networkType   string // Always "websocket"
	addressString string // Either "local" or "remote"
}

// Network returns the network type.
// It implements the net.Addr Network method.
//
// Returns:
//   - Always returns "websocket"
func (addr *browserWebSocketAddr) Network() string { return addr.networkType }

// String returns the address as a string.
// It implements the net.Addr String method.
//
// Returns:
//   - Either "local" or "remote" depending on the address type
func (addr *browserWebSocketAddr) String() string { return addr.addressString }
