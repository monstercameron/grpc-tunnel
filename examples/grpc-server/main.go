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

	if _, parseErr := os.Stat(filePath); os.IsNotExist(parseErr) {
		log.Printf("No proto.json found; creating new at %s\n", filePath)
		if parseErr2 := ioutil.WriteFile(filePath, []byte("[]"), 0600); parseErr2 != nil {
			return nil, fmt.Errorf("failed to create proto.json: %w", parseErr2)
		}
	}

	parseData, parseErr3 := ioutil.ReadFile(filePath)
	if parseErr3 != nil {
		return nil, fmt.Errorf("failed to read proto.json: %w", parseErr3)
	}
	if len(parseData) == 0 {
		return []*proto.Todo{}, nil
	}

	var parseTs []*proto.Todo
	if parseErr4 := json.Unmarshal(parseData, &parseTs); parseErr4 != nil {
		log.Printf("Invalid JSON; resetting file. Error: %v\n", parseErr4)
		_ = ioutil.WriteFile(filePath, []byte("[]"), 0600)
		return []*proto.Todo{}, nil
	}
	return parseTs, nil
}

// saveTodos writes the in-memory slice back to ../_shared/data/todos.json
func saveTodos(parseTodosSlice []*proto.Todo) error {
	const filePath = "../_shared/data/todos.json"
	parseOut, parseErr := json.MarshalIndent(parseTodosSlice, "", "  ")
	if parseErr != nil {
		return parseErr
	}
	return ioutil.WriteFile(filePath, parseOut, 0600)
}

// todoServer implements the gRPC methods for TodoService
type todoServer struct {
	proto.UnimplementedTodoServiceServer
	mu    sync.Mutex
	store []*proto.Todo
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
	parseNewTodo := &proto.Todo{
		Id:   parseNewID,
		Text: parseReq.Text,
		Done: false,
	}
	parseS.store = append(parseS.store, parseNewTodo)
	_ = saveTodos(parseS.store)

	log.Printf("Created new todo: %s => %s\n", parseNewID, parseReq.Text)
	return &proto.CreateTodoResponse{Todo: parseNewTodo}, nil
}

func (parseS *todoServer) ListTodos(parseCtx context.Context, parseReq *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	parseS.mu.Lock()
	defer parseS.mu.Unlock()

	log.Printf("Listing %d todos.\n", len(parseS.store))
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
		log.Printf("No todo found with ID: %s\n", parseReq.Id)
		return &proto.UpdateTodoResponse{}, nil
	}
	_ = saveTodos(parseS.store)
	log.Printf("Updated todo: %s => %s, done=%v\n", parseUpdated.Id, parseUpdated.Text, parseUpdated.Done)
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
		log.Printf("Delete: no todo found with ID %s\n", parseReq.Id)
		return &proto.DeleteTodoResponse{Success: false}, nil
	}
	parseS.store = append(parseS.store[:parseIndex], parseS.store[parseIndex+1:]...)
	_ = saveTodos(parseS.store)

	log.Printf("Deleted todo with ID: %s\n", parseReq.Id)
	return &proto.DeleteTodoResponse{Success: true}, nil
}

func main() {
	// 1. Create the gRPC server
	parseSrv, parseErr := newTodoServer()
	if parseErr != nil {
		log.Fatalf("Failed to create todoServer: %v\n", parseErr)
	}
	parseGrpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(parseGrpcServer, parseSrv)

	parseListener, parseErr := net.Listen("tcp", "localhost:50051")
	if parseErr != nil {
		log.Fatalf("Failed to listen: %v", parseErr)
	}
	log.Printf("gRPC server listening on :50051")
	if parseErr2 := parseGrpcServer.Serve(parseListener); parseErr2 != nil {
		log.Fatalf("gRPC server failed: %v", parseErr2)
	}
}
