package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"earlcameron.com/todos" // Replace with your module path + generated package
)

// ----------------------------------------------------------------
// In-memory representation of todos, backed by a JSON file on disk
// ----------------------------------------------------------------

// loadTodos reads ./data/todos.json into memory
func loadTodos() ([]*todos.Todo, error) {
	filePath := "./data/todos.json"

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// If not, create the file with an empty JSON array
		log.Printf("todos.json not found. Creating a new one at %s\n", filePath)
		if err := ioutil.WriteFile(filePath, []byte("[]"), 0644); err != nil {
			return nil, fmt.Errorf("failed to create todos.json: %w", err)
		}
	}

	// Read the file
	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read todos.json: %w", err)
	}

	// If the file is empty, treat it as an empty slice
	if len(fileData) == 0 {
		log.Printf("todos.json is empty. Initializing with an empty list.\n")
		return []*todos.Todo{}, nil
	}

	var ts []*todos.Todo
	if err := json.Unmarshal(fileData, &ts); err != nil {
		// If JSON is malformed, log the error and reset to empty
		log.Printf("Error unmarshaling todos.json: %v\n", err)
		log.Printf("Resetting todos.json to an empty list.\n")
		if err := ioutil.WriteFile(filePath, []byte("[]"), 0644); err != nil {
			return nil, fmt.Errorf("failed to reset todos.json: %w", err)
		}
		return []*todos.Todo{}, nil
	}
	return ts, nil
}

// saveTodos writes the in-memory todos to ./data/todos.json
func saveTodos(todosSlice []*todos.Todo) error {
	filePath := "./data/todos.json"

	fileData, err := json.MarshalIndent(todosSlice, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal todos: %w", err)
	}
	if err := ioutil.WriteFile(filePath, fileData, 0644); err != nil {
		return fmt.Errorf("failed to write to todos.json: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------
// gRPC Service Implementation: TodoService
// ----------------------------------------------------------------

type todoServer struct {
	todos.UnimplementedTodoServiceServer
	mu    sync.Mutex
	store []*todos.Todo
}

func newTodoServer() (*todoServer, error) {
	ts, err := loadTodos()
	if err != nil {
		return nil, err
	}
	return &todoServer{
		store: ts,
	}, nil
}

// CreateTodo adds a new todo to the store
func (s *todoServer) CreateTodo(ctx context.Context, req *todos.CreateTodoRequest) (*todos.CreateTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newTodo := &todos.Todo{
		Id:   uuid.New().String(),
		Text: req.Text,
		Done: false,
	}
	s.store = append(s.store, newTodo)

	// Persist changes
	if err := saveTodos(s.store); err != nil {
		log.Printf("Error saving todos: %v\n", err)
	}

	log.Printf("CreateTodo: created new todo [%s] => %s\n", newTodo.Id, newTodo.Text)
	return &todos.CreateTodoResponse{Todo: newTodo}, nil
}

// ListTodos returns the entire list of todos
func (s *todoServer) ListTodos(ctx context.Context, req *todos.ListTodosRequest) (*todos.ListTodosResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("ListTodos: returning %d todos\n", len(s.store))
	return &todos.ListTodosResponse{Todos: s.store}, nil
}

// UpdateTodo modifies an existing todo
func (s *todoServer) UpdateTodo(ctx context.Context, req *todos.UpdateTodoRequest) (*todos.UpdateTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated *todos.Todo
	for _, t := range s.store {
		if t.Id == req.Id {
			t.Text = req.Text
			t.Done = req.Done
			updated = t
			break
		}
	}

	if updated == nil {
		log.Printf("UpdateTodo: no todo found with ID %s\n", req.Id)
		return &todos.UpdateTodoResponse{}, nil
	}

	// Persist changes
	if err := saveTodos(s.store); err != nil {
		log.Printf("Error saving todos: %v\n", err)
	}

	log.Printf("UpdateTodo: updated todo [%s]\n", updated.Id)
	return &todos.UpdateTodoResponse{Todo: updated}, nil
}

// DeleteTodo removes a todo from the store
func (s *todoServer) DeleteTodo(ctx context.Context, req *todos.DeleteTodoRequest) (*todos.DeleteTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := -1
	for i, t := range s.store {
		if t.Id == req.Id {
			index = i
			break
		}
	}

	if index == -1 {
		log.Printf("DeleteTodo: no todo found with ID %s\n", req.Id)
		return &todos.DeleteTodoResponse{Success: false}, nil
	}

	// Remove from slice
	s.store = append(s.store[:index], s.store[index+1:]...)

	// Persist changes
	if err := saveTodos(s.store); err != nil {
		log.Printf("Error saving todos: %v\n", err)
	}

	log.Printf("DeleteTodo: removed todo with ID %s\n", req.Id)
	return &todos.DeleteTodoResponse{Success: true}, nil
}

// ----------------------------------------------------------------
// WebSocket Tunneling + HTTP File Serving
// ----------------------------------------------------------------

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// For demo purposes, allow all origins. Restrict in production!
		return true
	},
}

// handleWebSocketConnection:
// - Reads binary messages from the client (WASM)
// - Unmarshals them as gRPC requests
// - Forwards to the local gRPC server
// - Sends back gRPC responses as binary
func handleWebSocketConnection(conn *websocket.Conn, client todos.TodoServiceClient) {
	defer conn.Close()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v\n", err)
			return
		}
		if msgType != websocket.BinaryMessage {
			log.Println("Ignoring non-binary message on WebSocket.")
			continue
		}

		// The first byte indicates the method ID
		if len(data) < 1 {
			log.Println("Received empty WebSocket message, ignoring.")
			continue
		}

		method := data[0]   // 1 byte for method ID
		payload := data[1:] // the rest is the Protobuf request

		switch method {
		case 0: // CreateTodo
			var req todos.CreateTodoRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal CreateTodoRequest: %v\n", err)
				continue
			}
			resp, err := client.CreateTodo(context.Background(), &req)
			if err != nil {
				log.Printf("CreateTodo failed: %v\n", err)
				continue
			}
			sendResponse(conn, 0, resp)

		case 1: // ListTodos
			var req todos.ListTodosRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal ListTodosRequest: %v\n", err)
				continue
			}
			resp, err := client.ListTodos(context.Background(), &req)
			if err != nil {
				log.Printf("ListTodos failed: %v\n", err)
				continue
			}
			sendResponse(conn, 1, resp)

		case 2: // UpdateTodo
			var req todos.UpdateTodoRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal UpdateTodoRequest: %v\n", err)
				continue
			}
			resp, err := client.UpdateTodo(context.Background(), &req)
			if err != nil {
				log.Printf("UpdateTodo failed: %v\n", err)
				continue
			}
			sendResponse(conn, 2, resp)

		case 3: // DeleteTodo
			var req todos.DeleteTodoRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal DeleteTodoRequest: %v\n", err)
				continue
			}
			resp, err := client.DeleteTodo(context.Background(), &req)
			if err != nil {
				log.Printf("DeleteTodo failed: %v\n", err)
				continue
			}
			sendResponse(conn, 3, resp)

		default:
			log.Printf("Unknown method ID: %d\n", method)
		}
	}
}

