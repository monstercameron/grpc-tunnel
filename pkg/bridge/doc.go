// Package bridge provides low-level gRPC-over-WebSocket primitives.
//
// This package exposes:
//   - Handler-based server bridging via NewHandler
//   - net.Conn adaptation for gorilla/websocket via NewWebSocketConn
//   - a client dial helper via DialOption
//
// For most applications, prefer the higher-level pkg/grpctunnel package.
// Use this package when you need direct control over bridge handler wiring.
package bridge
