package helpers

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/monstercameron/GoGRPCBridge/pkg/bridge"

	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Config holds configuration options for the gRPC-over-WebSocket bridge.
type Config struct {
	// TargetAddress is the address of the backend gRPC server (e.g., "localhost:50051")
	TargetAddress string

	// CheckOrigin is called during the WebSocket upgrade to determine whether the origin is allowed.
	// If nil, all origins are allowed (development mode).
	CheckOrigin func(r *http.Request) bool

	// ReadBufferSize is the WebSocket read buffer size in bytes.
	// Default: 4096
	ReadBufferSize int

	// WriteBufferSize is the WebSocket write buffer size in bytes.
	// Default: 4096
	WriteBufferSize int

	// Logger is used for logging. If nil, the default logger is used.
	Logger Logger

	// OnConnect is called when a WebSocket connection is established.
	OnConnect func(r *http.Request)

	// OnDisconnect is called when a WebSocket connection ends.
	OnDisconnect func(r *http.Request)
}

// Logger interface for custom logging.
type Logger interface {
	Printf(format string, v ...interface{})
}

type defaultLogger struct{}

func (defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Handler is the gRPC-over-WebSocket bridge handler.
type Handler struct {
	config   Config
	upgrader websocket.Upgrader
	proxy    *httputil.ReverseProxy
	logger   Logger
}

// NewHandler creates a new gRPC-over-WebSocket bridge handler.
// This is the main entry point for integrating the bridge into your application.
//
// Example:
//
//	handler := helpers.NewHandler(helpers.Config{
//	    TargetAddress: "localhost:50051",
//	})
//	http.Handle("/", handler)
//	http.ListenAndServe(":8080", nil)
func NewHandler(cfg Config) *Handler {
	// Set defaults
	if cfg.ReadBufferSize == 0 {
		cfg.ReadBufferSize = 4096
	}
	if cfg.WriteBufferSize == 0 {
		cfg.WriteBufferSize = 4096
	}
	if cfg.CheckOrigin == nil {
		cfg.CheckOrigin = func(r *http.Request) bool { return true }
	}
	if cfg.Logger == nil {
		cfg.Logger = defaultLogger{}
	}

	targetURL, _ := url.Parse("http://" + cfg.TargetAddress)

	h := &Handler{
		config: cfg,
		logger: cfg.Logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.ReadBufferSize,
			WriteBufferSize: cfg.WriteBufferSize,
			CheckOrigin:     cfg.CheckOrigin,
		},
	}

	// Create the reverse proxy
	h.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host
		},
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			h.logger.Printf("Proxy error: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		},
	}

	return h
}

// ServeHTTP implements http.Handler. This is called for each incoming HTTP request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// Call OnConnect callback
	if h.config.OnConnect != nil {
		h.config.OnConnect(r)
	}
	defer func() {
		if h.config.OnDisconnect != nil {
			h.config.OnDisconnect(r)
		}
	}()

	// Wrap WebSocket as net.Conn
	conn := bridge.NewWebSocketConn(ws)
	defer conn.Close()

	// Serve HTTP/2 over the WebSocket connection
	http2Server := &http2.Server{}
	http2Server.ServeConn(conn, &http2.ServeConnOpts{
		Handler: h2c.NewHandler(h.proxy, http2Server),
	})
}
