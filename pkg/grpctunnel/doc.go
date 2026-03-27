// Package grpctunnel provides a high-level API for running gRPC over WebSocket.
//
// Server-side entry points:
//   - BuildBridgeHandler and HandleBridgeMux for typed composition
//   - Wrap for middleware-style integration
//   - Serve and ListenAndServe for convenience startup
//
// Client-side entry points:
//   - BuildTunnelConn for typed connection setup
//   - Dial and DialContext for gRPC client connections over WebSocket
//   - WithTLS for configuring wss:// client dialing
//
// This package is the recommended public API for most users of GoGRPCBridge.
package grpctunnel
