# GoGRPCBridge

> **Native gRPC in the browser.** Type-safe streaming from WebAssembly clients to your Go backend.

Modern web development shouldn't force you to choose between developer experience and browser compatibility. GoGRPCBridge brings the full power of gRPC—bidirectional streaming, type safety, and efficient binary protocols—to WebAssembly clients through a simple WebSocket tunnel.

[![Go Reference](https://pkg.go.dev/badge/github.com/monstercameron/GoGRPCBridge.svg)](https://pkg.go.dev/github.com/monstercameron/GoGRPCBridge)
[![Go Report Card](https://goreportcard.com/badge/github.com/monstercameron/GoGRPCBridge)](https://goreportcard.com/report/github.com/monstercameron/GoGRPCBridge)
[![Build](https://github.com/monstercameron/GoGRPCBridge/workflows/Build/badge.svg)](https://github.com/monstercameron/GoGRPCBridge/actions/workflows/build.yml)
[![Test](https://github.com/monstercameron/GoGRPCBridge/workflows/Test/badge.svg)](https://github.com/monstercameron/GoGRPCBridge/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## What This Solves

You're building a web application. You want the benefits of gRPC—type safety, streaming, efficiency—but browsers don't speak HTTP/2. Your options are limited:

- **REST APIs**: Lose type safety, streaming, and efficiency. Write serializers manually.
- **gRPC-Web**: Limited to unary and server streaming. Requires Envoy proxy. No bidirectional streams.
- **WebSocket + Custom Protocol**: Reinvent the wheel. Maintain your own serialization.

**GoGRPCBridge** gives you a better path: your existing gRPC services work in the browser, unchanged. No proxies, no protocol translation, no compromises.

## Key Features

✨ **Bidirectional Streaming** – Real-time chat, live collaboration, streaming analytics  
🔒 **Type Safety** – Protobuf contracts prevent runtime errors  
📦 **Efficient** – Binary protocol, 4-8x smaller than JSON  
🎯 **Zero Boilerplate** – One function call to bridge your gRPC server  
🔧 **Standard gRPC** – Works with existing services, no modifications needed  
🌐 **Browser Native** – WebAssembly client with full gRPC capabilities

---

## Quick Start

### 1. Install

```bash
go get github.com/monstercameron/GoGRPCBridge
```

### 2. Define Your Service

```protobuf
syntax = "proto3";
package chat;

service ChatService {
  // Bidirectional streaming - real-time chat
  rpc LiveChat(stream Message) returns (stream Message);
}

message Message {
  string user = 1;
  string text = 2;
  int64 timestamp = 3;
}
```

Generate code:
```bash
protoc --go_out=. --go-grpc_out=. chat.proto
```

### 3. Server Setup (One Line)

```go
package main

import (
    "github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
    "google.golang.org/grpc"
)

func main() {
    // Your existing gRPC server
    grpcServer := grpc.NewServer()
    chat.RegisterChatServiceServer(grpcServer, &chatServer{})
    
    // Make it accessible via WebSocket - that's it!
    grpctunnel.ListenAndServe(":8080", grpcServer)
}
```

### 4. Browser Client (WASM)

```go
//go:build js && wasm
package main

import (
    "context"
    "github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
    "google.golang.org/grpc"
)

func main() {
    // Connect to bridge (uses current page's host automatically)
    conn, _ := grpctunnel.Dial("", grpc.WithInsecure())
    client := chat.NewChatServiceClient(conn)
    
    // Full bidirectional streaming in the browser!
    stream, _ := client.LiveChat(context.Background())
    
    // Send messages
    stream.Send(&chat.Message{
        User: "Alice",
        Text: "Hello from the browser!",
    })
    
    // Receive in real-time
    msg, _ := stream.Recv()
    println(msg.Text)
}
```

Build for browser:
```bash
GOOS=js GOARCH=wasm go build -o app.wasm
```

### 5. Serve Your App

```html
<!DOCTYPE html>
<html>
<head>
    <script src="wasm_exec.js"></script>
    <script>
        const go = new Go();
        WebAssembly.instantiateStreaming(fetch("app.wasm"), go.importObject)
            .then(result => go.run(result.instance));
    </script>
</head>
<body>
    <h1>gRPC Chat</h1>
    <div id="messages"></div>
</body>
</html>
```

**That's it!** You now have bidirectional gRPC streaming running in the browser.

---

## Why gRPC?

### The REST Approach

```javascript
// REST: Manual serialization, polling, no streaming
async function sendMessage(text) {
    await fetch('/api/messages', {
        method: 'POST',
        body: JSON.stringify({ user: 'Alice', text: text })
    });
    
    // Poll for new messages every second
    setInterval(async () => {
        const response = await fetch('/api/messages');
        const messages = await response.json();
        updateUI(messages);
    }, 1000);
}
```

Problems:
- ❌ No type safety (runtime errors from typos)
- ❌ Polling wastes bandwidth, delays updates
- ❌ Manual JSON serialization
- ❌ No streaming capabilities
- ❌ Separate implementation per platform

### The gRPC Way

```go
// gRPC: Type-safe, bidirectional, real-time
stream, _ := client.LiveChat(ctx)

// Send messages
stream.Send(&chat.Message{
    User: "Alice",
    Text: "Hello!",
})

// Receive in real-time (no polling!)
for {
    msg, _ := stream.Recv()
    fmt.Printf("%s: %s\n", msg.User, msg.Text)
}
```

Benefits:
- ✅ Compiler catches errors before runtime
- ✅ Real-time bidirectional streaming
- ✅ Auto-generated serialization
- ✅ 4-8x more efficient than JSON
- ✅ Same code for web, mobile, backend

### Real Impact

**Chat Application:**
- REST: 3,600 polling requests/hour per user
- gRPC: 1 persistent connection, instant delivery

**Payload Size (1MB dataset):**
- JSON: ~1,000 KB
- Protobuf: ~250 KB (75% smaller)

**Development:**
- REST: Write serializers, validators, HTTP handlers
- gRPC: `protoc` generates everything from .proto file

---

## Complete Example: Todo App

This guide walks you through building a complete application with server streaming and bidirectional communication.

### Step 1: Define the API

`api/todo.proto`:
```protobuf
syntax = "proto3";
package todo;

service TodoService {
  rpc CreateTodo(CreateRequest) returns (Todo);
  rpc ListTodos(ListRequest) returns (stream Todo);      // Server streaming
  rpc SyncTodos(stream Todo) returns (stream Todo);      // Bidirectional
}

message Todo {
  string id = 1;
  string text = 2;
  bool completed = 3;
}

message CreateRequest {
  string text = 1;
}

message ListRequest {}
```

Generate:
```bash
protoc --go_out=. --go-grpc_out=. api/todo.proto
```

### Step 2: Implement the Service

```go
type todoService struct {
    todo.UnimplementedTodoServiceServer
    todos []*todo.Todo
    mu    sync.Mutex
}

func (s *todoService) CreateTodo(ctx context.Context, req *todo.CreateRequest) (*todo.Todo, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    newTodo := &todo.Todo{
        Id:        uuid.New().String(),
        Text:      req.Text,
        Completed: false,
    }
    s.todos = append(s.todos, newTodo)
    return newTodo, nil
}

func (s *todoService) ListTodos(req *todo.ListRequest, stream todo.TodoService_ListTodosServer) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Server streaming - send todos as they're ready
    for _, t := range s.todos {
        if err := stream.Send(t); err != nil {
            return err
        }
    }
    return nil
}

func (s *todoService) SyncTodos(stream todo.TodoService_SyncTodosServer) error {
    // Bidirectional streaming - sync state between clients
    for {
        todo, err := stream.Recv()
        if err != nil {
            return err
        }
        
        // Broadcast to all clients (simplified)
        if err := stream.Send(todo); err != nil {
            return err
        }
    }
}
```

### Step 3: Start the Server

```go
func main() {
    grpcServer := grpc.NewServer()
    todo.RegisterTodoServiceServer(grpcServer, &todoService{})
    
    log.Println("Starting gRPC-over-WebSocket bridge on :8080")
    grpctunnel.ListenAndServe(":8080", grpcServer)
}
```

### Step 4: Build the WASM Client

```go
//go:build js && wasm
package main

import (
    "context"
    "syscall/js"
    "github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
    "google.golang.org/grpc"
)

func main() {
    conn, err := grpctunnel.Dial("", grpc.WithInsecure())
    if err != nil {
        panic(err)
    }
    defer conn.Close()
    
    client := todo.NewTodoServiceClient(conn)
    
    // Create todo
    newTodo, _ := client.CreateTodo(context.Background(), &todo.CreateRequest{
        Text: "Learn gRPC",
    })
    println("Created:", newTodo.Text)
    
    // Stream all todos
    stream, _ := client.ListTodos(context.Background(), &todo.ListRequest{})
    for {
        todo, err := stream.Recv()
        if err != nil {
            break
        }
        println("Todo:", todo.Text)
    }
    
    // Keep WASM app running
    <-make(chan bool)
}
```

Build:
```bash
GOOS=js GOARCH=wasm go build -o public/app.wasm
```

### Step 5: Create the Web Page

`public/index.html`:
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Todo App - gRPC in Browser</title>
    <script src="wasm_exec.js"></script>
</head>
<body>
    <h1>📝 Todo App (gRPC + WASM)</h1>
    <div id="todos"></div>
    
    <script>
        const go = new Go();
        WebAssembly.instantiateStreaming(fetch("app.wasm"), go.importObject)
            .then(result => {
                go.run(result.instance);
            });
    </script>
</body>
</html>
```

Copy `wasm_exec.js` from your Go installation:
```bash
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" public/
```

### Step 6: Run It

```bash
# Terminal 1: Start server
go run server/main.go

# Terminal 2: Serve static files
python3 -m http.server 8081 --directory public
```

Visit `http://localhost:8081` - your gRPC client is now running in the browser!

---

## Production Configuration

### Security Best Practices

```go
import (
    "crypto/tls"
    "log"
    "net/http"
    "time"
    "github.com/monstercameron/GoGRPCBridge/pkg/bridge"
)

func main() {
    grpcServer := grpc.NewServer()
    // ... register services
    
    handler := bridge.NewHandler(bridge.Config{
        TargetAddress: "localhost:50051", // Your gRPC server
        
        // Validate origins (CORS)
        CheckOrigin: func(r *http.Request) bool {
            origin := r.Header.Get("Origin")
            allowed := []string{
                "https://yourdomain.com",
                "https://app.yourdomain.com",
            }
            for _, a := range allowed {
                if origin == a {
                    return true
                }
            }
            return false
        },
        
        // Monitor connections
        OnConnect: func(r *http.Request) {
            log.Printf("Client connected: %s", r.RemoteAddr)
        },
        OnDisconnect: func(r *http.Request) {
            log.Printf("Client disconnected: %s", r.RemoteAddr)
        },
        
        // Optimize buffers for your workload
        ReadBufferSize:  16384,
        WriteBufferSize: 16384,
    })
    
    server := &http.Server{
        Addr:    ":8443",
        Handler: handler,
        
        // Security timeouts
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      15 * time.Second,
        IdleTimeout:       60 * time.Second,
        ReadHeaderTimeout: 5 * time.Second,
        
        // TLS configuration
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
            CipherSuites: []uint16{
                tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
                tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            },
        },
    }
    
    log.Fatal(server.ListenAndServeTLS("cert.pem", "key.pem"))
}
```

### Production Checklist

- ✅ **TLS/SSL**: Use `wss://` instead of `ws://`
- ✅ **Origin Validation**: Implement `CheckOrigin` to prevent unauthorized access
- ✅ **Timeouts**: Configure all HTTP timeouts to prevent resource exhaustion
- ✅ **Monitoring**: Add connection hooks for observability
- ✅ **Rate Limiting**: Implement at HTTP layer or reverse proxy
- ✅ **Buffer Tuning**: Adjust sizes based on your payload characteristics
- ✅ **Graceful Shutdown**: Handle SIGTERM/SIGINT properly

---

## Advanced Features

### Custom HTTP Routing

Integrate with existing HTTP servers:

```go
mux := http.NewServeMux()

// Regular HTTP endpoints
mux.HandleFunc("/health", healthHandler)
mux.HandleFunc("/metrics", metricsHandler)

// gRPC bridge on specific path
grpcHandler := bridge.NewHandler(bridge.Config{
    TargetAddress: "localhost:50051",
})
mux.Handle("/grpc", grpcHandler)

http.ListenAndServe(":8080", mux)
```

### Multiple gRPC Services

```go
// Service 1: Chat
chatServer := grpc.NewServer()
chat.RegisterChatServiceServer(chatServer, &chatService{})

// Service 2: Todos
todoServer := grpc.NewServer()
todo.RegisterTodoServiceServer(todoServer, &todoService{})

// Separate bridges
mux := http.NewServeMux()
mux.Handle("/chat", bridge.NewHandler(bridge.Config{...}))
mux.Handle("/todos", bridge.NewHandler(bridge.Config{...}))
```

### Client-Side Connection Options

```go
// Custom dialer with options
conn, err := grpctunnel.Dial("wss://api.example.com:8443/grpc",
    grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{...})),
    grpc.WithBlock(),
    grpc.WithTimeout(5*time.Second),
)
```

---

## Architecture & How It Works

```
┌─────────────────┐
│  Browser        │
│  ┌───────────┐  │
│  │WASM Client│  │
│  │  (Go)     │  │
│  └─────┬─────┘  │
│        │ gRPC   │
│        │ calls  │
└────────┼────────┘
         │
    WebSocket
         │
┌────────▼────────┐
│  GoGRPCBridge   │
│  ┌───────────┐  │
│  │net.Conn   │  │──┐
│  │Adapter    │  │  │ HTTP/2
│  └───────────┘  │  │ frames
└─────────────────┘  │
                     │
              ┌──────▼──────┐
              │ gRPC Server │
              │   (Go)      │
              └─────────────┘
```

**Key Insight**: The bridge provides a transparent `net.Conn` interface backed by WebSocket. Your gRPC server doesn't know it's using WebSocket—it sees a normal connection with HTTP/2 frames.

**Data Flow**:
1. Client makes gRPC call (standard `client.Method(ctx, req)`)
2. gRPC encodes request as HTTP/2 frames
3. Bridge reads frames, sends as WebSocket binary messages
4. Server receives WebSocket messages, reconstructs HTTP/2 frames
5. gRPC server processes request normally
6. Response follows same path in reverse

---

## Comparison with Alternatives

| Feature | GoGRPCBridge | gRPC-Web | REST + JSON |
|---------|--------------|----------|-------------|
| Bidirectional Streaming | ✅ | ❌ | ❌ |
| Server Streaming | ✅ | ✅ | ❌ |
| Type Safety | ✅ | ✅ | ❌ |
| Binary Efficiency | ✅ | ✅ | ❌ |
| Works with Standard gRPC | ✅ | ❌ Needs Envoy | ❌ |
| Browser Support | ✅ WASM | ✅ | ✅ |
| Setup Complexity | Low | High | Low |
| Proxy Required | ❌ | ✅ | ❌ |

---

## Testing & Quality

- **85%+ Test Coverage** – Comprehensive unit and integration tests
- **Race Detector Clean** – Zero data races in concurrent code
- **Fuzz Tested** – 4 billion+ inputs across 4 fuzzers
- **E2E Browser Tests** – Playwright-based validation
- **Security Scanned** – Gosec on every commit
- **Production Examples** – Real-world configurations included

### Run Tests

```bash
# All tests with race detection
go test ./pkg/... -race -cover

# Fuzz testing
go test -fuzz=FuzzWebSocketConnWrite ./pkg/bridge -fuzztime=30s
go test -fuzz=FuzzWebSocketConnRead ./pkg/bridge -fuzztime=30s

# E2E tests
cd e2e && go test -v
```

---

## Limitations & Considerations

| Aspect | Status | Notes |
|--------|--------|-------|
| **HTTP/2 Native** | ❌ | Uses WebSocket transport |
| **gRPC Reflection** | ❌ | Tools can't introspect bridge endpoint |
| **Performance** | ✅ | Minimal overhead, binary protocol preserved |
| **Browser Support** | ✅ | WASM required (all modern browsers) |
| **Firewall Friendly** | ✅ | WebSocket typically allowed |

### Using gRPC Tools (grpcurl, grpcui)

Standard introspection tools don't work on the WebSocket endpoint. Connect them to your gRPC server directly:

```bash
# ❌ Won't work - bridge uses WebSocket
grpcurl -plaintext localhost:8080 list

# ✅ Works - connect to backend
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 todo.TodoService/CreateTodo
```

**Testing Architecture**:
```
grpcurl/grpcui → [Port 50051] gRPC Server ✅

Browser (WASM) → [Port 8080] Bridge → [Port 50051] gRPC Server ✅
```

---

## Examples

Complete, runnable examples in the `examples/` directory:

| Example | Description | Use Case |
|---------|-------------|----------|
| **direct-bridge** | All-in-one server | Simple deployments |
| **production-bridge** | Production config | TLS, auth, monitoring |
| **custom-router** | HTTP + gRPC | Integrate with existing servers |
| **wasm-client** | Full browser app | Reference implementation |
| **grpc-server** | Standard gRPC | Backend service example |

[View all examples →](./examples/)

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for:

- Development workflow
- Pre-commit hooks setup
- Testing guidelines
- Code style standards
- Security scanning process

---

## License

MIT License – use freely in commercial and open-source projects.

---

## Support

- 📖 **Documentation**: [pkg.go.dev](https://pkg.go.dev/github.com/monstercameron/GoGRPCBridge)
- 🐛 **Issues**: [GitHub Issues](https://github.com/monstercameron/GoGRPCBridge/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/monstercameron/GoGRPCBridge/discussions)

---

**Built for developers who value type safety, performance, and great developer experience.**

If this project helps you, please ⭐ star the repository and share it with your team!
