package main

import (
	"log"
	"syscall/js"

	"google.golang.org/protobuf/proto"

	"earlcameron.com/todos" // Replace with your actual module path and package
)

var ws js.Value

// Method IDs to match the server's switch-case
const (
	methodCreateTodo = 0
	methodListTodos  = 1
	methodUpdateTodo = 2
	methodDeleteTodo = 3
)

func main() {
	log.Println("WASM: Starting up...")

	// 1. Create a WebSocket connection to the server
	ws = js.Global().Get("WebSocket").New("ws://localhost:8080/ws")
	ws.Set("binaryType", "arraybuffer") // Receive binary data

	// 2. Set up WebSocket event handlers
	ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		log.Println("WASM: WebSocket connected.")
		// Notify JS that the connection is open
		js.Global().Call("onWebSocketOpen")
		return nil
	}))

	ws.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		log.Println("WASM: WebSocket error occurred.")
		return nil
	}))

	ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		log.Println("WASM: WebSocket closed.")
		return nil
	}))

	ws.Set("onmessage", js.FuncOf(onWebSocketMessage))

	// 3. Expose CRUD functions to JavaScript
	js.Global().Set("CreateTodo", js.FuncOf(createTodo))
	js.Global().Set("ListTodos", js.FuncOf(listTodos))
	js.Global().Set("UpdateTodo", js.FuncOf(updateTodo))
	js.Global().Set("DeleteTodo", js.FuncOf(deleteTodo))

	// 4. Keep the WASM module running
	select {}
}

// onWebSocketMessage handles incoming messages from the server
func onWebSocketMessage(this js.Value, args []js.Value) interface{} {
	event := args[0]
	data := event.Get("data")

	// Convert JS ArrayBuffer to Go []byte
	array := js.Global().Get("Uint8Array").New(data)
	buf := make([]byte, array.Get("length").Int())
	js.CopyBytesToGo(buf, array)

	if len(buf) < 1 {
		log.Println("WASM: Received empty message, ignoring.")
		return nil
	}

	methodID := buf[0]
	payload := buf[1:]

	switch methodID {
	case methodCreateTodo:
		var resp todos.CreateTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("WASM: Failed to unmarshal CreateTodoResponse: %v\n", err)
			return nil
		}
		log.Printf("WASM: Received CreateTodoResponse => ID: %s\n", resp.Todo.Id)
		// Notify JS to update the UI
		js.Global().Call("onCreateTodo", resp.Todo.Id, resp.Todo.Text, resp.Todo.Done)

	case methodListTodos:
		var resp todos.ListTodosResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("WASM: Failed to unmarshal ListTodosResponse: %v\n", err)
			return nil
		}
		log.Printf("WASM: Received ListTodosResponse => %d todos\n", len(resp.Todos))
		// Pass each todo to JS to render in the UI
		for _, t := range resp.Todos {
			js.Global().Call("onListTodo", t.Id, t.Text, t.Done)
		}

	case methodUpdateTodo:
		var resp todos.UpdateTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("WASM: Failed to unmarshal UpdateTodoResponse: %v\n", err)
			return nil
		}
		log.Printf("WASM: Received UpdateTodoResponse => ID: %s\n", resp.Todo.Id)
		js.Global().Call("onUpdateTodo", resp.Todo.Id, resp.Todo.Text, resp.Todo.Done)

	case methodDeleteTodo:
		var resp todos.DeleteTodoResponse
		if err := proto.Unmarshal(payload, &resp); err != nil {
			log.Printf("WASM: Failed to unmarshal DeleteTodoResponse: %v\n", err)
			return nil
		}
		log.Printf("WASM: Received DeleteTodoResponse => success: %v\n", resp.Success)
		js.Global().Call("onDeleteTodo", resp.Success)

	default:
		log.Printf("WASM: Unknown methodID in response: %d\n", methodID)
	}

	return nil
}

// createTodo handles the CreateTodo request from JavaScript
func createTodo(this js.Value, args []js.Value) interface{} {
	text := args[0].String()
	log.Printf("WASM: createTodo called with text=%s\n", text)

	// Build the CreateTodoRequest
	req := &todos.CreateTodoRequest{Text: text}

	// Marshal the request to Protobuf
	data, err := proto.Marshal(req)
	if err != nil {
		log.Printf("WASM: Failed to marshal CreateTodoRequest: %v\n", err)
		return nil
	}

	// Prepend method ID
	finalMsg := append([]byte{methodCreateTodo}, data...)

	// Send as binary over WebSocket
	log.Println("WASM: Sending CreateTodoRequest over WebSocket...")
	array := js.Global().Get("Uint8Array").New(len(finalMsg))
	js.CopyBytesToJS(array, finalMsg)
	ws.Call("send", array)

	return nil
}

// listTodos handles the ListTodos request from JavaScript
func listTodos(this js.Value, args []js.Value) interface{} {
	log.Println("WASM: listTodos called")

	// Build the ListTodosRequest
	req := &todos.ListTodosRequest{}

	// Marshal the request to Protobuf
	data, err := proto.Marshal(req)
	if err != nil {
		log.Printf("WASM: Failed to marshal ListTodosRequest: %v\n", err)
		return nil
	}

	// Prepend method ID
	finalMsg := append([]byte{methodListTodos}, data...)

	// Send as binary over WebSocket
	log.Println("WASM: Sending ListTodosRequest over WebSocket...")
	array := js.Global().Get("Uint8Array").New(len(finalMsg))
	js.CopyBytesToJS(array, finalMsg)
	ws.Call("send", array)

	return nil
}

// updateTodo handles the UpdateTodo request from JavaScript
func updateTodo(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	text := args[1].String()
	done := args[2].Bool()
	log.Printf("WASM: updateTodo called with id=%s, text=%s, done=%v\n", id, text, done)

	// Build the UpdateTodoRequest
	req := &todos.UpdateTodoRequest{
		Id:   id,
		Text: text,
		Done: done,
	}

	// Marshal the request to Protobuf
	data, err := proto.Marshal(req)
	if err != nil {
		log.Printf("WASM: Failed to marshal UpdateTodoRequest: %v\n", err)
		return nil
	}

	// Prepend method ID
	finalMsg := append([]byte{methodUpdateTodo}, data...)

	// Send as binary over WebSocket
	log.Println("WASM: Sending UpdateTodoRequest over WebSocket...")
	array := js.Global().Get("Uint8Array").New(len(finalMsg))
	js.CopyBytesToJS(array, finalMsg)
	ws.Call("send", array)

	return nil
}

// deleteTodo handles the DeleteTodo request from JavaScript
func deleteTodo(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	log.Printf("WASM: deleteTodo called with id=%s\n", id)

	// Build the DeleteTodoRequest
	req := &todos.DeleteTodoRequest{
		Id: id,
	}

	// Marshal the request to Protobuf
	data, err := proto.Marshal(req)
	if err != nil {
		log.Printf("WASM: Failed to marshal DeleteTodoRequest: %v\n", err)
		return nil
	}

	// Prepend method ID
	finalMsg := append([]byte{methodDeleteTodo}, data...)

	// Send as binary over WebSocket
	log.Println("WASM: Sending DeleteTodoRequest over WebSocket...")
	array := js.Global().Get("Uint8Array").New(len(finalMsg))
	js.CopyBytesToJS(array, finalMsg)
	ws.Call("send", array)

	return nil
}
