//go:build js && wasm

package dialer

import (
	"net"
	"syscall/js"
	"time"
)

// webSocketConnection implements the net.Conn interface for a browser WebSocket.
// This allows the Go gRPC client to use a WebSocket as its underlying transport.
type webSocketConnection struct {
	webSocket js.Value
	// Channels to bridge asynchronous WebSocket messages to synchronous Read calls.
	readMessageChannel  chan []byte
	readErrorChannel    chan error
	writeMessageChannel chan []byte // For internal buffering if needed
}

// NewWebSocketConn creates a new net.Conn implementation that wraps a browser WebSocket.
// The provided js.Value should be a JavaScript WebSocket object.
func NewWebSocketConn(webSocket js.Value) net.Conn {
	connection := &webSocketConnection{
		webSocket:           webSocket,
		readMessageChannel:  make(chan []byte),
		readErrorChannel:    make(chan error),
		writeMessageChannel: make(chan []byte), // Initialize for potential future use
	}

	// Set up the onmessage handler for the WebSocket
	webSocket.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		event := args[0]
		data := event.Get("data")

		// Data from WebSocket can be ArrayBuffer or Blob
		var byteSlice []byte
		if data.Type() == js.TypeObject {
			// Convert to Uint8Array
			arrayBuffer := js.Global().Get("Uint8Array").New(data)
			length := arrayBuffer.Get("length").Int()
			if length > 0 {
				byteSlice = make([]byte, length)
				js.CopyBytesToGo(byteSlice, arrayBuffer)
			}
		}

		if len(byteSlice) > 0 {
			connection.readMessageChannel <- byteSlice
		}
		return nil
	}))

	// Set up the onerror handler
	webSocket.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// For simplicity, just send a generic error.
		// In a real app, you might parse the event for more details.
		connection.readErrorChannel <- net.ErrClosed
		return nil
	}))

	// Set up the onclose handler
	webSocket.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		connection.readErrorChannel <- net.ErrClosed
		close(connection.readMessageChannel)
		close(connection.readErrorChannel)
		return nil
	}))

	return connection
}

// Read reads data from the WebSocket. This is a blocking call.
func (webSocketConnection *webSocketConnection) Read(buffer []byte) (int, error) {
	select {
	case message := <-webSocketConnection.readMessageChannel:
		n := copy(buffer, message)
		return n, nil
	case err := <-webSocketConnection.readErrorChannel:
		return 0, err
	}
}

// Write writes data to the WebSocket.
func (webSocketConnection *webSocketConnection) Write(data []byte) (int, error) {
	// Convert Go byte slice to JavaScript Uint8Array
	uint8Array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(uint8Array, data)

	// Send the data over the WebSocket
	webSocketConnection.webSocket.Call("send", uint8Array)
	return len(data), nil
}

// Close closes the WebSocket connection.
func (webSocketConnection *webSocketConnection) Close() error {
	webSocketConnection.webSocket.Call("close")
	return nil
}

// LocalAddr returns the local network address. (Dummy implementation)
func (webSocketConnection *webSocketConnection) LocalAddr() net.Addr {
	return &webSocketAddr{"websocket", "local"}
}

// RemoteAddr returns the remote network address. (Dummy implementation)
func (webSocketConnection *webSocketConnection) RemoteAddr() net.Addr {
	return &webSocketAddr{"websocket", "remote"}
}

// SetDeadline sets the read and write deadlines associated with the connection. (Dummy implementation)
func (webSocketConnection *webSocketConnection) SetDeadline(time time.Time) error {
	// WebSockets don't directly support deadlines in the same way as TCP sockets.
	// For now, return an error or do nothing.
	return nil // Or errors.New("SetDeadline not supported for WebSockets")
}

// SetReadDeadline sets the read deadline. (Dummy implementation)
func (webSocketConnection *webSocketConnection) SetReadDeadline(time time.Time) error {
	return nil
}

// SetWriteDeadline sets the write deadline. (Dummy implementation)
func (webSocketConnection *webSocketConnection) SetWriteDeadline(time time.Time) error {
	return nil
}

// webSocketAddr is a dummy implementation of net.Addr for WebSockets.
type webSocketAddr struct {
	network string
	address string
}

func (addr *webSocketAddr) Network() string { return addr.network }
func (addr *webSocketAddr) String() string  { return addr.address }
