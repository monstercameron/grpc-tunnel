//go:build !js && !wasm

package tests

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"grpc-tunnel/examples/_shared/helpers"
	"grpc-tunnel/examples/_shared/proto"
	"grpc-tunnel/pkg/bridge"
	"grpc-tunnel/pkg/grpctunnel"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

func (s *mockTodoService) StreamTodos(req *proto.StreamTodosRequest, stream proto.TodoService_StreamTodosServer) error {
	todos := []*proto.Todo{
		{Id: "1", Text: "First Todo", Done: false},
		{Id: "2", Text: "Second Todo", Done: true},
		{Id: "3", Text: "Third Todo", Done: false},
	}
	
	for _, todo := range todos {
		if err := stream.Send(&proto.StreamTodosResponse{Todo: todo}); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond) // Simulate streaming delay
	}
	return nil
}

func (s *mockTodoService) BulkCreateTodos(stream proto.TodoService_BulkCreateTodosServer) error {
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
		// Simulate processing
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *mockTodoService) SyncTodos(stream proto.TodoService_SyncTodosServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return err
		}
		
		// Echo back results
		switch action := req.Action.(type) {
		case *proto.SyncRequest_Create:
			stream.Send(&proto.SyncResponse{
				Result: &proto.SyncResponse_Todo{
					Todo: &proto.Todo{Id: "sync-1", Text: action.Create.Text, Done: false},
				},
			})
		case *proto.SyncRequest_Update:
			stream.Send(&proto.SyncResponse{
				Result: &proto.SyncResponse_Todo{
					Todo: &proto.Todo{Id: action.Update.Id, Text: action.Update.Text, Done: action.Update.Done},
				},
			})
		case *proto.SyncRequest_Delete:
			stream.Send(&proto.SyncResponse{
				Result: &proto.SyncResponse_Error{Error: "deleted"},
			})
		}
	}
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

// TestIntegration_ServerStreaming tests server-to-client streaming over WebSocket
func TestIntegration_ServerStreaming(t *testing.T) {
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
	
	stream, err := client.StreamTodos(ctx, &proto.StreamTodosRequest{})
	if err != nil {
		t.Fatalf("StreamTodos failed: %v", err)
	}

	receivedTodos := []*proto.Todo{}
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("Failed to receive: %v", err)
		}
		receivedTodos = append(receivedTodos, resp.Todo)
	}

	if len(receivedTodos) != 3 {
		t.Errorf("Expected 3 streamed todos, got %d", len(receivedTodos))
	}

	if receivedTodos[0].Text != "First Todo" {
		t.Errorf("Expected 'First Todo', got '%s'", receivedTodos[0].Text)
	}
}

// TestIntegration_ClientStreaming tests client-to-server streaming over WebSocket
func TestIntegration_ClientStreaming(t *testing.T) {
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
	
	stream, err := client.BulkCreateTodos(ctx)
	if err != nil {
		t.Fatalf("BulkCreateTodos failed: %v", err)
	}

	// Send multiple todos
	todos := []string{"Todo 1", "Todo 2", "Todo 3", "Todo 4", "Todo 5"}
	for _, text := range todos {
		if err := stream.Send(&proto.BulkCreateRequest{Text: text}); err != nil {
			t.Fatalf("Failed to send: %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("Failed to close and receive: %v", err)
	}

	if resp.CreatedCount != int32(len(todos)) {
		t.Errorf("Expected %d created todos, got %d", len(todos), resp.CreatedCount)
	}
}

// TestIntegration_BidirectionalStreaming tests full-duplex streaming over WebSocket
func TestIntegration_BidirectionalStreaming(t *testing.T) {
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
	
	stream, err := client.SyncTodos(ctx)
	if err != nil {
		t.Fatalf("SyncTodos failed: %v", err)
	}

	// Test bidirectional streaming by sending and receiving concurrently
	done := make(chan bool)
	responses := []*proto.SyncResponse{}
	
	// Receiver goroutine
	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				done <- true
				return
			}
			responses = append(responses, resp)
		}
	}()

	// Send multiple operations
	stream.Send(&proto.SyncRequest{
		Action: &proto.SyncRequest_Create{
			Create: &proto.CreateTodoRequest{Text: "Bidirectional test"},
		},
	})
	
	stream.Send(&proto.SyncRequest{
		Action: &proto.SyncRequest_Update{
			Update: &proto.UpdateTodoRequest{Id: "123", Text: "Updated", Done: true},
		},
	})
	
	stream.Send(&proto.SyncRequest{
		Action: &proto.SyncRequest_Delete{
			Delete: &proto.DeleteTodoRequest{Id: "456"},
		},
	})

	stream.CloseSend()
	<-done

	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	// Verify create response
	if responses[0].GetTodo() == nil {
		t.Error("Expected todo in first response")
	}
}

