package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"earlcameron.com/grpcwsserver" // Adjust to your module path
	"earlcameron.com/todos"        // Adjust to your module path
)

// loadTodos reads from ./data/todos.json
func loadTodos() ([]*todos.Todo, error) {
	const filePath = "./data/todos.json"

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("No todos.json found; creating new at %s\n", filePath)
		if err := ioutil.WriteFile(filePath, []byte("[]"), 0644); err != nil {
			return nil, fmt.Errorf("failed to create todos.json: %w", err)
		}
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read todos.json: %w", err)
	}
	if len(data) == 0 {
		return []*todos.Todo{}, nil
	}

	var ts []*todos.Todo
	if err := json.Unmarshal(data, &ts); err != nil {
		log.Printf("Invalid JSON; resetting file. Error: %v\n", err)
		_ = ioutil.WriteFile(filePath, []byte("[]"), 0644)
		return []*todos.Todo{}, nil
	}
	return ts, nil
}

// saveTodos writes the in-memory slice back to ./data/todos.json
func saveTodos(todosSlice []*todos.Todo) error {
	const filePath = "./data/todos.json"
	out, err := json.MarshalIndent(todosSlice, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, out, 0644)
}

// todoServer implements the gRPC methods for TodoService
type todoServer struct {
	todos.UnimplementedTodoServiceServer
	mu    sync.Mutex
	store []*todos.Todo
}

func newTodoServer() (*todoServer, error) {
	t, err := loadTodos()
	if err != nil {
		return nil, err
	}
	return &todoServer{store: t}, nil
}

func (s *todoServer) CreateTodo(ctx context.Context, req *todos.CreateTodoRequest) (*todos.CreateTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newID := uuid.New().String()
	newTodo := &todos.Todo{
		Id:   newID,
		Text: req.Text,
		Done: false,
	}
	s.store = append(s.store, newTodo)
	_ = saveTodos(s.store)

	log.Printf("Created new todo: %s => %s\n", newID, req.Text)
	return &todos.CreateTodoResponse{Todo: newTodo}, nil
}

func (s *todoServer) ListTodos(ctx context.Context, req *todos.ListTodosRequest) (*todos.ListTodosResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Listing %d todos.\n", len(s.store))
	return &todos.ListTodosResponse{Todos: s.store}, nil
}

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
		log.Printf("No todo found with ID: %s\n", req.Id)
		return &todos.UpdateTodoResponse{}, nil
	}
	_ = saveTodos(s.store)
	log.Printf("Updated todo: %s => %s, done=%v\n", updated.Id, updated.Text, updated.Done)
	return &todos.UpdateTodoResponse{Todo: updated}, nil
}

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
		log.Printf("Delete: no todo found with ID %s\n", req.Id)
		return &todos.DeleteTodoResponse{Success: false}, nil
	}
	s.store = append(s.store[:index], s.store[index+1:]...)
	_ = saveTodos(s.store)

	log.Printf("Deleted todo with ID: %s\n", req.Id)
	return &todos.DeleteTodoResponse{Success: true}, nil
}

func main() {
	// 1. Create the gRPC server
	srv, err := newTodoServer()
	if err != nil {
		log.Fatalf("Failed to create todoServer: %v\n", err)
	}
	grpcServer := grpc.NewServer()
	todos.RegisterTodoServiceServer(grpcServer, srv)

	// 2. Start gRPC + WebSocket servers in parallel
	var wg sync.WaitGroup

	// Start gRPC server on :50051
	wg.Add(1)
	go grpcwsserver.StartGRPCServer(grpcServer, ":50051", &wg)

	// Connect a gRPC client to our own server (for the WS tunnel)
	conn, client, err := grpcwsserver.ConnectGRPCClient("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to connect gRPC client: %v\n", err)
	}
	defer conn.Close()

	// Start WebSocket server on :8080 serving ./public (with gzip on static files)
	wg.Add(1)
	go grpcwsserver.StartWebSocketServer(client, ":8080", "./public", &wg)

	// Wait for everything to finish if the servers ever shut down
	wg.Wait()
	log.Println("Server gracefully shut down.")
}
