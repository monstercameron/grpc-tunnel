// Direct gRPC-over-WebSocket example - no proxy needed!
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"grpc-tunnel/examples/_shared/proto"
	"grpc-tunnel/pkg/grpctunnel"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

// TodoServer implementation
type todoServer struct {
	proto.UnimplementedTodoServiceServer
	mu    sync.Mutex
	store []*proto.Todo
}

func loadTodos() ([]*proto.Todo, error) {
	const filePath = "../_shared/data/todos.json"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create data directory if it doesn't exist
		if err := os.MkdirAll("../_shared/data", 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
		if err := ioutil.WriteFile(filePath, []byte("[]"), 0644); err != nil {
			return nil, fmt.Errorf("failed to create todos.json: %w", err)
		}
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read todos.json: %w", err)
	}
	if len(data) == 0 {
		return []*proto.Todo{}, nil
	}
	var ts []*proto.Todo
	if err := json.Unmarshal(data, &ts); err != nil {
		_ = ioutil.WriteFile(filePath, []byte("[]"), 0644)
		return []*proto.Todo{}, nil
	}
	return ts, nil
}

func saveTodos(todosSlice []*proto.Todo) error {
	const filePath = "../_shared/data/todos.json"
	out, err := json.MarshalIndent(todosSlice, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, out, 0644)
}

func newTodoServer() (*todoServer, error) {
	t, err := loadTodos()
	if err != nil {
		return nil, err
	}
	return &todoServer{store: t}, nil
}

func (s *todoServer) CreateTodo(ctx context.Context, req *proto.CreateTodoRequest) (*proto.CreateTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	newID := uuid.New().String()
	newTodo := &proto.Todo{Id: newID, Text: req.Text, Done: false}
	s.store = append(s.store, newTodo)
	_ = saveTodos(s.store)
	log.Printf("Created new todo: %s => %s\n", newID, req.Text)
	return &proto.CreateTodoResponse{Todo: newTodo}, nil
}

func (s *todoServer) ListTodos(ctx context.Context, req *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &proto.ListTodosResponse{Todos: s.store}, nil
}

func (s *todoServer) UpdateTodo(ctx context.Context, req *proto.UpdateTodoRequest) (*proto.UpdateTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var updated *proto.Todo
	for _, t := range s.store {
		if t.Id == req.Id {
			t.Text = req.Text
			t.Done = req.Done
			updated = t
			break
		}
	}
	if updated == nil {
		return &proto.UpdateTodoResponse{}, nil
	}
	_ = saveTodos(s.store)
	return &proto.UpdateTodoResponse{Todo: updated}, nil
}

func (s *todoServer) DeleteTodo(ctx context.Context, req *proto.DeleteTodoRequest) (*proto.DeleteTodoResponse, error) {
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
		return &proto.DeleteTodoResponse{Success: false}, nil
	}
	s.store = append(s.store[:index], s.store[index+1:]...)
	_ = saveTodos(s.store)
	return &proto.DeleteTodoResponse{Success: true}, nil
}

func main() {
	// Create gRPC server with TodoService
	srv, err := newTodoServer()
	if err != nil {
		log.Fatalf("Failed to create todoServer: %v\n", err)
	}
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, srv)

	// One-liner: Serve gRPC over WebSocket
	log.Println("Direct gRPC-over-WebSocket server listening on :5000")
	log.Fatal(grpctunnel.ListenAndServe(":5000", grpcServer,
		grpctunnel.WithConnectHook(func(r *http.Request) {
			log.Printf("Client connected: %s", r.RemoteAddr)
		}),
		grpctunnel.WithDisconnectHook(func(r *http.Request) {
			log.Printf("Client disconnected: %s", r.RemoteAddr)
		}),
	))
}
