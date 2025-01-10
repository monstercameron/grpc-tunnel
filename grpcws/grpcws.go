// client/grpcws/grpcws.go

package grpcws

import (
	"errors"
	"log"
	"sync"
	"syscall/js"

	"google.golang.org/protobuf/proto"

	"earlcameron.com/todos" // Replace with your actual module path and package
)

// Method IDs corresponding to gRPC methods
const (
	MethodCreateTodo = 0
	MethodListTodos  = 1
	MethodUpdateTodo = 2
	MethodDeleteTodo = 3
)

// CallbackFunc defines the signature for response callbacks
type CallbackFunc func(...interface{})

// GRPCWS encapsulates the WebSocket connection and method callbacks
type GRPCWS struct {
	ws           js.Value
	ready        bool
	callbacks    map[byte]CallbackFunc
	callbacksMux sync.RWMutex
}

// New creates a new GRPCWS instance and initiates the WebSocket connection
func New(url string) (*GRPCWS, error) {
	// Initialize the GRPCWS struct
	g := &GRPCWS{
		callbacks: make(map[byte]CallbackFunc),
	}

	// Establish WebSocket connection
	g.ws = js.Global().Get("WebSocket").New(url)
	g.ws.Set("binaryType", "arraybuffer")

	// Set up WebSocket event handlers
	g.ws.Set("onopen", js.FuncOf(g.onOpen))
	g.ws.Set("onerror", js.FuncOf(g.onError))
	g.ws.Set("onclose", js.FuncOf(g.onClose))
	g.ws.Set("onmessage", js.FuncOf(g.onMessage))

	return g, nil
}

// RegisterCallback associates a method ID with a callback function
func (g *GRPCWS) RegisterCallback(methodID byte, callback CallbackFunc) {
	g.callbacksMux.Lock()
	defer g.callbacksMux.Unlock()
	g.callbacks[methodID] = callback
}

// SendRequest marshals the request, prepends the method ID, and sends it over WebSocket
func (g *GRPCWS) SendRequest(methodID byte, req proto.Message) error {
	if !g.ready {
		log.Println("GRPCWS: WebSocket not ready for sending requests.")
		return errors.New("WebSocket connection not ready")
	}

	data, err := proto.Marshal(req)
	if err != nil {
		log.Printf("GRPCWS: Failed to marshal request for method %d: %v\n", methodID, err)
		return err
	}

	finalMsg := append([]byte{methodID}, data...)
	log.Printf("GRPCWS: Sending message for method ID %d: %v\n", methodID, finalMsg)
	uint8Array := js.Global().Get("Uint8Array").New(len(finalMsg))
	js.CopyBytesToJS(uint8Array, finalMsg)

	g.ws.Call("send", uint8Array)
	return nil
}

// onOpen handles the WebSocket 'onopen' event
func (g *GRPCWS) onOpen(this js.Value, args []js.Value) interface{} {
	log.Println("GRPCWS: WebSocket connection opened.")
	g.ready = true

	// Optionally, invoke a global JS callback to notify frontend
	js.Global().Call("onWebSocketOpen")

	return nil
}

// onError handles the WebSocket 'onerror' event
func (g *GRPCWS) onError(this js.Value, args []js.Value) interface{} {
	log.Println("GRPCWS: WebSocket encountered an error.")
	return nil
}

// onClose handles the WebSocket 'onclose' event
func (g *GRPCWS) onClose(this js.Value, args []js.Value) interface{} {
	log.Println("GRPCWS: WebSocket connection closed.")
	g.ready = false
	return nil
}

// onMessage handles the WebSocket 'onmessage' event
func (g *GRPCWS) onMessage(this js.Value, args []js.Value) interface{} {
	event := args[0]
	data := event.Get("data")
	log.Printf("GRPCWS: Message received: %v\n", data)

	array := js.Global().Get("Uint8Array").New(data)
	buf := make([]byte, array.Get("length").Int())
	js.CopyBytesToGo(buf, array)

	if len(buf) < 1 {
		log.Println("GRPCWS: Received empty message, ignoring.")
		return nil
	}

	methodID := buf[0]
	payload := buf[1:]
	log.Printf("GRPCWS: Method ID %d, payload: %v\n", methodID, payload)

	g.callbacksMux.RLock()
	callback, exists := g.callbacks[methodID]
	g.callbacksMux.RUnlock()

	if !exists {
		log.Printf("GRPCWS: No callback registered for method ID %d\n", methodID)
		return nil
	}

	argsParsed := g.parsePayload(methodID, payload)
	if argsParsed == nil {
		log.Printf("GRPCWS: Failed to parse arguments for method ID %d\n", methodID)
		return nil
	}

	log.Printf("GRPCWS: Invoking callback for method ID %d with arguments: %v\n", methodID, argsParsed)
	callback(argsParsed...)
	return nil
}

// parsePayload parses the response payload based on the method ID
func (g *GRPCWS) parsePayload(methodID byte, payload []byte) []interface{} {
	var args []interface{}

	switch methodID {
	case MethodCreateTodo:
		var resp todos.CreateTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("GRPCWS: Failed to unmarshal CreateTodoResponse: %v\n", err)
			return nil
		}
		args = append(args, resp.Todo.Id, resp.Todo.Text, resp.Todo.Done)

	case MethodListTodos:
		var resp todos.ListTodosResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("GRPCWS: Failed to unmarshal ListTodosResponse: %v\n", err)
			return nil
		}
		// Pass the entire slice of todos as a single argument
		return []interface{}{resp.Todos}

	case MethodUpdateTodo:
		var resp todos.UpdateTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("GRPCWS: Failed to unmarshal UpdateTodoResponse: %v\n", err)
			return nil
		}
		args = append(args, resp.Todo.Id, resp.Todo.Text, resp.Todo.Done)

	case MethodDeleteTodo:
		var resp todos.DeleteTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("GRPCWS: Failed to unmarshal DeleteTodoResponse: %v\n", err)
			return nil
		}
		args = append(args, resp.Success)

	default:
		log.Printf("GRPCWS: Unknown method ID %d\n", methodID)
	}

	return args
}
