package bridge

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c" // h2c for cleartext HTTP/2
)

// bridgeHandler implements http.Handler and acts as the server-side bridge.
// It upgrades WebSocket connections and layers HTTP/2 over them, then proxies
// gRPC requests to a target gRPC server.
type bridgeHandler struct {
	targetGRPCServerAddress string
	upgrader                websocket.Upgrader
}

// NewHandler creates a new http.Handler that serves as the gRPC-over-WebSocket bridge.
// targetGRPCServerAddress is the address of the backend gRPC server (e.g., "localhost:50051").
func NewHandler(targetGRPCServerAddress string) http.Handler {
	return &bridgeHandler{
		targetGRPCServerAddress: targetGRPCServerAddress,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development. Restrict in production.
				return true
			},
		},
	}
}

// ServeHTTP handles HTTP requests, upgrading them to WebSocket connections
// and then layering HTTP/2 over the WebSocket.
func (handler *bridgeHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection.
	webSocketConnection, upgradeError := handler.upgrader.Upgrade(responseWriter, request, nil)
	if upgradeError != nil {
		log.Printf("Bridge: Failed to upgrade to WebSocket: %v", upgradeError)
		return
	}
	defer webSocketConnection.Close()

	log.Println("Bridge: WebSocket connection established. Layering HTTP/2...")

	// Create a net.Conn adapter for the WebSocket connection.
	// This allows the HTTP/2 server to treat the WebSocket as a standard network connection.
	webSocketNetworkConnection := newWebSocketConn(webSocketConnection)
	defer webSocketNetworkConnection.Close()

	// Create a thin, transparent HTTP/2 reverse proxy to the gRPC server
	targetURL, _ := url.Parse("http://" + handler.targetGRPCServerAddress)
	
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host
			log.Printf("Bridge: Proxying %s %s", req.Method, req.URL.Path)
		},
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	// Create an HTTP/2 server that will serve over our WebSocket-backed net.Conn
	http2Server := &http2.Server{}

	// Serve HTTP/2 over the WebSocket connection using the reverse proxy
	http2Server.ServeConn(webSocketNetworkConnection, &http2.ServeConnOpts{
		Handler: h2c.NewHandler(reverseProxy, http2Server),
	})

	log.Println("Bridge: HTTP/2 over WebSocket session ended.")
}

// webSocketConn implements net.Conn for a gorilla/websocket.Conn.
// This allows the HTTP/2 server to operate over the WebSocket.
type webSocketConn struct {
	webSocketConnection *websocket.Conn
	readBuffer          []byte
	readDeadline        time.Time
	writeDeadline       time.Time
}

func newWebSocketConn(conn *websocket.Conn) net.Conn {
	return &webSocketConn{
		webSocketConnection: conn,
	}
}

func (webSocketNetworkConnection *webSocketConn) Read(buffer []byte) (int, error) {
	// If there's data in the buffer from a previous read, use it first.
	if len(webSocketNetworkConnection.readBuffer) > 0 {
		n := copy(buffer, webSocketNetworkConnection.readBuffer)
		webSocketNetworkConnection.readBuffer = webSocketNetworkConnection.readBuffer[n:]
		return n, nil
	}

	// Read a new message from the WebSocket.
	messageType, messageData, readError := webSocketNetworkConnection.webSocketConnection.ReadMessage()
	if readError != nil {
		return 0, readError
	}

	if messageType != websocket.BinaryMessage {
		log.Printf("Bridge: Received non-binary WebSocket message type: %d", messageType)
		return 0, net.ErrClosed // Or a more specific error
	}

	// Copy the received message into the provided buffer.
	// If the buffer is too small, store the remainder for the next Read call.
	n := copy(buffer, messageData)
	if n < len(messageData) {
		webSocketNetworkConnection.readBuffer = messageData[n:]
	} else {
		webSocketNetworkConnection.readBuffer = nil
	}
	return n, nil
}

func (webSocketNetworkConnection *webSocketConn) Write(data []byte) (int, error) {
	writeError := webSocketNetworkConnection.webSocketConnection.WriteMessage(websocket.BinaryMessage, data)
	if writeError != nil {
		return 0, writeError
	}
	return len(data), nil
}

func (webSocketNetworkConnection *webSocketConn) Close() error {
	return webSocketNetworkConnection.webSocketConnection.Close()
}

func (webSocketNetworkConnection *webSocketConn) LocalAddr() net.Addr {
	return webSocketNetworkConnection.webSocketConnection.LocalAddr()
}

func (webSocketNetworkConnection *webSocketConn) RemoteAddr() net.Addr {
	return webSocketNetworkConnection.webSocketConnection.RemoteAddr()
}

func (webSocketNetworkConnection *webSocketConn) SetDeadline(deadline time.Time) error {
	webSocketNetworkConnection.readDeadline = deadline
	webSocketNetworkConnection.writeDeadline = deadline
	// gorilla/websocket does not directly support deadlines on Read/WriteMessage.
	// This would require more complex logic with contexts and goroutines.
	// For now, we'll just store the deadline.
	return nil
}

func (webSocketNetworkConnection *webSocketConn) SetReadDeadline(deadline time.Time) error {
	webSocketNetworkConnection.readDeadline = deadline
	return nil
}

func (webSocketNetworkConnection *webSocketConn) SetWriteDeadline(deadline time.Time) error {
	webSocketNetworkConnection.writeDeadline = deadline
	return nil
}