// sendResponse serializes the gRPC response and sends it back over the WebSocket with the method ID
func sendResponse(conn *websocket.Conn, methodID byte, message proto.Message) {
	respBytes, err := proto.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal response for method %d: %v\n", methodID, err)
		return
	}
	// Prepend method ID to the response
	finalData := append([]byte{methodID}, respBytes...)
	if err := conn.WriteMessage(websocket.BinaryMessage, finalData); err != nil {
		log.Printf("WebSocket write error: %v\n", err)
	}
}

// startWebSocketServer starts the HTTP server with WebSocket upgrade and static file serving
func startWebSocketServer(grpcClient todos.TodoServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	// Serve static files from ./public
	publicDir := "./public"
	fs := http.FileServer(http.Dir(publicDir))
	http.Handle("/", fs)

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v\n", err)
			return
		}
		log.Println("New WebSocket connection established.")
		handleWebSocketConnection(conn, grpcClient)
	})

	log.Printf("Serving static files from %s\n", publicDir)
	log.Println("HTTP + WebSocket server listening on :8080")

	// Start the HTTP server
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("HTTP server failed: %v\n", err)
	}
}

// ----------------------------------------------------------------
// Main Function
// ----------------------------------------------------------------

func main() {
	// 1. Instantiate gRPC server with TodoService
	srv, err := newTodoServer()
	if err != nil {
		log.Fatalf("Failed to create todoServer: %v\n", err)
	}

	grpcServer := grpc.NewServer()
	todos.RegisterTodoServiceServer(grpcServer, srv)

	// 2. Start gRPC server in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("Failed to listen on :50051: %v\n", err)
		}
		log.Println("gRPC server listening on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v\n", err)
		}
	}()

	// 3. Connect a gRPC client to our own server (for the WebSocket tunnel)
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to local gRPC server: %v\n", err)
	}
	client := todos.NewTodoServiceClient(conn)
	defer conn.Close()

	// 4. Start the WebSocket server and file server
	wg.Add(1)
	go startWebSocketServer(client, &wg)

	// 5. Wait indefinitely
	wg.Wait()
	fmt.Println("Server shut down gracefully.")
}
