//go:build js && wasm

package dialer

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"
)

const (
	// JavaScript API names
	jsGlobalWebSocket  = "WebSocket"
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

	// limitConnectionQueuedMessages caps queued inbound frames so slow readers
	// cannot cause unbounded memory growth in the browser process.
	limitConnectionQueuedMessages = 256
	// limitConnectionQueuedBytes caps total queued inbound payload bytes.
	limitConnectionQueuedBytes = 16 << 20
)

// browserWebSocketConnection implements net.Conn over browser WebSocket APIs.
type browserWebSocketConnection struct {
	browserWebSocket js.Value
	// cacheConnectionUint8Array holds the constructor so hot-path callbacks
	// do not repeatedly resolve it from the JS global object.
	cacheConnectionUint8Array js.Value

	// incomingMessagesChannel is a signal channel that tells Read() new data is queued.
	incomingMessagesChannel chan struct{}
	// incomingErrorsChannel receives socket errors and close notifications.
	incomingErrorsChannel chan error

	// readMessageBuffer stores remaining bytes when Read() receives a message
	// larger than the destination buffer.
	readMessageBuffer []byte
	// queuedMessages stores complete WebSocket messages until Read() consumes them.
	// The queue is bounded by limitConnectionQueuedMessages.
	queuedMessages  [][]byte
	queuedBytesSize int
	shiftQueueStart int

	queueMu sync.Mutex
	readMu  sync.Mutex
	// errorChannelMu serializes error-channel sends vs close to avoid send-on-closed-channel panics.
	errorChannelMu sync.RWMutex

	closeOnce sync.Once
	isClosed  atomic.Bool

	messageHandler *js.Func
	errorHandler   *js.Func
	closeHandler   *js.Func
}

// NewWebSocketConn creates a net.Conn adapter for a browser WebSocket.
func NewWebSocketConn(parseBrowserWebSocket js.Value) net.Conn {
	parseConnection := &browserWebSocketConnection{
		browserWebSocket:          parseBrowserWebSocket,
		cacheConnectionUint8Array: js.Global().Get(jsGlobalUint8Array),
		incomingMessagesChannel:   make(chan struct{}, 1),
		incomingErrorsChannel:     make(chan error, 4),
		queuedMessages:            make([][]byte, 0, 8),
	}

	parseMessageHandler := js.FuncOf(func(parseThis js.Value, parseEventArgs []js.Value) interface{} {
		parseMessageEvent := parseEventArgs[0]
		parseMessageData := parseMessageEvent.Get(jsPropertyData)

		if parseMessageData.Type() != js.TypeObject {
			parseConnection.storeConnectionError(fmt.Errorf("WASM: unsupported websocket data type: %s", parseMessageData.Type().String()))
			return nil
		}

		parseUint8Array := parseConnection.cacheConnectionUint8Array.New(parseMessageData)
		parseArrayLength := parseUint8Array.Get(jsPropertyLength).Int()
		parseMessageBytes := make([]byte, parseArrayLength)
		if parseArrayLength > 0 {
			js.CopyBytesToGo(parseMessageBytes, parseUint8Array)
		}

		parseConnection.storeIncomingMessage(parseMessageBytes)
		return nil
	})
	parseConnection.messageHandler = &parseMessageHandler
	parseBrowserWebSocket.Set(jsEventOnMessage, parseMessageHandler)

	parseErrorHandler := js.FuncOf(func(parseThis js.Value, parseEventArgs []js.Value) interface{} {
		parseConnection.storeConnectionError(net.ErrClosed)
		return nil
	})
	parseConnection.errorHandler = &parseErrorHandler
	parseBrowserWebSocket.Set(jsEventOnError, parseErrorHandler)

	parseCloseHandler := js.FuncOf(func(parseThis js.Value, parseEventArgs []js.Value) interface{} {
		parseConnection.closeChannels()
		return nil
	})
	parseConnection.closeHandler = &parseCloseHandler
	parseBrowserWebSocket.Set(jsEventOnClose, parseCloseHandler)

	return parseConnection
}

// storeIncomingMessage queues an inbound WebSocket message and signals readers.
func (parseConnection *browserWebSocketConnection) storeIncomingMessage(parseMessage []byte) {
	if parseConnection.isConnectionClosed() {
		return
	}

	var parseQueueDepth int
	var parseQueueBytes int
	isQueueOverflow := false

	parseConnection.queueMu.Lock()
	parseQueueDepth = len(parseConnection.queuedMessages) - parseConnection.shiftQueueStart
	parseQueueBytes = parseConnection.queuedBytesSize
	parseAvailableQueueBytes := limitConnectionQueuedBytes - len(parseMessage)
	if parseQueueDepth >= limitConnectionQueuedMessages ||
		parseAvailableQueueBytes < 0 ||
		parseQueueBytes > parseAvailableQueueBytes {
		isQueueOverflow = true
	} else {
		parseConnection.clearQueuedMessagesHeadLocked()
		parseConnection.queuedMessages = append(parseConnection.queuedMessages, parseMessage)
		parseConnection.queuedBytesSize += len(parseMessage)
	}
	parseConnection.queueMu.Unlock()

	if isQueueOverflow {
		parseConnection.handleConnectionQueueOverflow(parseQueueDepth, parseQueueBytes, len(parseMessage))
		return
	}

	// Signal is edge-triggered: one token is enough to wake readers, even if many
	// messages were queued while the reader was busy.
	select {
	case parseConnection.incomingMessagesChannel <- struct{}{}:
	default:
	}
}

