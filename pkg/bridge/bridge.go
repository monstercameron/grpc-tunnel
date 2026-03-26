package bridge

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

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

func (defaultLogger) Printf(format string, parseV ...interface{}) {
	log.Printf(format, parseV...)
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
//	bridge := bridge.NewHandler(bridge.Config{
//	    TargetAddress: "localhost:50051",
//	})
//	http.Handle("/", bridge)
//	http.ListenAndServe(":8080", nil)
func NewHandler(parseCfg Config) *Handler {
	// Set defaults
	if parseCfg.ReadBufferSize == 0 {
		parseCfg.ReadBufferSize = 4096
	}
	if parseCfg.WriteBufferSize == 0 {
		parseCfg.WriteBufferSize = 4096
	}
	if parseCfg.CheckOrigin == nil {
		parseCfg.CheckOrigin = func(parseR *http.Request) bool { return true }
	}
	if parseCfg.Logger == nil {
		parseCfg.Logger = defaultLogger{}
	}

	parseTargetURL, _ := url.Parse("http://" + parseCfg.TargetAddress)

	parseH := &Handler{
		config: parseCfg,
		logger: parseCfg.Logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  parseCfg.ReadBufferSize,
			WriteBufferSize: parseCfg.WriteBufferSize,
			CheckOrigin:     parseCfg.CheckOrigin,
		},
	}

	// Create the reverse proxy
	parseH.proxy = &httputil.ReverseProxy{
		Director: func(parseReq *http.Request) {
			parseReq.URL.Scheme = parseTargetURL.Scheme
			parseReq.URL.Host = parseTargetURL.Host
			parseReq.Host = parseTargetURL.Host
		},
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(parseNetwork, parseAddr string, parseCfg2 *tls.Config) (net.Conn, error) {
				return net.Dial(parseNetwork, parseAddr)
			},
		},
		ErrorHandler: func(parseW http.ResponseWriter, parseR2 *http.Request, parseErr error) {
			parseH.logger.Printf("Proxy error: %v", parseErr)
			http.Error(parseW, parseErr.Error(), http.StatusBadGateway)
		},
	}

	return parseH
}

// ServeHTTP implements http.Handler. This is called for each incoming HTTP request.
func (parseH *Handler) ServeHTTP(parseW http.ResponseWriter, parseR *http.Request) {
	// Upgrade to WebSocket
	parseWs, parseErr := parseH.upgrader.Upgrade(parseW, parseR, nil)
	if parseErr != nil {
		parseH.logger.Printf("WebSocket upgrade failed: %v", parseErr)
		return
	}
	defer parseWs.Close()

	// Call OnConnect callback
	if parseH.config.OnConnect != nil {
		parseH.config.OnConnect(parseR)
	}
	defer func() {
		if parseH.config.OnDisconnect != nil {
			parseH.config.OnDisconnect(parseR)
		}
	}()

	// Wrap WebSocket as net.Conn
	parseConn := NewWebSocketConn(parseWs)
	defer parseConn.Close()

	// Serve HTTP/2 over the WebSocket connection
	parseHttp2Server := &http2.Server{}
	parseHttp2Server.ServeConn(parseConn, &http2.ServeConnOpts{
		Handler: h2c.NewHandler(parseH.proxy, parseHttp2Server),
	})
}
