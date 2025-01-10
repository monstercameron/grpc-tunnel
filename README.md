# gRPC-over-WebSocket Chat Demo

This project demonstrates how to tunnel **native gRPC** calls over a **WebSocket** connection in Go, with a browser client compiled to **WebAssembly**. Instead of sending plain text over the WebSocket, the client and server **serialize gRPC messages as Protobuf**, send them as **binary frames**, and handle them using standard gRPC server and client logic.

## Table of Contents

1. [Overview](#overview)  
2. [Why gRPC-over-WebSocket?](#why-grpc-over-websocket)  
3. [Architecture](#architecture)  
4. [Setup and Usage](#setup-and-usage)  
5. [Project Structure](#project-structure)  
6. [Theory and Details](#theory-and-details)  
   - [1. Tunneling Native gRPC in Browsers](#1-tunneling-native-grpc-in-browsers)  
   - [2. Binary Framing and Proto Serialization](#2-binary-framing-and-proto-serialization)  
   - [3. WASM Limitations](#3-wasm-limitations)  
   - [4. Performance Considerations](#4-performance-considerations)  
   - [5. Debugging Tips](#5-debugging-tips)  
7. [Limitations](#limitations)  
8. [License](#license)

---

## Overview

- **Server**:
  - A Go process that **hosts a gRPC server** on `:50051`.
  - Also starts an **HTTP server** on `:8080` that:
    - Serves static files (including the WebAssembly client).
    - Listens for **WebSocket** connections at `ws://localhost:8080/ws`.
  - When the WebSocket server receives data, it **unmarshals** that data as a gRPC request (using Protobuf), forwards it to the gRPC server, and returns the serialized gRPC response over the WebSocket.

- **Client**:
  - A **WebAssembly** application (compiled from Go) loaded in the browser.
  - It opens a **WebSocket** to the server and sends binary-encoded gRPC requests (Protobuf) over the WebSocket.
  - Receives binary gRPC responses over that same WebSocket and **unmarshals** them into Protobuf messages.

This allows full **gRPC** communication to occur from within a web browser using a **raw WebSocket** approach, avoiding the usual limitations of gRPC in browsers (which otherwise typically requires `gRPC-Web` or plain HTTP/1.1 fallback).

---

## Why gRPC-over-WebSocket?

- **Browser Limitations**: Native gRPC typically requires HTTP/2 with special framing not fully supported in most browsers. By tunneling over a WebSocket, we bypass these HTTP/2 constraints in the browser environment.
- **Firewall/Proxy Circumvention**: Some networks block or degrade HTTP/2 or unknown protocols, but may allow WebSockets. This approach can help in such environments.
- **End-to-End gRPC**: Instead of rewriting your server logic to use `gRPC-Web` or JSON, you can retain **native gRPC** on the server side, and your browser client can send actual gRPC messages over a WebSocket tunnel.
- **Binary Efficiency**: We still reap the benefits of Protobuf’s compact binary representation—rather than sending JSON or text-based messages.

---

## Architecture

1. **Go gRPC Server**:  
   - Implements your proto-defined service (`EchoService` in our demo).
   - Listens on port `50051`.

2. **WebSocket Tunnel**:  
   - Runs inside the same process.
   - Listens on `:8080` (HTTP).
   - When a client connects to `ws://localhost:8080/ws`, the server upgrades to a WebSocket connection.
   - Each **binary message** is treated as a raw gRPC request (`EchoRequest` in this example).
   - The server calls the local gRPC service and sends back the serialized response (`EchoResponse`) over the WebSocket.

3. **WebAssembly Client**:  
   - A Go program compiled to WASM that runs in the browser.
   - Opens a **WebSocket** to `ws://localhost:8080/ws`.
   - Marshals an `EchoRequest` (Protobuf) and sends it as **binary data**.
   - Receives the `EchoResponse` as binary, unmarshals it, and displays the result.

---

## Setup and Usage

1. **Install Dependencies**  
   - You need Go 1.18+ (or higher) with WASM support, plus the `protoc` compiler.
   - Install `protoc-gen-go` and `protoc-gen-go-grpc`:
     ```bash
     go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
     go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
     ```

2. **Compile Protobuf**  
   ```bash
   protoc --go_out=. --go-grpc_out=. myservice.proto
   ```
   This generates `myservice.pb.go` and `myservice_grpc.pb.go`.

3. **Build**  
   Use the included `build.sh` (or your own script) to build the **server** (for ARM macOS, for example) and the **client** (for WASM):
   ```bash
   ./build.sh
   ```

4. **Run the Server**  
   ```bash
   ./bin/server
   ```
   You should see logs indicating:
   - “gRPC server listening on :50051”
   - “HTTP + WebSocket server listening on :8080”

5. **Open the Browser**  
   - Navigate to `http://localhost:8080` (it serves `./public/index.html`).
   - Open dev tools to watch console logs.

6. **Send Messages**  
   - In the chat input, type a message and hit **Send**.
   - The WASM client will:
     1. Serialize your message as `EchoRequest` Protobuf.
     2. Send it as a **binary** WebSocket frame.
   - The server:
     1. Unmarshals the request, calls `EchoService.Echo`.
     2. Marshals `EchoResponse` as binary, sends back over WebSocket.
   - The WASM client logs the response, and the UI can display it.

---

## Project Structure

A possible layout:

```
.
├── build.sh
├── go.mod
├── myservice.proto
├── server/
│   └── main.go           # Combined gRPC + WebSocket + File server
├── client/
│   └── main.go           # WASM client code
├── public/
│   ├── index.html        # Simple chat UI
│   ├── script.js         # JS that loads the WASM binary
│   ├── client.wasm       # Compiled WASM output
│   └── wasm_exec.js      # Copied from Go’s misc/wasm directory
└── ...
```

---

## Theory and Details

### 1. Tunneling Native gRPC in Browsers

Typically, **browsers** cannot open raw HTTP/2 connections with custom frames, which gRPC relies upon. Many projects solve this by using **gRPC-Web** or by falling back to JSON/REST. Here, we directly **tunnel** the gRPC frames via WebSockets:

- The browser can open a WebSocket to `ws://` or `wss://` endpoints.
- We treat each WebSocket message as a raw gRPC request or response (encoded with Protobuf).
- On the server side, we **unmarshal** the Protobuf message and pass it to the local gRPC server.
- The gRPC server replies, and we **marshal** that response back to the WebSocket.

This sidesteps the **HTTP/2** limitation in browsers by leveraging the universal **WebSocket** protocol as a transport layer.

### 2. Binary Framing and Proto Serialization

**Protobuf** is used for serialization:

1. **Marshal** the `EchoRequest` into a `[]byte` in the WASM client.
2. **Send** that byte slice as a WebSocket **binary** message (`websocket.BinaryMessage`).
3. **Unmarshal** it on the server into the Go struct.  
4. Repeat for the response.

There’s no plaintext or JSON. This ensures minimal overhead and preserves type safety at both ends.

### 3. WASM Limitations

- **Networking**: In WASM, you cannot directly use `net.Dial` or standard sockets. Instead, you rely on the browser’s APIs (like `WebSocket` or fetch). 
- **syscall/js**: Go’s WASM support provides `syscall/js`, which you use to call JavaScript. That is how we create the WebSocket in the client code.

### 4. Performance Considerations

- **WebSocket Overhead**: The frames add some overhead compared to a raw TCP or HTTP/2 connection. In many real-world scenarios, the overhead is negligible, but for extremely high throughput or low-latency demands, evaluate carefully.
- **Concurrency**: The Go server can handle many WebSocket connections concurrently, but be mindful of memory usage if you expect thousands of concurrent connections.
- **Serialization**: Protobuf is efficient, but each message still requires (un)marshaling. In extremely tight loops, measure performance.

### 5. Debugging Tips

- **Browser DevTools**: You can see WebSocket frames in the Network tab. They will appear as binary frames. Tools like Wireshark can dissect them only if you have .proto definitions integrated (not typical).
- **Console Logs**: The provided code includes many `log.Printf(...)` statements in both server and WASM client. Watch both the terminal and the browser console for messages.
- **Error Handling**: On the server, watch out for “WebSocket read error” or “unmarshal error.” On the client, watch for “failed to unmarshal.” This often indicates a mismatch in the `.proto` schema or a simple data corruption scenario.

---

## Limitations

1. **No Standard HTTP/2**: This solution is effectively a **custom transport**. It does not use standard gRPC over HTTP/2 in the browser, which might break conventional proxies that expect HTTP/2 frames. 
2. **Lack of Official Tooling**: Tools like `grpcurl` or standard gRPC reflection do not directly apply to your WebSocket endpoint. 
3. **Streaming Complexity**: While WebSockets inherently allow bidirectional streaming, implementing full gRPC streaming (server streaming, client streaming, bidi streaming) requires more elaborate logic to handle partial frames, flow control, etc. This example focuses on unary requests for simplicity.
4. **Security**: For production, you should secure with `wss://` (TLS). Also consider authentication (e.g., tokens, session cookies) at the WebSocket handshake level.
5. **Browser Compatibility**: While WebSockets are widely supported, some older browsers or extremely locked-down corporate environments might block them.