// clearQueuedMessagesHeadLocked compacts the queue when consumed head space is large.
func (parseConnection *browserWebSocketConnection) clearQueuedMessagesHeadLocked() {
	if parseConnection.shiftQueueStart == 0 {
		return
	}
	if parseConnection.shiftQueueStart < len(parseConnection.queuedMessages)/2 &&
		len(parseConnection.queuedMessages) < cap(parseConnection.queuedMessages) {
		return
	}

	copy(parseConnection.queuedMessages, parseConnection.queuedMessages[parseConnection.shiftQueueStart:])
	parseKeepCount := len(parseConnection.queuedMessages) - parseConnection.shiftQueueStart
	for parseI := parseKeepCount; parseI < len(parseConnection.queuedMessages); parseI++ {
		parseConnection.queuedMessages[parseI] = nil
	}
	parseConnection.queuedMessages = parseConnection.queuedMessages[:parseKeepCount]
	parseConnection.shiftQueueStart = 0
}

// handleConnectionQueueOverflow aborts the socket when inbound buffering exceeds
// the queue safety limit.
func (parseConnection *browserWebSocketConnection) handleConnectionQueueOverflow(parseQueueDepth int, parseQueueBytes int, parseIncomingMessageBytes int) {
	parseOverflowErr := fmt.Errorf(
		"WASM: incoming websocket backlog exceeded safety limits (messages: %d/%d, bytes: %d/%d)",
		parseQueueDepth+1,
		limitConnectionQueuedMessages,
		parseQueueBytes+parseIncomingMessageBytes,
		limitConnectionQueuedBytes,
	)
	parseConnection.storeConnectionError(parseOverflowErr)
	parseConnection.closeChannels()
	parseConnection.browserWebSocket.Call(jsMethodClose)
}

// storeConnectionError sends a connection error to waiting readers.
func (parseConnection *browserWebSocketConnection) storeConnectionError(parseErr error) {
	if parseConnection.isConnectionClosed() {
		return
	}
	parseConnection.errorChannelMu.RLock()
	defer parseConnection.errorChannelMu.RUnlock()
	if parseConnection.isConnectionClosed() {
		return
	}

	// Error delivery is best-effort and non-blocking so browser callbacks never
	// stall the JS event loop.
	select {
	case parseConnection.incomingErrorsChannel <- parseErr:
	default:
	}
}

// shiftQueuedMessage pops the next queued message.
func (parseConnection *browserWebSocketConnection) shiftQueuedMessage() ([]byte, bool) {
	parseConnection.queueMu.Lock()
	defer parseConnection.queueMu.Unlock()

	if parseConnection.shiftQueueStart >= len(parseConnection.queuedMessages) {
		return nil, false
	}

	parseMessage := parseConnection.queuedMessages[parseConnection.shiftQueueStart]
	parseConnection.queuedMessages[parseConnection.shiftQueueStart] = nil
	parseConnection.queuedBytesSize -= len(parseMessage)
	if parseConnection.queuedBytesSize < 0 {
		parseConnection.queuedBytesSize = 0
	}
	parseConnection.shiftQueueStart++
	if parseConnection.shiftQueueStart >= len(parseConnection.queuedMessages) {
		parseConnection.queuedMessages = parseConnection.queuedMessages[:0]
		parseConnection.shiftQueueStart = 0
	} else {
		parseConnection.clearQueuedMessagesHeadLocked()
	}
	return parseMessage, true
}

// isConnectionClosed reports whether the connection is already closed.
func (parseConnection *browserWebSocketConnection) isConnectionClosed() bool {
	return parseConnection.isClosed.Load()
}

// closeChannels marks the connection closed, detaches event handlers, and wakes readers.
func (parseConnection *browserWebSocketConnection) closeChannels() {
	parseConnection.closeOnce.Do(func() {
		parseConnection.isClosed.Store(true)

		// Detach JS callbacks first so no late event can write into Go-owned state
		// after teardown begins.
		parseConnection.browserWebSocket.Set(jsEventOnMessage, js.Null())
		parseConnection.browserWebSocket.Set(jsEventOnError, js.Null())
		parseConnection.browserWebSocket.Set(jsEventOnClose, js.Null())

		// Drop buffered payload references so closed sockets release memory promptly.
		parseConnection.queueMu.Lock()
		for parseI := range parseConnection.queuedMessages {
			parseConnection.queuedMessages[parseI] = nil
		}
		parseConnection.queuedMessages = nil
		parseConnection.shiftQueueStart = 0
		parseConnection.queuedBytesSize = 0
		parseConnection.queueMu.Unlock()
		if parseConnection.readMu.TryLock() {
			parseConnection.readMessageBuffer = nil
			parseConnection.readMu.Unlock()
		}

		parseConnection.errorChannelMu.Lock()
		close(parseConnection.incomingErrorsChannel)
		parseConnection.errorChannelMu.Unlock()
		parseConnection.releaseEventHandlers()
	})
}

