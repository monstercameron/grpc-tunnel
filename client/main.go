package main

import (
	"log"
	"syscall/js"

	"earlcameron.com/grpcws"
	"earlcameron.com/todos"
)

var ws *grpcws.GRPCWS

func main() {
	log.Println("WASM: Starting up...")

	// Initialize the GRPCWS package with the WebSocket URL
	var err error
	ws, err = grpcws.New("ws://localhost:8080/ws")
	if err != nil {
		log.Fatalf("WASM: Failed to initialize GRPCWS: %v\n", err)
	}

	// Register callbacks for each method ID
	ws.RegisterCallback(grpcws.MethodCreateTodo, onCreateTodo)
	ws.RegisterCallback(grpcws.MethodListTodos, onListTodo)
	ws.RegisterCallback(grpcws.MethodUpdateTodo, onUpdateTodo)
	ws.RegisterCallback(grpcws.MethodDeleteTodo, onDeleteTodo)

	// Expose CRUD functions to JavaScript
	js.Global().Set("CreateTodo", js.FuncOf(createTodo))
	js.Global().Set("ListTodos", js.FuncOf(listTodos))
	js.Global().Set("UpdateTodo", js.FuncOf(updateTodo))
	js.Global().Set("DeleteTodo", js.FuncOf(deleteTodo))

	// Keep the WASM module running
	select {}
}

// createTodo handles the CreateTodo request from JavaScript
func createTodo(this js.Value, args []js.Value) interface{} {
	text := args[0].String()
	log.Printf("WASM: createTodo called with text=%s\n", text)

	req := &todos.CreateTodoRequest{Text: text}
	if err := ws.SendRequest(grpcws.MethodCreateTodo, req); err != nil {
		log.Printf("WASM: Failed to send CreateTodoRequest: %v\n", err)
	}

	return nil
}

// listTodos handles the ListTodos request from JavaScript
func listTodos(this js.Value, args []js.Value) interface{} {
	log.Println("WASM: listTodos called")

	req := &todos.ListTodosRequest{}
	if err := ws.SendRequest(grpcws.MethodListTodos, req); err != nil {
		log.Printf("WASM: Failed to send ListTodosRequest: %v\n", err)
	}

	return nil
}

// updateTodo handles the UpdateTodo request from JavaScript
func updateTodo(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	text := args[1].String()
	done := args[2].Bool()
	log.Printf("WASM: updateTodo called with id=%s, text=%s, done=%v\n", id, text, done)

	req := &todos.UpdateTodoRequest{Id: id, Text: text, Done: done}
	if err := ws.SendRequest(grpcws.MethodUpdateTodo, req); err != nil {
		log.Printf("WASM: Failed to send UpdateTodoRequest: %v\n", err)
	}

	return nil
}

// deleteTodo handles the DeleteTodo request from JavaScript
func deleteTodo(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	log.Printf("WASM: deleteTodo called with id=%s\n", id)

	req := &todos.DeleteTodoRequest{Id: id}
	if err := ws.SendRequest(grpcws.MethodDeleteTodo, req); err != nil {
		log.Printf("WASM: Failed to send DeleteTodoRequest: %v\n", err)
	}

	return nil
}

// onCreateTodo handles the CreateTodoResponse
func onCreateTodo(args ...interface{}) {
	id := args[0].(string)
	text := args[1].(string)
	done := args[2].(bool)
	log.Printf("WASM: Received CreateTodoResponse => ID: %s, Text: %s, Done: %v\n", id, text, done)
	js.Global().Call("onCreateTodo", id, text, done)
}

// onListTodo handles each Todo item in ListTodosResponse
func onListTodo(args ...interface{}) {
    todos := args[0].([]*todos.Todo)

    jsTodos := js.Global().Get("Array").New()
    for _, todo := range todos {
        jsTodo := js.Global().Get("Object").New()
        jsTodo.Set("id", todo.Id)
        jsTodo.Set("text", todo.Text)
        jsTodo.Set("done", todo.Done)
        jsTodos.Call("push", jsTodo)
    }

    // Verify JS function exists before calling
    if js.Global().Get("onListTodos").Type() == js.TypeFunction {
        js.Global().Call("onListTodos", jsTodos)
    } else {
        log.Panic("JS function onListTodos is not defined")
    }
}

// onUpdateTodo handles the UpdateTodoResponse
func onUpdateTodo(args ...interface{}) {
	id := args[0].(string)
	text := args[1].(string)
	done := args[2].(bool)
	log.Printf("WASM: Received UpdateTodoResponse => ID: %s, Text: %s, Done: %v\n", id, text, done)
	js.Global().Call("onUpdateTodo", id, text, done)
}

// onDeleteTodo handles the DeleteTodoResponse
func onDeleteTodo(args ...interface{}) {
	success := args[0].(bool)
	log.Printf("WASM: Received DeleteTodoResponse => Success: %v\n", success)
	js.Global().Call("onDeleteTodo", success)
}
