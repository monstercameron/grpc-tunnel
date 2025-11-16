//go:build !js && !wasm

package grpctunnel

import (
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

// ServerOption configures the WebSocket server behavior.
type ServerOption func(*serverOptions)

type serverOptions struct {
	checkOrigin     func(r *http.Request) bool
	readBufferSize  int
	writeBufferSize int
	onConnect       func(r *http.Request)
	onDisconnect    func(r *http.Request)
}

// WithOriginCheck sets a custom origin validation function.
// If not set, all origins are allowed (development mode).
func WithOriginCheck(fn func(r *http.Request) bool) ServerOption {
	return func(o *serverOptions) {
		o.checkOrigin = fn
	}
}

// WithBufferSizes sets custom WebSocket buffer sizes.
func WithBufferSizes(read, write int) ServerOption {
	return func(o *serverOptions) {
		o.readBufferSize = read
		o.writeBufferSize = write
	}
}

// WithConnectHook sets a callback for when clients connect.
func WithConnectHook(fn func(r *http.Request)) ServerOption {
	return func(o *serverOptions) {
		o.onConnect = fn
	}
}

// WithDisconnectHook sets a callback for when clients disconnect.
func WithDisconnectHook(fn func(r *http.Request)) ServerOption {
	return func(o *serverOptions) {
		o.onDisconnect = fn
	}
}

// Wrap creates an http.Handler that serves a gRPC server over WebSocket.
// This is the middleware-style API for integrating WebSocket transport.
//
// Example:
//
//	grpcServer := grpc.NewServer()
//	proto.RegisterYourServiceServer(grpcServer, &yourImpl{})
//	http.ListenAndServe(":8080", grpctunnel.Wrap(grpcServer))
func Wrap(grpcServer *grpc.Server, opts ...ServerOption) http.Handler {
	options := &serverOptions{
		readBufferSize:  4096,
		writeBufferSize: 4096,
		checkOrigin:     func(r *http.Request) bool { return true },
	}

	for _, opt := range opts {
		opt(options)
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  options.readBufferSize,
		WriteBufferSize: options.writeBufferSize,
		CheckOrigin:     options.checkOrigin,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		// Lifecycle hooks
		if options.onConnect != nil {
			options.onConnect(r)
		}
		defer func() {
			if options.onDisconnect != nil {
				options.onDisconnect(r)
			}
		}()

		// Wrap WebSocket as net.Conn
		conn := newWebSocketConn(ws)
		defer conn.Close()

		// Serve gRPC over HTTP/2 on the WebSocket connection
		h2Server := &http2.Server{}
		h2Server.ServeConn(conn, &http2.ServeConnOpts{
			Handler: h2c.NewHandler(grpcServer, h2Server),
		})
	})
}

// Serve accepts connections on the listener and serves gRPC over WebSocket.
// This is a convenience wrapper for simple server setup.
//
// Example:
//
//	grpcServer := grpc.NewServer()
//	proto.RegisterYourServiceServer(grpcServer, &yourImpl{})
//
//	lis, _ := net.Listen("tcp", ":8080")
//	grpctunnel.Serve(lis, grpcServer)
func Serve(listener net.Listener, grpcServer *grpc.Server, opts ...ServerOption) error {
	server := &http.Server{
		Handler:      Wrap(grpcServer, opts...),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return server.Serve(listener)
}

// ListenAndServe listens on the TCP network address and serves gRPC over WebSocket.
// This is the simplest one-liner for starting a gRPC-over-WebSocket server.
//
// Example:
//
//	grpcServer := grpc.NewServer()
//	proto.RegisterYourServiceServer(grpcServer, &yourImpl{})
//	grpctunnel.ListenAndServe(":8080", grpcServer)
func ListenAndServe(addr string, grpcServer *grpc.Server, opts ...ServerOption) error {
	server := &http.Server{
		Addr:         addr,
		Handler:      Wrap(grpcServer, opts...),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return server.ListenAndServe()
}
