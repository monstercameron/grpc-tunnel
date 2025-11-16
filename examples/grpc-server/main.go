package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/monstercameron/GoGRPCBridge/examples/_shared/proto" // Adjust to your module path
)

// loadTodos reads from ../_shared/data/todos.json
func loadTodos() ([]*proto.Todo, error) {
	const filePath = "../_shared/data/todos.json"

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("No proto.json found; creating new at %s\n", filePath)
		if err := ioutil.WriteFile(filePath, []byte("[]"), 0600); err != nil {
			return nil, fmt.Errorf("failed to create proto.json: %w", err)
		}
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proto.json: %w", err)
	}
	if len(data) == 0 {
		return []*proto.Todo{}, nil
	}

	var ts []*proto.Todo
	if err := json.Unmarshal(data, &ts); err != nil {
		log.Printf("Invalid JSON; resetting file. Error: %v\n", err)
		_ = ioutil.WriteFile(filePath, []byte("[]"), 0600)
		return []*proto.Todo{}, nil
	}
	return ts, nil
}

// saveTodos writes the in-memory slice back to ../_shared/data/todos.json
func saveTodos(todosSlice []*proto.Todo) error {
	const filePath = "../_shared/data/todos.json"
	out, err := json.MarshalIndent(todosSlice, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, out, 0600)
}

// todoServer implements the gRPC methods for TodoService
type todoServer struct {
	proto.UnimplementedTodoServiceServer
	mu    sync.Mutex
	store []*proto.Todo
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
	newTodo := &proto.Todo{
		Id:   newID,
		Text: req.Text,
		Done: false,
	}
	s.store = append(s.store, newTodo)
	_ = saveTodos(s.store)

	log.Printf("Created new todo: %s => %s\n", newID, req.Text)
	return &proto.CreateTodoResponse{Todo: newTodo}, nil
}

func (s *todoServer) ListTodos(ctx context.Context, req *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Listing %d todos.\n", len(s.store))
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
		log.Printf("No todo found with ID: %s\n", req.Id)
		return &proto.UpdateTodoResponse{}, nil
	}
	_ = saveTodos(s.store)
	log.Printf("Updated todo: %s => %s, done=%v\n", updated.Id, updated.Text, updated.Done)
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
		log.Printf("Delete: no todo found with ID %s\n", req.Id)
		return &proto.DeleteTodoResponse{Success: false}, nil
	}
	s.store = append(s.store[:index], s.store[index+1:]...)
	_ = saveTodos(s.store)

	log.Printf("Deleted todo with ID: %s\n", req.Id)
	return &proto.DeleteTodoResponse{Success: true}, nil
}

func main() {
	// 1. Create the gRPC server
	srv, err := newTodoServer()
	if err != nil {
		log.Fatalf("Failed to create todoServer: %v\n", err)
	}
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, srv)

	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Printf("gRPC server listening on :50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
