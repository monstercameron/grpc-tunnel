package helpers

import (
	"net"
	"net/http"
	"time"

	"github.com/monstercameron/GoGRPCBridge/pkg/bridge"

	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

// ServerConfig holds configuration for the server-side bridge.
type ServerConfig struct {
	// GRPCServer is the gRPC server to serve over WebSocket
	GRPCServer *grpc.Server

	// CheckOrigin validates WebSocket origins (nil = allow all)
	CheckOrigin func(r *http.Request) bool

	// ReadBufferSize for WebSocket (default: 4096)
	ReadBufferSize int

	// WriteBufferSize for WebSocket (default: 4096)
	WriteBufferSize int

	// OnConnect is called when a client connects
	OnConnect func(r *http.Request)

	// OnDisconnect is called when a client disconnects
	OnDisconnect func(r *http.Request)
}

// ServeHandler creates an http.Handler that serves a gRPC server over WebSocket.
// Use this on the server side to accept gRPC connections via WebSocket.
//
// Example:
//
//	grpcServer := grpc.NewServer()
//	proto.RegisterYourServiceServer(grpcServer, &yourImpl{})
//
//	http.Handle("/grpc", bridge.ServeHandler(bridge.ServerConfig{
//	    GRPCServer: grpcServer,
//	}))
//	http.ListenAndServe(":8080", nil)
func ServeHandler(cfg ServerConfig) http.Handler {
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

	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin:     cfg.CheckOrigin,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		// Lifecycle hooks
		if cfg.OnConnect != nil {
			cfg.OnConnect(r)
		}
		defer func() {
			if cfg.OnDisconnect != nil {
				cfg.OnDisconnect(r)
			}
		}()

		// Wrap WebSocket as net.Conn
		conn := bridge.NewWebSocketConn(ws)
		defer conn.Close()

		// Serve gRPC over HTTP/2 on the WebSocket connection
		h2Server := &http2.Server{}
		h2Server.ServeConn(conn, &http2.ServeConnOpts{
			Handler: h2c.NewHandler(cfg.GRPCServer, h2Server),
		})
	})
}

// Serve accepts WebSocket connections and serves gRPC over them.
// This is a convenience wrapper around ServeHandler for simple cases.
//
// Example:
//
//	grpcServer := grpc.NewServer()
//	proto.RegisterYourServiceServer(grpcServer, &yourImpl{})
//
//	lis, _ := net.Listen("tcp", ":8080")
//	bridge.Serve(lis, grpcServer)
func Serve(listener net.Listener, grpcServer *grpc.Server) error {
	handler := ServeHandler(ServerConfig{
		GRPCServer: grpcServer,
	})
	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return server.Serve(listener)
}
