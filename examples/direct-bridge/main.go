//go:build !js && !wasm

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

	"github.com/monstercameron/GoGRPCBridge/examples/_shared/proto"
	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"

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
	if _, parseErr := os.Stat(filePath); os.IsNotExist(parseErr) {
		// Create data directory if it doesn't exist
		if parseErr2 := os.MkdirAll("../_shared/data", 0750); parseErr2 != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", parseErr2)
		}
		if parseErr3 := ioutil.WriteFile(filePath, []byte("[]"), 0600); parseErr3 != nil {
			return nil, fmt.Errorf("failed to create todos.json: %w", parseErr3)
		}
	}
	parseData, parseErr4 := ioutil.ReadFile(filePath)
	if parseErr4 != nil {
		return nil, fmt.Errorf("failed to read todos.json: %w", parseErr4)
	}
	if len(parseData) == 0 {
		return []*proto.Todo{}, nil
	}
	var parseTs []*proto.Todo
	if parseErr5 := json.Unmarshal(parseData, &parseTs); parseErr5 != nil {
		_ = ioutil.WriteFile(filePath, []byte("[]"), 0600)
		return []*proto.Todo{}, nil
	}
	return parseTs, nil
}

func saveTodos(parseTodosSlice []*proto.Todo) error {
	const filePath = "../_shared/data/todos.json"
	parseOut, parseErr := json.MarshalIndent(parseTodosSlice, "", "  ")
	if parseErr != nil {
		return parseErr
	}
	return ioutil.WriteFile(filePath, parseOut, 0600)
}

func newTodoServer() (*todoServer, error) {
	parseT, parseErr := loadTodos()
	if parseErr != nil {
		return nil, parseErr
	}
	return &todoServer{store: parseT}, nil
}

func (parseS *todoServer) CreateTodo(parseCtx context.Context, parseReq *proto.CreateTodoRequest) (*proto.CreateTodoResponse, error) {
	parseS.mu.Lock()
	defer parseS.mu.Unlock()
	parseNewID := uuid.New().String()
	parseNewTodo := &proto.Todo{Id: parseNewID, Text: parseReq.Text, Done: false}
	parseS.store = append(parseS.store, parseNewTodo)
	_ = saveTodos(parseS.store)
	log.Printf("Created new todo: %s => %s\n", parseNewID, parseReq.Text)
	return &proto.CreateTodoResponse{Todo: parseNewTodo}, nil
}

func (parseS *todoServer) ListTodos(parseCtx context.Context, parseReq *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	parseS.mu.Lock()
	defer parseS.mu.Unlock()
	return &proto.ListTodosResponse{Todos: parseS.store}, nil
}

func (parseS *todoServer) UpdateTodo(parseCtx context.Context, parseReq *proto.UpdateTodoRequest) (*proto.UpdateTodoResponse, error) {
	parseS.mu.Lock()
	defer parseS.mu.Unlock()
	var parseUpdated *proto.Todo
	for _, parseT := range parseS.store {
		if parseT.Id == parseReq.Id {
			parseT.Text = parseReq.Text
			parseT.Done = parseReq.Done
			parseUpdated = parseT
			break
		}
	}
	if parseUpdated == nil {
		return &proto.UpdateTodoResponse{}, nil
	}
	_ = saveTodos(parseS.store)
	return &proto.UpdateTodoResponse{Todo: parseUpdated}, nil
}

func (parseS *todoServer) DeleteTodo(parseCtx context.Context, parseReq *proto.DeleteTodoRequest) (*proto.DeleteTodoResponse, error) {
	parseS.mu.Lock()
	defer parseS.mu.Unlock()
	parseIndex := -1
	for parseI, parseT := range parseS.store {
		if parseT.Id == parseReq.Id {
			parseIndex = parseI
			break
		}
	}
	if parseIndex == -1 {
		return &proto.DeleteTodoResponse{Success: false}, nil
	}
	parseS.store = append(parseS.store[:parseIndex], parseS.store[parseIndex+1:]...)
	_ = saveTodos(parseS.store)
	return &proto.DeleteTodoResponse{Success: true}, nil
}

func main() {
	// Create gRPC server with TodoService
	parseSrv, parseErr := newTodoServer()
	if parseErr != nil {
		log.Fatalf("Failed to create todoServer: %v\n", parseErr)
	}
	parseGrpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(parseGrpcServer, parseSrv)

	// One-liner: Serve gRPC over WebSocket
	log.Println("Direct gRPC-over-WebSocket server listening on :5000")
	log.Fatal(grpctunnel.ListenAndServe(":5000", parseGrpcServer,
		grpctunnel.WithConnectHook(func(parseR *http.Request) {
			log.Printf("Client connected: %s", parseR.RemoteAddr)
		}),
		grpctunnel.WithDisconnectHook(func(parseR2 *http.Request) {
			log.Printf("Client disconnected: %s", parseR2.RemoteAddr)
		}),
	))
}
