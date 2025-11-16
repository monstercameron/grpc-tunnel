package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"grpc-tunnel/pkg/bridge"
	"grpc-tunnel/proto"
)

// Inline TodoServer implementation for the bridge
type todoServer struct {
	proto.UnimplementedTodoServiceServer
	mu    sync.Mutex
	store []*proto.Todo
}

func loadTodos() ([]*proto.Todo, error) {
	const filePath = "./data/todos.json"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
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
	const filePath = "./data/todos.json"
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
	listenAddress := flag.String("listen", ":8080", "Address to listen on for WebSocket connections")
	flag.Parse()

	log.Printf("Starting direct gRPC-over-WebSocket bridge...")
	log.Printf("Listening on: %s", *listenAddress)

	// Create gRPC server with TodoService
	srv, err := newTodoServer()
	if err != nil {
		log.Fatalf("Failed to create todoServer: %v\n", err)
	}
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, srv)

	// Serve gRPC directly over WebSocket
	http.Handle("/", bridge.ServeHandler(bridge.ServerConfig{
		GRPCServer: grpcServer,
		OnConnect: func(r *http.Request) {
			log.Printf("Client connected from %s", r.RemoteAddr)
		},
		OnDisconnect: func(r *http.Request) {
			log.Printf("Client disconnected from %s", r.RemoteAddr)
		},
	}))

	log.Println("Serving gRPC over WebSocket (direct mode)")
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Bridge server failed: %v", err)
	}
}