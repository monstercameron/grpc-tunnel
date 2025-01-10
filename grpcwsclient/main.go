package grpcwsclient

import (
	"errors"
	"log"
	"sync"
	"syscall/js"

	"google.golang.org/protobuf/proto"

	"earlcameron.com/todos" // Adjust to match your module path
)

type CallbackFunc func(...interface{})

// GRPCWSClient encapsulates WebSocket communication and callbacks.
type GRPCWSClient struct {
	ws           js.Value
	ready        bool
	callbacks    map[byte]CallbackFunc
	callbacksMux sync.RWMutex
}

// New creates a new GRPCWSClient and connects to the WebSocket server.
func New(url string) (*GRPCWSClient, error) {
	client := &GRPCWSClient{
		callbacks: make(map[byte]CallbackFunc),
	}

	client.ws = js.Global().Get("WebSocket").New(url)
	client.ws.Set("binaryType", "arraybuffer")
	client.ws.Set("onopen", js.FuncOf(client.onOpen))
	client.ws.Set("onerror", js.FuncOf(client.onError))
	client.ws.Set("onclose", js.FuncOf(client.onClose))
	client.ws.Set("onmessage", js.FuncOf(client.onMessage))

	return client, nil
}

// RegisterCallback associates a method ID with a callback function.
func (g *GRPCWSClient) RegisterCallback(methodID byte, callback CallbackFunc) {
	g.callbacksMux.Lock()
	defer g.callbacksMux.Unlock()
	g.callbacks[methodID] = callback
}

// SendRequest sends a gRPC request over WebSocket.
func (c *GRPCWSClient) SendRequest(methodID byte, req proto.Message) error {
    if !c.ready {
        log.Println("WASM: WebSocket not ready for sending requests.")
        return errors.New("WebSocket connection not ready")
    }

    data, err := proto.Marshal(req)
    if err != nil {
        log.Printf("WASM: Failed to marshal request for method %d: %v\n", methodID, err)
        return err
    }

    finalMsg := append([]byte{methodID}, data...)
    log.Printf("WASM: Sending message for method ID %d: %v\n", methodID, finalMsg)
    uint8Array := js.Global().Get("Uint8Array").New(len(finalMsg))
    js.CopyBytesToJS(uint8Array, finalMsg)

    c.ws.Call("send", uint8Array)
    return nil
}

func (c *GRPCWSClient) onOpen(this js.Value, args []js.Value) interface{} {
    log.Println("WASM: WebSocket connection opened. Setting ready state...")
    c.ready = true
    
    // Expose WSReady to the JS global scope
    js.Global().Set("WSReady", js.ValueOf(true))

    // Check if onWebSocketOpen is defined in JS; if so, call it.
    jsFunc := js.Global().Get("onWebSocketOpen")
    if jsFunc.Type() == js.TypeFunction {
        jsFunc.Invoke()
    }

    return nil
}

func (g *GRPCWSClient) onError(this js.Value, args []js.Value) interface{} {
	log.Println("WebSocket error occurred.")
	return nil
}

func (g *GRPCWSClient) onClose(this js.Value, args []js.Value) interface{} {
	log.Println("WebSocket connection closed.")
	g.ready = false
	return nil
}

func (g *GRPCWSClient) onMessage(this js.Value, args []js.Value) interface{} {
	event := args[0]
	data := event.Get("data")
	array := js.Global().Get("Uint8Array").New(data)
	buf := make([]byte, array.Get("length").Int())
	js.CopyBytesToGo(buf, array)

	if len(buf) < 1 {
		return nil
	}

	methodID := buf[0]
	payload := buf[1:]

	g.callbacksMux.RLock()
	callback, exists := g.callbacks[methodID]
	g.callbacksMux.RUnlock()
	if !exists {
		log.Printf("No callback for method ID %d", methodID)
		return nil
	}

	argsParsed := g.parsePayload(methodID, payload)
	if argsParsed != nil {
		callback(argsParsed...)
	}
	return nil
}

func (g *GRPCWSClient) parsePayload(methodID byte, payload []byte) []interface{} {
	switch methodID {
	case 0: // CreateTodo
		var resp todos.CreateTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("Failed to unmarshal CreateTodoResponse: %v", err)
			return nil
		}
		return []interface{}{resp.Todo.Id, resp.Todo.Text, resp.Todo.Done}

	case 1: // ListTodos
		var resp todos.ListTodosResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("Failed to unmarshal ListTodosResponse: %v", err)
			return nil
		}
		return []interface{}{resp.Todos}

	case 2: // UpdateTodo
		var resp todos.UpdateTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("Failed to unmarshal UpdateTodoResponse: %v", err)
			return nil
		}
		return []interface{}{resp.Todo.Id, resp.Todo.Text, resp.Todo.Done}

	case 3: // DeleteTodo
		var resp todos.DeleteTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("Failed to unmarshal DeleteTodoResponse: %v", err)
			return nil
		}
		return []interface{}{resp.Success}

	default:
		log.Printf("Unknown method ID %d", methodID)
		return nil
	}
}
