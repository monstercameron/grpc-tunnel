# gRPC-over-WebSocket Bridge Library

A lightweight library that enables **direct gRPC communication over WebSocket**. Both client and server use this library to tunnel gRPC calls through WebSocket connections.

## Architecture

```
Client App                    Server App
    ↓                             ↓
gRPC Client                  gRPC Server
    ↓                             ↓
bridge.DialOption           bridge.ServeHandler
    ↓                             ↓
  WebSocket ←─────────────────→ WebSocket
  (HTTP/2 frames inside binary messages)
```

The bridge library provides transparent WebSocket transport for gRPC on both ends:
- **Client side**: gRPC writes HTTP/2 frames → WebSocket binary messages
- **Server side**: WebSocket binary messages → gRPC reads HTTP/2 frames

## Features

- 🎯 **Direct communication** - No proxy needed, client and server talk directly
- 🚀 **Zero overhead** - Just wraps WebSocket as `net.Conn`, gRPC handles everything
- 🔌 **Simple API** - One function for client, one for server
- 🛡️ **Full gRPC support** - Unary, streaming, metadata, all work perfectly
- 📦 **Minimal** - Only `gorilla/websocket` + `google.golang.org/grpc`

## Quick Start

### Server Side

```go
package main

import (
    "log"
    "net/http"
    
    "google.golang.org/grpc"
    "your-project/pkg/bridge"
    "your-project/proto"
)

func main() {
    // Create gRPC server
    grpcServer := grpc.NewServer()
    proto.RegisterYourServiceServer(grpcServer, &yourServiceImpl{})

    // Serve gRPC over WebSocket
    http.Handle("/grpc", bridge.ServeHandler(bridge.ServerConfig{
        GRPCServer: grpcServer,
    }))

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Client Side (Regular Go)

```go
package main

import (
    "context"
    
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "your-project/pkg/bridge"
    "your-project/proto"
)

func main() {
    // Connect via WebSocket
    conn, _ := grpc.Dial(
        "localhost:8080",
        bridge.DialOption("ws://localhost:8080/grpc"),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    defer conn.Close()

    // Use gRPC client normally
    client := proto.NewYourServiceClient(conn)
    resp, _ := client.YourMethod(context.Background(), &proto.Request{})
}
```

### Client Side (WASM)

```go
//go:build js && wasm

package main

import (
    "context"
    
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "your-project/pkg/wasm/dialer"
    "your-project/proto"
)

func main() {
    // WASM uses special dialer
    conn, _ := grpc.Dial(
        "localhost:8080",
        dialer.New("ws://localhost:8080/grpc"),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    defer conn.Close()

    // Rest is identical
    client := proto.NewYourServiceClient(conn)
    resp, _ := client.YourMethod(context.Background(), &proto.Request{})
}
```

## Server Configuration

```go
bridge.ServeHandler(bridge.ServerConfig{
    // Required: Your gRPC server
    GRPCServer: grpcServer,

    // Optional: Origin validation
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        return origin == "https://yourdomain.com"
    },

    // Optional: Buffer sizes (default: 4096)
    ReadBufferSize:  8192,
    WriteBufferSize: 8192,

    // Optional: Connection hooks
    OnConnect: func(r *http.Request) {
        log.Printf("Client connected: %s", r.RemoteAddr)
    },
    OnDisconnect: func(r *http.Request) {
        log.Printf("Client disconnected: %s", r.RemoteAddr)
    },
})
```

## How It Works

1. **Client**: gRPC generates HTTP/2 frames → bridge wraps WebSocket as `net.Conn` → frames sent as WebSocket binary messages
2. **Server**: WebSocket binary messages received → bridge wraps as `net.Conn` → gRPC reads HTTP/2 frames → processes request
3. **Response**: Same flow in reverse

The bridge library is completely transparent - it just makes WebSocket look like a regular network connection to gRPC.

## Use Cases

- ✅ Browser WASM clients calling gRPC servers
- ✅ Bypassing corporate firewalls (WebSocket on port 80/443)
- ✅ gRPC over restrictive networks that only allow HTTP/HTTPS
- ✅ Real-time bidirectional streaming from browsers

## Production Checklist

- ✅ Set `CheckOrigin` to validate request origins
- ✅ Use TLS (`wss://` and `https://`)
- ✅ Configure buffer sizes based on payload
- ✅ Add monitoring via lifecycle hooks
- ✅ Consider rate limiting at HTTP level

## License

Same as parent project.
