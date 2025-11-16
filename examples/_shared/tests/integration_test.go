//go:build !js && !wasm

package tests

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"grpc-tunnel/examples/_shared/helpers"
	"grpc-tunnel/examples/_shared/proto"
	"grpc-tunnel/pkg/bridge"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockTodoService implements a simple in-memory TodoService for testing
type mockTodoService struct {
	proto.UnimplementedTodoServiceServer
}

func (s *mockTodoService) CreateTodo(ctx context.Context, req *proto.CreateTodoRequest) (*proto.CreateTodoResponse, error) {
	return &proto.CreateTodoResponse{
		Todo: &proto.Todo{
			Id:   "test-123",
			Text: req.Text,
			Done: false,
		},
	}, nil
}

func (s *mockTodoService) ListTodos(ctx context.Context, req *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	return &proto.ListTodosResponse{
		Todos: []*proto.Todo{
			{Id: "1", Text: "Test Todo", Done: false},
		},
	}, nil
}

// TestIntegration_FullRoundtrip tests a complete client-server roundtrip
func TestIntegration_FullRoundtrip(t *testing.T) {
	// Create gRPC server with mock service
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	// Create bridge handler
	handler := helpers.ServeHandler(helpers.ServerConfig{
		GRPCServer: grpcServer,
	})

	// Start HTTP test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + server.URL[4:]

	// Create gRPC client using bridge bridge.DialOption
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		server.URL,
		bridge.DialOption(wsURL),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Create client
	client := proto.NewTodoServiceClient(conn)

	// Test CreateTodo
	createResp, err := client.CreateTodo(ctx, &proto.CreateTodoRequest{
		Text: "Integration test todo",
	})
	if err != nil {
		t.Fatalf("CreateTodo failed: %v", err)
	}

	if createResp.GetTodo().GetId() != "test-123" {
		t.Errorf("Expected ID 'test-123', got '%s'", createResp.GetTodo().GetId())
	}

	if createResp.GetTodo().GetText() != "Integration test todo" {
		t.Errorf("Expected text 'Integration test todo', got '%s'", createResp.GetTodo().GetText())
	}

	// Test ListTodos
	listResp, err := client.ListTodos(ctx, &proto.ListTodosRequest{})
	if err != nil {
		t.Fatalf("ListTodos failed: %v", err)
	}

	if len(listResp.GetTodos()) != 1 {
		t.Errorf("Expected 1 todo, got %d", len(listResp.GetTodos()))
	}
}

// TestIntegration_LifecycleHooks tests that hooks are called
func TestIntegration_LifecycleHooks(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	var connectCalled, disconnectCalled bool

	handler := helpers.ServeHandler(helpers.ServerConfig{
		GRPCServer: grpcServer,
		OnConnect: func(r *http.Request) {
			connectCalled = true
		},
		OnDisconnect: func(r *http.Request) {
			disconnectCalled = true
		},
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		server.URL,
		bridge.DialOption(wsURL),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	client := proto.NewTodoServiceClient(conn)
	_, _ = client.ListTodos(ctx, &proto.ListTodosRequest{})

	conn.Close()

	// Give time for disconnect hook
	time.Sleep(100 * time.Millisecond)

	if !connectCalled {
		t.Error("OnConnect hook was not called")
	}

	if !disconnectCalled {
		t.Error("OnDisconnect hook was not called")
	}
}

// TestIntegration_CustomBufferSizes tests with custom buffer sizes
func TestIntegration_CustomBufferSizes(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	handler := helpers.ServeHandler(helpers.ServerConfig{
		GRPCServer:      grpcServer,
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		server.URL,
		bridge.DialOption(wsURL),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)
	_, err = client.CreateTodo(ctx, &proto.CreateTodoRequest{Text: "Test"})
	if err != nil {
		t.Fatalf("CreateTodo failed: %v", err)
	}
}

// TestServe_WithListener tests the Serve convenience function
func TestServe_WithListener(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	// Create listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Start server in background
	go func() {
		_ = helpers.Serve(listener, grpcServer)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect to it
	addr := listener.Addr().String()
	wsURL := "ws://" + addr

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		addr,
		bridge.DialOption(wsURL),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)
	resp, err := client.ListTodos(ctx, &proto.ListTodosRequest{})
	if err != nil {
		t.Fatalf("ListTodos failed: %v", err)
	}

	if len(resp.GetTodos()) == 0 {
		t.Error("Expected at least one todo")
	}
}

// TestIntegration_MultipleRequests tests multiple sequential requests
func TestIntegration_MultipleRequests(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	handler := helpers.ServeHandler(helpers.ServerConfig{
		GRPCServer: grpcServer,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		server.URL,
		bridge.DialOption(wsURL),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)

	// Make multiple requests over the same connection
	for i := 0; i < 5; i++ {
		_, err := client.ListTodos(ctx, &proto.ListTodosRequest{})
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
	}
}

// TestIntegration_OriginCheck tests origin validation
func TestIntegration_OriginCheck(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	handler := helpers.ServeHandler(helpers.ServerConfig{
		GRPCServer: grpcServer,
		CheckOrigin: func(r *http.Request) bool {
			// Reject all origins
			return false
		},
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should fail due to origin check
	_, err := grpc.DialContext(
		ctx,
		server.URL,
		bridge.DialOption(wsURL),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)

	// We expect a connection error due to rejected origin
	if err == nil {
		t.Error("Expected error due to origin rejection, got nil")
	}
}
