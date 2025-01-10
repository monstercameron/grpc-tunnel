package main

import (
    "log"
    "syscall/js"

    "google.golang.org/protobuf/proto"

    "earlcameron.com/myservice" // Replace with your actual package path
)

var ws js.Value

// sendMessage handles the gRPC request creation and sending.
func sendMessage(this js.Value, args []js.Value) interface{} {
    text := args[0].String()
    log.Printf("WASM: Received text from JS: %s\n", text)

    // Build the gRPC request
    req := &myservice.EchoRequest{Message: text}

    // Marshal it
    data, err := proto.Marshal(req)
    if err != nil {
        log.Printf("WASM: Failed to marshal EchoRequest: %v\n", err)
        return nil
    }

    // Send as binary over the WebSocket
    log.Println("WASM: Sending binary gRPC request over WebSocket...")
    array := js.Global().Get("Uint8Array").New(len(data))
    js.CopyBytesToJS(array, data)
    ws.Call("send", array)
    return nil
}

// onWebSocketMessage handles incoming messages from the server.
func onWebSocketMessage(this js.Value, args []js.Value) interface{} {
    event := args[0]
    data := event.Get("data")

    // Convert JS Uint8Array -> Go []byte
    array := js.Global().Get("Uint8Array").New(data)
    buf := make([]byte, array.Get("length").Int())
    js.CopyBytesToGo(buf, array)

    // Unmarshal the gRPC response
    var resp myservice.EchoResponse
    if err := proto.Unmarshal(buf, &resp); err != nil {
        log.Printf("WASM: Failed to unmarshal EchoResponse: %v\n", err)
        return nil
    }

    log.Printf("WASM: Received gRPC response: %s\n", resp.Message)

    // We could optionally pass this response back to JS if needed
    return nil
}

// onWebSocketOpen logs a successful connection
func onWebSocketOpen(this js.Value, args []js.Value) interface{} {
    log.Println("WASM: WebSocket connected!")
    return nil
}

// onWebSocketError logs errors
func onWebSocketError(this js.Value, args []js.Value) interface{} {
    log.Println("WASM: WebSocket error occurred.")
    return nil
}

// onWebSocketClose logs closure
func onWebSocketClose(this js.Value, args []js.Value) interface{} {
    log.Println("WASM: WebSocket closed.")
    return nil
}

func main() {
    log.Println("WASM: Initializing...")

    // Create a WebSocket from JS
    ws = js.Global().Get("WebSocket").New("ws://localhost:8080/ws")
    ws.Set("binaryType", "arraybuffer") // We want binary data

    // Set event handlers
    ws.Set("onopen", js.FuncOf(onWebSocketOpen))
    ws.Set("onerror", js.FuncOf(onWebSocketError))
    ws.Set("onclose", js.FuncOf(onWebSocketClose))
    ws.Set("onmessage", js.FuncOf(onWebSocketMessage))

    // Expose sendMessage(...) to JavaScript
    // so that JS can call: window.sendMessage("Hello!")
    js.Global().Set("sendMessage", js.FuncOf(sendMessage))

    // Keep Go WASM runtime alive
    select {}
}
