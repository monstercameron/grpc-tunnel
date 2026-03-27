//go:build js && wasm

package dialer

import (
	"context"
	"net"
	"syscall/js"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// JavaScript global objects
	jsGlobalNavigator = "navigator"
	jsGlobalDocument  = "document"

	// JavaScript WebSocket event handlers
	jsEventOnOpen = "onopen"

	// JavaScript WebSocket properties
	jsPropertyReadyState      = "readyState"
	jsPropertyOnLine          = "onLine"
	jsPropertyVisibilityState = "visibilityState"
)

// Config configures browser websocket dialing behavior.
type Config struct {
	// Subprotocols configures optional websocket subprotocol negotiation.
	Subprotocols []string
}

// newBrowserWebSocketDialer creates a custom gRPC dialer that establishes a WebSocket
// connection in the browser environment and prepares it for gRPC communication.
//
// This function returns a dialer function that:
// 1. Creates a browser WebSocket connection to the specified URL
// 2. Configures it for binary communication (required for gRPC)
// 3. Waits for the connection to establish or fail
// 4. Returns a net.Conn adapter that gRPC can use
//
// The dialer handles the asynchronous nature of browser WebSocket connections
// by using channels to synchronize the connection establishment.
//
// Parameters:
//   - webSocketURL: The WebSocket URL to connect to (e.g., "ws://localhost:8080/grpc")
//
// Returns:
//   - A dialer function compatible with grpc.WithContextDialer
func newBrowserWebSocketDialer(parseWebSocketURL string) func(context.Context, string) (net.Conn, error) {
	return newBrowserWebSocketDialerWithConfig(parseWebSocketURL, Config{})
}

// newBrowserWebSocketDialerWithConfig creates a browser websocket dialer with additive options.
func newBrowserWebSocketDialerWithConfig(parseWebSocketURL string, parseConfig Config) func(context.Context, string) (net.Conn, error) {
	return func(parseDialContext context.Context, parseGrpcTargetAddress string) (net.Conn, error) {
		// Access the browser's WebSocket constructor from the JavaScript global scope.
		// This is the standard browser WebSocket API.
		parseBrowserWebSocketConstructor := js.Global().Get(jsGlobalWebSocket)
		if !parseBrowserWebSocketConstructor.Truthy() {
			// WebSocket API not available - this shouldn't happen in modern browsers
			// but could occur in non-browser WASM environments
			return nil, status.Errorf(codes.Unavailable, "WASM: WebSocket not available in this environment")
		}

		// Create a new browser WebSocket instance with the provided URL.
		// This initiates the WebSocket handshake in the background.
		parseBrowserWebSocket := buildBrowserWebSocket(parseBrowserWebSocketConstructor, parseWebSocketURL, parseConfig)

		// Configure the WebSocket to use ArrayBuffer for binary data.
		// gRPC requires binary communication, so we must set binaryType to 'arraybuffer'.
		// The alternative 'blob' type would be incompatible with our data handling.
		parseBrowserWebSocket.Set(jsPropertyBinaryType, jsBinaryTypeArrayBuffer)

		// Set up temporary connection handlers used only while dialing.
		// NewWebSocketConn will install the steady-state handlers after open.
		parseConnectionOpenChannel := make(chan struct{}, 1)
		parseConnectionErrorChannel := make(chan error, 1)
		parseOpenHandler := js.FuncOf(func(parseThis js.Value, parseEventArgs []js.Value) interface{} {
			// Dial only needs to know that open happened at least once.
			select {
			case parseConnectionOpenChannel <- struct{}{}:
			default:
			}
			return nil
		})
		defer parseOpenHandler.Release()
		parseBrowserWebSocket.Set(jsEventOnOpen, parseOpenHandler)

		parseErrorHandler := js.FuncOf(func(parseThis js.Value, parseEventArgs []js.Value) interface{} {
			// During handshake we only surface availability-level failures.
			select {
			case parseConnectionErrorChannel <- status.Errorf(codes.Unavailable, "WASM: WebSocket error during connection setup (%s)", buildDialerEnvironmentState()):
			default:
			}
			return nil
		})
		defer parseErrorHandler.Release()
		parseBrowserWebSocket.Set(jsEventOnError, parseErrorHandler)

		// Wait for one of three outcomes:
		// 1. Connection opens successfully
		// 2. Connection fails with an error
		// 3. Context is cancelled (timeout or explicit cancellation)
		select {
		case <-parseConnectionOpenChannel:
			// Connection is now open.
		case parseErr := <-parseConnectionErrorChannel:
			// Connection failed during the handshake
			parseBrowserWebSocket.Set(jsEventOnOpen, js.Null())
			parseBrowserWebSocket.Set(jsEventOnError, js.Null())
			parseBrowserWebSocket.Call(jsMethodClose)
			return nil, parseErr
		case <-parseDialContext.Done():
			// The dialing context was cancelled or timed out before connection completed.
			// Always close on cancellation to avoid orphaning sockets if the browser
			// transitions to OPEN concurrently with context cancellation.
			parseBrowserWebSocket.Set(jsEventOnOpen, js.Null())
			parseBrowserWebSocket.Set(jsEventOnError, js.Null())
			parseBrowserWebSocket.Call(jsMethodClose)
			// Return the context error (DeadlineExceeded or Canceled)
			return nil, parseDialContext.Err()
		}

		// Remove temporary handlers before NewWebSocketConn installs steady-state handlers.
		// This prevents handler overlap where dial-time callbacks shadow runtime callbacks.
		parseBrowserWebSocket.Set(jsEventOnOpen, js.Null())
		parseBrowserWebSocket.Set(jsEventOnError, js.Null())

		// Return the net.Conn adapter so gRPC can send HTTP/2 frames over this socket.
		return NewWebSocketConn(parseBrowserWebSocket), nil
	}
}

