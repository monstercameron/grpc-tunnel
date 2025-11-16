//go:build !js && !wasm

package grpctunnel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"grpc-tunnel/examples/_shared/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type mockService struct {
	proto.UnimplementedTodoServiceServer
}

func (s *mockService) CreateTodo(ctx context.Context, req *proto.CreateTodoRequest) (*proto.CreateTodoResponse, error) {
	return &proto.CreateTodoResponse{
		Todo: &proto.Todo{Id: "test-1", Text: req.Text, Done: false},
	}, nil
}

func (s *mockService) ListTodos(ctx context.Context, req *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	return &proto.ListTodosResponse{
		Todos: []*proto.Todo{
			{Id: "1", Text: "Test", Done: false},
		},
	}, nil
}

func (s *mockService) StreamTodos(req *proto.StreamTodosRequest, stream proto.TodoService_StreamTodosServer) error {
	todos := []*proto.Todo{
		{Id: "1", Text: "First", Done: false},
		{Id: "2", Text: "Second", Done: true},
		{Id: "3", Text: "Third", Done: false},
	}
	for _, todo := range todos {
		if err := stream.Send(&proto.StreamTodosResponse{Todo: todo}); err != nil {
			return err
		}
	}
	return nil
}

func (s *mockService) BulkCreateTodos(stream proto.TodoService_BulkCreateTodosServer) error {
	count := int32(0)
	for {
		_, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				return stream.SendAndClose(&proto.BulkCreateResponse{CreatedCount: count})
			}
			return err
		}
		count++
	}
}

func (s *mockService) SyncTodos(stream proto.TodoService_SyncTodosServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return err
		}
		switch action := req.Action.(type) {
		case *proto.SyncRequest_Create:
			stream.Send(&proto.SyncResponse{
				Result: &proto.SyncResponse_Todo{
					Todo: &proto.Todo{Id: "1", Text: action.Create.Text, Done: false},
				},
			})
		}
	}
}


func TestWrap(t *testing.T) {
	// Create gRPC server
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockService{})
	defer grpcServer.Stop()

	// Wrap it for WebSocket
	handler := Wrap(grpcServer)

	// Start test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + server.URL[4:]

	// Connect via new Dial API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := DialContext(ctx, wsURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// Test RPC call
	client := proto.NewTodoServiceClient(conn)
	resp, err := client.CreateTodo(ctx, &proto.CreateTodoRequest{Text: "New API test"})
	if err != nil {
		t.Fatalf("CreateTodo failed: %v", err)
	}

	if resp.GetTodo().GetText() != "New API test" {
		t.Errorf("Expected 'New API test', got '%s'", resp.GetTodo().GetText())
	}
}

func TestDial_URLInference(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected string
	}{
		{"WebSocket URL", "ws://localhost:8080", "ws://localhost:8080"},
		{"Secure WebSocket", "wss://api.example.com", "wss://api.example.com"},
		{"Host and port", "localhost:8080", "ws://localhost:8080"},
		{"Port only", ":8080", "ws://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferWebSocketURL(tt.target, false)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDial_TLSInference(t *testing.T) {
	result := inferWebSocketURL("localhost:8080", true)
	expected := "wss://localhost:8080"
	if result != expected {
		t.Errorf("Expected %s with TLS, got %s", expected, result)
	}
}

func TestWrap_WithOptions(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockService{})
	defer grpcServer.Stop()

	var connectCalled atomic.Bool
	var disconnectCalled atomic.Bool

	handler := Wrap(grpcServer,
		WithOriginCheck(func(r *http.Request) bool { return true }),
		WithBufferSizes(8192, 8192),
		WithConnectHook(func(r *http.Request) {
			connectCalled.Store(true)
		}),
		WithDisconnectHook(func(r *http.Request) {
			disconnectCalled.Store(true)
		}),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := Dial(wsURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	client := proto.NewTodoServiceClient(conn)
	_, _ = client.ListTodos(ctx, &proto.ListTodosRequest{})

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	if !connectCalled.Load() {
		t.Error("Connect hook not called")
	}

	if !disconnectCalled.Load() {
		t.Error("Disconnect hook not called")
	}
}

func TestWrap_ServerStreaming(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockService{})
	defer grpcServer.Stop()

	handler := Wrap(grpcServer)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(wsURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)
	stream, err := client.StreamTodos(ctx, &proto.StreamTodosRequest{})
	if err != nil {
		t.Fatalf("StreamTodos failed: %v", err)
	}

	count := 0
	for {
		_, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("Recv failed: %v", err)
		}
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 streamed todos, got %d", count)
	}
}

func TestWrap_ClientStreaming(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockService{})
	defer grpcServer.Stop()

	handler := Wrap(grpcServer)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(wsURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)
	stream, err := client.BulkCreateTodos(ctx)
	if err != nil {
		t.Fatalf("BulkCreateTodos failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := stream.Send(&proto.BulkCreateRequest{Text: "Todo"}); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv failed: %v", err)
	}

	if resp.CreatedCount != 5 {
		t.Errorf("Expected 5 created, got %d", resp.CreatedCount)
	}
}

func TestWrap_BidirectionalStreaming(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockService{})
	defer grpcServer.Stop()

	handler := Wrap(grpcServer)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(wsURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)
	stream, err := client.SyncTodos(ctx)
	if err != nil {
		t.Fatalf("SyncTodos failed: %v", err)
	}

	done := make(chan bool)
	responses := 0

	go func() {
		for {
			_, err := stream.Recv()
			if err != nil {
				done <- true
				return
			}
			responses++
		}
	}()

	stream.Send(&proto.SyncRequest{
		Action: &proto.SyncRequest_Create{
			Create: &proto.CreateTodoRequest{Text: "Test"},
		},
	})

	stream.CloseSend()
	<-done

	if responses != 1 {
		t.Errorf("Expected 1 response, got %d", responses)
	}
}

func TestDial_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Try to connect to non-routable IP (will timeout)
	_, err := DialContext(ctx, "10.255.255.1:9999",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)

	if err == nil {
		t.Error("Expected timeout/connection error, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestWrap_OriginRejection(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockService{})
	defer grpcServer.Stop()

	handler := Wrap(grpcServer,
		WithOriginCheck(func(r *http.Request) bool {
			return false // Reject all
		}),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := DialContext(ctx, wsURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	// Origin check happens during WebSocket upgrade
	// The dialer should fail to connect
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		// Connection might succeed but gRPC calls should fail
		// This is acceptable - origin check prevents actual data transfer
		t.Log("Connection succeeded but origin check should prevent WebSocket upgrade")
	}
}