// releaseEventHandlers releases js.Func handlers that were installed on the socket.
func (parseConnection *browserWebSocketConnection) releaseEventHandlers() {
	if parseConnection.messageHandler != nil {
		parseConnection.messageHandler.Release()
		parseConnection.messageHandler = nil
	}
	if parseConnection.errorHandler != nil {
		parseConnection.errorHandler.Release()
		parseConnection.errorHandler = nil
	}
	if parseConnection.closeHandler != nil {
		parseConnection.closeHandler.Release()
		parseConnection.closeHandler = nil
	}
}

// Read reads bytes from the WebSocket stream into parseDestinationBuffer.
func (parseConnection *browserWebSocketConnection) Read(parseDestinationBuffer []byte) (int, error) {
	parseConnection.readMu.Lock()
	defer parseConnection.readMu.Unlock()

	if parseConnection.isConnectionClosed() {
		return 0, net.ErrClosed
	}

	if len(parseConnection.readMessageBuffer) > 0 {
		parseBytesRead := copy(parseDestinationBuffer, parseConnection.readMessageBuffer)
		parseConnection.readMessageBuffer = parseConnection.readMessageBuffer[parseBytesRead:]
		return parseBytesRead, nil
	}

	for {
		// Preserve stream semantics across message boundaries by draining any queued
		// frame and buffering only the unread tail for the next Read call.
		if parseQueuedMessage, parseOk := parseConnection.shiftQueuedMessage(); parseOk {
			parseBytesRead := copy(parseDestinationBuffer, parseQueuedMessage)
			if parseBytesRead < len(parseQueuedMessage) {
				parseConnection.readMessageBuffer = parseQueuedMessage[parseBytesRead:]
			}
			return parseBytesRead, nil
		}

		select {
		case parseErr, parseOk := <-parseConnection.incomingErrorsChannel:
			if !parseOk {
				return 0, net.ErrClosed
			}
			return 0, parseErr
		case <-parseConnection.incomingMessagesChannel:
		}
	}
}

// Write writes parseSourceData to the WebSocket as one binary message.
func (parseConnection *browserWebSocketConnection) Write(parseSourceData []byte) (parseBytesWritten int, parseErr error) {
	if parseConnection.isConnectionClosed() {
		return 0, net.ErrClosed
	}

	defer func() {
		// JS interop can panic if the browser socket state is invalid; convert that
		// into a Go error so callers see a normal transport failure.
		if parseRecovered := recover(); parseRecovered != nil {
			parseBytesWritten = 0
			parseErr = fmt.Errorf("WASM: websocket write failed: %v", parseRecovered)
		}
	}()

	parseUint8ArrayToSend := parseConnection.cacheConnectionUint8Array.New(len(parseSourceData))
	js.CopyBytesToJS(parseUint8ArrayToSend, parseSourceData)
	parseConnection.browserWebSocket.Call(jsMethodSend, parseUint8ArrayToSend)

	return len(parseSourceData), nil
}

// Close closes the WebSocket connection.
func (parseConnection *browserWebSocketConnection) Close() error {
	parseConnection.closeChannels()
	parseConnection.browserWebSocket.Call(jsMethodClose)
	return nil
}

// LocalAddr returns a placeholder local address.
func (parseConnection *browserWebSocketConnection) LocalAddr() net.Addr {
	return &browserWebSocketAddr{networkTypeWebSocket, addressLocal}
}

// RemoteAddr returns a placeholder remote address.
func (parseConnection *browserWebSocketConnection) RemoteAddr() net.Addr {
	return &browserWebSocketAddr{networkTypeWebSocket, addressRemote}
}

// SetDeadline is a no-op in browser WebSocket environments.
func (parseConnection *browserWebSocketConnection) SetDeadline(parseDeadline time.Time) error {
	return nil
}

// SetReadDeadline is a no-op in browser WebSocket environments.
func (parseConnection *browserWebSocketConnection) SetReadDeadline(parseDeadline time.Time) error {
	return nil
}

// SetWriteDeadline is a no-op in browser WebSocket environments.
func (parseConnection *browserWebSocketConnection) SetWriteDeadline(parseDeadline time.Time) error {
	return nil
}

// browserWebSocketAddr is a placeholder implementation of net.Addr.
type browserWebSocketAddr struct {
	networkType   string
	addressString string
}

// Network returns the placeholder network type.
func (parseAddr *browserWebSocketAddr) Network() string { return parseAddr.networkType }

// String returns the placeholder address string.
func (parseAddr *browserWebSocketAddr) String() string { return parseAddr.addressString }