// buildBrowserWebSocket constructs the browser WebSocket with optional subprotocols.
func buildBrowserWebSocket(parseConstructor js.Value, parseWebSocketURL string, parseConfig Config) js.Value {
	if len(parseConfig.Subprotocols) == 0 {
		return parseConstructor.New(parseWebSocketURL)
	}

	parseProtocols := make([]interface{}, 0, len(parseConfig.Subprotocols))
	for _, parseSubprotocol := range parseConfig.Subprotocols {
		parseProtocols = append(parseProtocols, parseSubprotocol)
	}
	return parseConstructor.New(parseWebSocketURL, js.ValueOf(parseProtocols))
}

// isDialerBrowserOnline reports browser online state when navigator metadata is available.
func isDialerBrowserOnline() bool {
	parseNavigator := js.Global().Get(jsGlobalNavigator)
	if !parseNavigator.Truthy() {
		return true
	}

	parseOnLine := parseNavigator.Get(jsPropertyOnLine)
	if parseOnLine.Type() != js.TypeBoolean {
		return true
	}
	return parseOnLine.Bool()
}

// getDialerVisibilityState reports document visibility state when available.
func getDialerVisibilityState() string {
	parseDocument := js.Global().Get(jsGlobalDocument)
	if !parseDocument.Truthy() {
		return "visible"
	}

	parseVisibilityState := parseDocument.Get(jsPropertyVisibilityState)
	if parseVisibilityState.Type() != js.TypeString {
		return "visible"
	}
	return parseVisibilityState.String()
}

// buildDialerEnvironmentState returns a compact online and visibility snapshot for diagnostics.
func buildDialerEnvironmentState() string {
	parseOnlineState := "online"
	if !isDialerBrowserOnline() {
		parseOnlineState = "offline"
	}
	return parseOnlineState + ",visibility=" + getDialerVisibilityState()
}

// New creates a grpc.DialOption that can be used to dial a gRPC server over a WebSocket
// from a WebAssembly environment (browser).
//
// This is the WASM equivalent of bridge.DialOption for browser-based clients.
// It configures gRPC to use browser WebSocket APIs instead of traditional TCP sockets,
// which are not available in browser environments.
//
// The returned DialOption should be passed to grpc.Dial() or grpc.DialContext() along
// with other required options like credentials.
//
// Parameters:
//   - webSocketURL: The full WebSocket URL to connect to, including scheme (ws:// or wss://),
//     host, port, and path (e.g., "ws://localhost:8080/grpc" or "wss://api.example.com/grpc")
//
// Returns:
//   - grpc.DialOption: A dial option that configures gRPC to use browser WebSocket transport
//
// Example:
//
//	ctx := context.Background()
//	conn, err := grpc.DialContext(
//	    ctx,
//	    "localhost:8080", // This target is ignored; WebSocket URL is used instead
//	    dialer.New("ws://localhost:8080/grpc"),
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//
//	client := proto.NewYourServiceClient(conn)
//	// Use the client normally
//
// Note: This function is only available in WASM builds (//go:build js && wasm).
// For non-WASM Go code, use bridge.DialOption instead.
func New(parseWebSocketURL string) grpc.DialOption {
	return grpc.WithContextDialer(newBrowserWebSocketDialer(parseWebSocketURL))
}

// NewWithConfig creates a grpc.DialOption with additive browser websocket dialing options.
func NewWithConfig(parseWebSocketURL string, parseConfig Config) grpc.DialOption {
	return grpc.WithContextDialer(newBrowserWebSocketDialerWithConfig(parseWebSocketURL, parseConfig))
}
