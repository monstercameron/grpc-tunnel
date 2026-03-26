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
func WithOriginCheck(parseFn func(r *http.Request) bool) ServerOption {
	return func(parseO *serverOptions) {
		parseO.checkOrigin = parseFn
	}
}

// WithBufferSizes sets custom WebSocket buffer sizes.
func WithBufferSizes(parseRead, parseWrite int) ServerOption {
	return func(parseO *serverOptions) {
		parseO.readBufferSize = parseRead
		parseO.writeBufferSize = parseWrite
	}
}

// WithConnectHook sets a callback for when clients connect.
func WithConnectHook(parseFn func(r *http.Request)) ServerOption {
	return func(parseO *serverOptions) {
		parseO.onConnect = parseFn
	}
}

// WithDisconnectHook sets a callback for when clients disconnect.
func WithDisconnectHook(parseFn func(r *http.Request)) ServerOption {
	return func(parseO *serverOptions) {
		parseO.onDisconnect = parseFn
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
func Wrap(parseGrpcServer *grpc.Server, parseOpts ...ServerOption) http.Handler {
	parseOptions := &serverOptions{
		readBufferSize:  4096,
		writeBufferSize: 4096,
		checkOrigin:     func(parseR *http.Request) bool { return true },
	}

	for _, parseOpt := range parseOpts {
		parseOpt(parseOptions)
	}

	parseUpgrader := websocket.Upgrader{
		ReadBufferSize:  parseOptions.readBufferSize,
		WriteBufferSize: parseOptions.writeBufferSize,
		CheckOrigin:     parseOptions.checkOrigin,
	}

	return http.HandlerFunc(func(parseW http.ResponseWriter, parseR2 *http.Request) {
		// Upgrade to WebSocket
		parseWs, parseErr := parseUpgrader.Upgrade(parseW, parseR2, nil)
		if parseErr != nil {
			return
		}
		defer parseWs.Close()

		// Lifecycle hooks
		if parseOptions.onConnect != nil {
			parseOptions.onConnect(parseR2)
		}
		defer func() {
			if parseOptions.onDisconnect != nil {
				parseOptions.onDisconnect(parseR2)
			}
		}()

		// Wrap WebSocket as net.Conn
		parseConn := newWebSocketConn(parseWs)
		defer parseConn.Close()

		// Serve gRPC over HTTP/2 on the WebSocket connection
		parseH2Server := &http2.Server{}
		parseH2Server.ServeConn(parseConn, &http2.ServeConnOpts{
			Handler: h2c.NewHandler(parseGrpcServer, parseH2Server),
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
func Serve(parseListener net.Listener, parseGrpcServer *grpc.Server, parseOpts ...ServerOption) error {
	parseServer := &http.Server{
		Handler:      Wrap(parseGrpcServer, parseOpts...),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return parseServer.Serve(parseListener)
}

// ListenAndServe listens on the TCP network address and serves gRPC over WebSocket.
// This is the simplest one-liner for starting a gRPC-over-WebSocket server.
//
// Example:
//
//	grpcServer := grpc.NewServer()
//	proto.RegisterYourServiceServer(grpcServer, &yourImpl{})
//	grpctunnel.ListenAndServe(":8080", grpcServer)
func ListenAndServe(parseAddr string, parseGrpcServer *grpc.Server, parseOpts ...ServerOption) error {
	parseServer := &http.Server{
		Addr:         parseAddr,
		Handler:      Wrap(parseGrpcServer, parseOpts...),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return parseServer.ListenAndServe()
}
