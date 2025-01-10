package main

import (
	"log"
	"syscall/js"

	"earlcameron.com/grpcwsclient" // Adjust to your module path
	"earlcameron.com/todos"       // Adjust to your module path
)

var wsClient *grpcwsclient.GRPCWSClient

func main() {
	log.Println("WASM: Starting client...")

	// 1. Initialize WS client
	var err error
	wsClient, err = grpcwsclient.New("ws://localhost:8080/ws")
	if err != nil {
		log.Fatalf("WASM: Failed to create grpcwsclient: %v\n", err)
	}

	// 2. Register callbacks for each gRPC method ID
	wsClient.RegisterCallback(0, onCreateTodo)
	wsClient.RegisterCallback(1, onListTodos)
	wsClient.RegisterCallback(2, onUpdateTodo)
	wsClient.RegisterCallback(3, onDeleteTodo)

	// 3. Expose CRUD functions to JavaScript
	js.Global().Set("CreateTodo", js.FuncOf(createTodo))
	js.Global().Set("ListTodos", js.FuncOf(listTodos))
	js.Global().Set("UpdateTodo", js.FuncOf(updateTodo))
	js.Global().Set("DeleteTodo", js.FuncOf(deleteTodo))

	// Keep the WASM runtime alive
	select {}
}

// ----------------------------------------------------------------------------
// JavaScript-exposed functions
// ----------------------------------------------------------------------------

func createTodo(this js.Value, args []js.Value) interface{} {
	text := args[0].String()
	req := &todos.CreateTodoRequest{Text: text}
	if err := wsClient.SendRequest(0, req); err != nil {
		log.Printf("WASM: CreateTodo request failed: %v\n", err)
	}
	return nil
}

func listTodos(this js.Value, args []js.Value) interface{} {
	req := &todos.ListTodosRequest{}
	if err := wsClient.SendRequest(1, req); err != nil {
		log.Printf("WASM: ListTodos request failed: %v\n", err)
	}
	return nil
}

func updateTodo(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	text := args[1].String()
	done := args[2].Bool()

	req := &todos.UpdateTodoRequest{Id: id, Text: text, Done: done}
	if err := wsClient.SendRequest(2, req); err != nil {
		log.Printf("WASM: UpdateTodo request failed: %v\n", err)
	}
	return nil
}

func deleteTodo(this js.Value, args []js.Value) interface{} {
	id := args[0].String()

	req := &todos.DeleteTodoRequest{Id: id}
	if err := wsClient.SendRequest(3, req); err != nil {
		log.Printf("WASM: DeleteTodo request failed: %v\n", err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Callbacks
// ----------------------------------------------------------------------------

func onCreateTodo(args ...interface{}) {
	id := args[0].(string)
	text := args[1].(string)
	done := args[2].(bool)
	log.Printf("WASM: onCreateTodo => ID=%s, Text=%s, Done=%v\n", id, text, done)

	// Call a JS function "onCreateTodo" if it exists
	jsFunc := js.Global().Get("onCreateTodo")
	if jsFunc.Type() == js.TypeFunction {
		jsFunc.Invoke(id, text, done)
	}
}

func onListTodos(args ...interface{}) {
	todoSlice := args[0].([]*todos.Todo)
	log.Printf("WASM: onListTodos => received %d items\n", len(todoSlice))

	jsTodos := js.Global().Get("Array").New()
	for _, t := range todoSlice {
		obj := js.Global().Get("Object").New()
		obj.Set("id", t.Id)
		obj.Set("text", t.Text)
		obj.Set("done", t.Done)
		jsTodos.Call("push", obj)
	}

	jsFunc := js.Global().Get("onListTodos")
	if jsFunc.Type() == js.TypeFunction {
		jsFunc.Invoke(jsTodos)
	}
}

func onUpdateTodo(args ...interface{}) {
	id := args[0].(string)
	text := args[1].(string)
	done := args[2].(bool)
	log.Printf("WASM: onUpdateTodo => ID=%s, Text=%s, Done=%v\n", id, text, done)

	jsFunc := js.Global().Get("onUpdateTodo")
	if jsFunc.Type() == js.TypeFunction {
		jsFunc.Invoke(id, text, done)
	}
}

func onDeleteTodo(args ...interface{}) {
	success := args[0].(bool)
	log.Printf("WASM: onDeleteTodo => Success=%v\n", success)

	jsFunc := js.Global().Get("onDeleteTodo")
	if jsFunc.Type() == js.TypeFunction {
		jsFunc.Invoke(success)
	}
}