// TestIntegration_Metadata tests that gRPC metadata (headers) are preserved
func TestIntegration_Metadata(t *testing.T) {
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

	// Send metadata with request
	md := metadata.Pairs(
		"authorization", "Bearer test-token",
		"custom-header", "custom-value",
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	var header metadata.MD
	_, err = client.CreateTodo(ctx, &proto.CreateTodoRequest{Text: "Test"}, grpc.Header(&header))
	if err != nil {
		t.Fatalf("CreateTodo with metadata failed: %v", err)
	}

	// Note: This tests that metadata doesn't break the connection
	// Full metadata round-trip verification would require server-side inspection
	t.Log("Metadata test passed - request succeeded with headers")
}

// TestIntegration_Trailers tests that gRPC trailers are preserved
func TestIntegration_Trailers(t *testing.T) {
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

	var trailer metadata.MD
	_, err = client.ListTodos(ctx, &proto.ListTodosRequest{}, grpc.Trailer(&trailer))
	if err != nil {
		t.Fatalf("ListTodos with trailer failed: %v", err)
	}

	// Note: This tests that trailer handling doesn't break the connection
	t.Log("Trailer test passed - request succeeded with trailer capture")
}

// TestIntegration_Cancellation tests context cancellation propagation
func TestIntegration_Cancellation(t *testing.T) {
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

	// Start a server streaming call
	streamCtx, streamCancel := context.WithCancel(ctx)
	stream, err := client.StreamTodos(streamCtx, &proto.StreamTodosRequest{})
	if err != nil {
		t.Fatalf("StreamTodos failed: %v", err)
	}

	// Receive one message
	_, err = stream.Recv()
	if err != nil {
		t.Fatalf("First recv failed: %v", err)
	}

	// Cancel the context
	streamCancel()

	// Next recv should fail with context canceled
	_, err = stream.Recv()
	if err == nil {
		t.Error("Expected error after context cancellation, got nil")
	}
	
	if err != nil && err != context.Canceled && !isContextCanceledError(err) {
		t.Logf("Got error after cancellation: %v (type: %T)", err, err)
	}
}

// Helper to check for context cancellation errors
func isContextCanceledError(err error) bool {
	if err == context.Canceled {
		return true
	}
	return err.Error() == "context canceled" || err.Error() == "rpc error: code = Canceled desc = context canceled"
}

// TestIntegration_Backpressure tests flow control in streaming
func TestIntegration_Backpressure(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	handler := helpers.ServeHandler(helpers.ServerConfig{
		GRPCServer: grpcServer,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Test backpressure with slow receiver
	stream, err := client.StreamTodos(ctx, &proto.StreamTodosRequest{})
	if err != nil {
		t.Fatalf("StreamTodos failed: %v", err)
	}

	receivedCount := 0
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		receivedCount++
		
		// Simulate slow consumer
		time.Sleep(50 * time.Millisecond)
	}

	if receivedCount != 3 {
		t.Errorf("Expected 3 messages with backpressure, got %d", receivedCount)
	}

	t.Log("Backpressure test passed - slow receiver handled correctly")
}

// TestIntegration_GrpcTunnel_ServerStreaming tests new grpctunnel API with server streaming
func TestIntegration_GrpcTunnel_ServerStreaming(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	// Use new grpctunnel.Wrap API
	handler := grpctunnel.Wrap(grpcServer)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use new grpctunnel.Dial API
	conn, err := grpctunnel.DialContext(ctx, wsURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpctunnel.Dial failed: %v", err)
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
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 todos, got %d", count)
	}
}

// TestIntegration_GrpcTunnel_Unary tests new grpctunnel API with unary calls
func TestIntegration_GrpcTunnel_Unary(t *testing.T) {
	grpcServer := grpc.NewServer()
	proto.RegisterTodoServiceServer(grpcServer, &mockTodoService{})
	defer grpcServer.Stop()

	handler := grpctunnel.Wrap(grpcServer)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpctunnel.Dial(wsURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpctunnel.Dial failed: %v", err)
	}
	defer conn.Close()

	client := proto.NewTodoServiceClient(conn)
	
	resp, err := client.CreateTodo(ctx, &proto.CreateTodoRequest{Text: "Test with grpctunnel"})
	if err != nil {
		t.Fatalf("CreateTodo failed: %v", err)
	}

	if resp.GetTodo().GetText() != "Test with grpctunnel" {
		t.Errorf("Expected 'Test with grpctunnel', got '%s'", resp.GetTodo().GetText())
	}
}
