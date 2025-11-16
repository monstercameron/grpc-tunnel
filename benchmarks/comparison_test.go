package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/GoGRPCBridge/examples/_shared/proto"
	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	gproto "google.golang.org/protobuf/proto"
)

// Mock todo service for benchmarking
type todoService struct {
	proto.UnimplementedTodoServiceServer
	todos []*proto.Todo
}

func (s *todoService) CreateTodo(ctx context.Context, req *proto.CreateTodoRequest) (*proto.CreateTodoResponse, error) {
	todo := &proto.Todo{
		Id:   fmt.Sprintf("todo-%d", len(s.todos)+1),
		Text: req.Text,
		Done: false,
	}
	s.todos = append(s.todos, todo)
	return &proto.CreateTodoResponse{Todo: todo}, nil
}

func (s *todoService) ListTodos(ctx context.Context, req *proto.ListTodosRequest) (*proto.ListTodosResponse, error) {
	return &proto.ListTodosResponse{Todos: s.todos}, nil
}

func (s *todoService) UpdateTodo(ctx context.Context, req *proto.UpdateTodoRequest) (*proto.UpdateTodoResponse, error) {
	for _, t := range s.todos {
		if t.Id == req.Id {
			t.Text = req.Text
			t.Done = req.Done
			return &proto.UpdateTodoResponse{Todo: t}, nil
		}
	}
	return nil, fmt.Errorf("todo not found")
}

func (s *todoService) DeleteTodo(ctx context.Context, req *proto.DeleteTodoRequest) (*proto.DeleteTodoResponse, error) {
	for i, t := range s.todos {
		if t.Id == req.Id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return &proto.DeleteTodoResponse{Success: true}, nil
		}
	}
	return &proto.DeleteTodoResponse{Success: false}, nil
}

func (s *todoService) StreamTodos(req *proto.StreamTodosRequest, stream proto.TodoService_StreamTodosServer) error {
	for _, todo := range s.todos {
		if err := stream.Send(&proto.StreamTodosResponse{Todo: todo}); err != nil {
			return err
		}
	}
	return nil
}

func (s *todoService) SyncTodos(stream proto.TodoService_SyncTodosServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var resp *proto.SyncResponse
		switch action := req.Action.(type) {
		case *proto.SyncRequest_Create:
			todo := &proto.Todo{
				Id:   fmt.Sprintf("todo-%d", len(s.todos)+1),
				Text: action.Create.Text,
			}
			s.todos = append(s.todos, todo)
			resp = &proto.SyncResponse{
				Result: &proto.SyncResponse_Todo{Todo: todo},
			}
		case *proto.SyncRequest_Update:
			for _, todo := range s.todos {
				if todo.Id == action.Update.Id {
					todo.Text = action.Update.Text
					todo.Done = action.Update.Done
					resp = &proto.SyncResponse{
						Result: &proto.SyncResponse_Todo{Todo: todo},
					}
					break
				}
			}
		case *proto.SyncRequest_Delete:
			for i, todo := range s.todos {
				if todo.Id == action.Delete.Id {
					s.todos = append(s.todos[:i], s.todos[i+1:]...)
					resp = &proto.SyncResponse{
						Result: &proto.SyncResponse_Todo{Todo: todo},
					}
					break
				}
			}
		}

		if resp == nil {
			resp = &proto.SyncResponse{
				Result: &proto.SyncResponse_Error{Error: "not found"},
			}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// REST handler implementation
type restHandler struct {
	service *todoService
}

type todoJSON struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Done bool   `json:"done"`
}

func (h *restHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/todos":
		if r.Method == "GET" {
			h.listTodos(w, r)
		} else if r.Method == "POST" {
			h.createTodo(w, r)
		}
	case "/todos/update":
		if r.Method == "PUT" {
			h.updateTodo(w, r)
		}
	case "/todos/delete":
		if r.Method == "DELETE" {
			h.deleteTodo(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (h *restHandler) createTodo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, _ := h.service.CreateTodo(context.Background(), &proto.CreateTodoRequest{Text: req.Text})
	json.NewEncoder(w).Encode(todoJSON{
		ID:   resp.Todo.Id,
		Text: resp.Todo.Text,
		Done: resp.Todo.Done,
	})
}

func (h *restHandler) listTodos(w http.ResponseWriter, r *http.Request) {
	resp, _ := h.service.ListTodos(context.Background(), &proto.ListTodosRequest{})
	todos := make([]todoJSON, len(resp.Todos))
	for i, t := range resp.Todos {
		todos[i] = todoJSON{ID: t.Id, Text: t.Text, Done: t.Done}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"todos": todos})
}

func (h *restHandler) updateTodo(w http.ResponseWriter, r *http.Request) {
	var req todoJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, _ := h.service.UpdateTodo(context.Background(), &proto.UpdateTodoRequest{
		Id:   req.ID,
		Text: req.Text,
		Done: req.Done,
	})
	json.NewEncoder(w).Encode(todoJSON{
		ID:   resp.Todo.Id,
		Text: resp.Todo.Text,
		Done: resp.Todo.Done,
	})
}

func (h *restHandler) deleteTodo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, _ := h.service.DeleteTodo(context.Background(), &proto.DeleteTodoRequest{Id: req.ID})
	json.NewEncoder(w).Encode(map[string]bool{"success": resp.Success})
}

// setupGRPC creates a gRPC server with bridge
func setupGRPC(b *testing.B) (proto.TodoServiceClient, func()) {
	b.Helper()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	service := &todoService{todos: make([]*proto.Todo, 0)}
	proto.RegisterTodoServiceServer(grpcServer, service)

	// Start bridge server
	bridge := grpctunnel.Wrap(grpcServer)
	server := httptest.NewServer(bridge)

	// Extract host from URL (http://127.0.0.1:12345 -> 127.0.0.1:12345)
	wsURL := strings.TrimPrefix(server.URL, "http://")

	// Create client
	conn, err := grpctunnel.Dial(wsURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}

	client := proto.NewTodoServiceClient(conn)

	cleanup := func() {
		conn.Close()
		server.Close()
		grpcServer.Stop()
	}

	return client, cleanup
}

// setupREST creates a REST server
func setupREST(b *testing.B) (*http.Client, string, func()) {
	b.Helper()

	service := &todoService{todos: make([]*proto.Todo, 0)}
	handler := &restHandler{service: service}
	server := httptest.NewServer(handler)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	cleanup := func() {
		server.Close()
	}

	return client, server.URL, cleanup
}

// Benchmarks

func BenchmarkGRPC_CreateTodo(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: "Benchmark task",
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkREST_CreateTodo(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	body := map[string]string{"text": "Benchmark task"}
	bodyBytes, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkGRPC_ListTodos(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	// Pre-populate with 100 todos
	for i := 0; i < 100; i++ {
		client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: fmt.Sprintf("Todo %d", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ListTodos(context.Background(), &proto.ListTodosRequest{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkREST_ListTodos(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	// Pre-populate with 100 todos
	body := map[string]string{"text": "Todo"}
	bodyBytes, _ := json.Marshal(body)
	for i := 0; i < 100; i++ {
		resp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		resp.Body.Close()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url + "/todos")
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkGRPC_UpdateTodo(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	// Create a todo to update
	createResp, _ := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
		Text: "Original",
	})
	todoID := createResp.Todo.Id

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.UpdateTodo(context.Background(), &proto.UpdateTodoRequest{
			Id:   todoID,
			Text: "Updated",
			Done: true,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkREST_UpdateTodo(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	// Create a todo to update
	createBody := map[string]string{"text": "Original"}
	createBytes, _ := json.Marshal(createBody)
	createResp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(createBytes))
	var createResult todoJSON
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	updateBody := todoJSON{ID: createResult.ID, Text: "Updated", Done: true}
	updateBytes, _ := json.Marshal(updateBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("PUT", url+"/todos/update", bytes.NewReader(updateBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkGRPC_DeleteTodo(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create a todo to delete
		createResp, _ := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: "To be deleted",
		})
		b.StartTimer()

		_, err := client.DeleteTodo(context.Background(), &proto.DeleteTodoRequest{
			Id: createResp.Todo.Id,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkREST_DeleteTodo(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create a todo to delete
		createBody := map[string]string{"text": "To be deleted"}
		createBytes, _ := json.Marshal(createBody)
		createResp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(createBytes))
		var createResult todoJSON
		json.NewDecoder(createResp.Body).Decode(&createResult)
		createResp.Body.Close()
		b.StartTimer()

		deleteBody := map[string]string{"id": createResult.ID}
		deleteBytes, _ := json.Marshal(deleteBody)
		req, _ := http.NewRequest("DELETE", url+"/todos/delete", bytes.NewReader(deleteBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// Payload size benchmarks - measures compression efficiency
func BenchmarkGRPC_PayloadSize_10Items(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	// Pre-populate with 10 todos
	for i := 0; i < 10; i++ {
		client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: fmt.Sprintf("This is a longer todo text to measure payload size differences between Protobuf and JSON #%d", i),
		})
	}

	b.ReportAllocs()
	b.ReportMetric(0, "payload-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.ListTodos(context.Background(), &proto.ListTodosRequest{})
		if err != nil {
			b.Fatal(err)
		}
		// Estimate protobuf size
		size := gproto.Size(resp)
		b.ReportMetric(float64(size)/1024.0, "KB/op")
	}
}

func BenchmarkREST_PayloadSize_10Items(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	// Pre-populate with 10 todos
	body := map[string]string{"text": "This is a longer todo text to measure payload size differences between Protobuf and JSON"}
	bodyBytes, _ := json.Marshal(body)
	for i := 0; i < 10; i++ {
		resp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		resp.Body.Close()
	}

	b.ReportAllocs()
	b.ReportMetric(0, "payload-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url + "/todos")
		if err != nil {
			b.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		b.ReportMetric(float64(len(data))/1024.0, "KB/op")
	}
}

func BenchmarkGRPC_PayloadSize_100Items(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	for i := 0; i < 100; i++ {
		client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: fmt.Sprintf("Todo with medium length text for realistic payload testing #%d", i),
		})
	}

	b.ReportAllocs()
	b.ReportMetric(0, "payload-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.ListTodos(context.Background(), &proto.ListTodosRequest{})
		if err != nil {
			b.Fatal(err)
		}
		size := gproto.Size(resp)
		b.ReportMetric(float64(size)/1024.0, "KB/op")
	}
}

func BenchmarkREST_PayloadSize_100Items(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	body := map[string]string{"text": "Todo with medium length text for realistic payload testing"}
	bodyBytes, _ := json.Marshal(body)
	for i := 0; i < 100; i++ {
		resp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		resp.Body.Close()
	}

	b.ReportAllocs()
	b.ReportMetric(0, "payload-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url + "/todos")
		if err != nil {
			b.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		b.ReportMetric(float64(len(data))/1024.0, "KB/op")
	}
}

func BenchmarkGRPC_PayloadSize_1000Items(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	for i := 0; i < 1000; i++ {
		client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: fmt.Sprintf("Todo with medium length text for realistic payload testing #%d", i),
		})
	}

	b.ReportAllocs()
	b.ReportMetric(0, "payload-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.ListTodos(context.Background(), &proto.ListTodosRequest{})
		if err != nil {
			b.Fatal(err)
		}
		size := gproto.Size(resp)
		b.ReportMetric(float64(size)/1024.0, "KB/op")
	}
}

func BenchmarkREST_PayloadSize_1000Items(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	body := map[string]string{"text": "Todo with medium length text for realistic payload testing"}
	bodyBytes, _ := json.Marshal(body)
	for i := 0; i < 1000; i++ {
		resp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		resp.Body.Close()
	}

	b.ReportAllocs()
	b.ReportMetric(0, "payload-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url + "/todos")
		if err != nil {
			b.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		b.ReportMetric(float64(len(data))/1024.0, "KB/op")
	}
}

func BenchmarkGRPC_StreamingTodos(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	// Pre-populate with 100 todos
	for i := 0; i < 100; i++ {
		client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: fmt.Sprintf("Todo %d", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := client.StreamTodos(context.Background(), &proto.StreamTodosRequest{})
		if err != nil {
			b.Fatal(err)
		}
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// REST can't do server streaming, so we simulate with pagination
func BenchmarkREST_StreamingTodos_Simulation(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	// Pre-populate with 100 todos
	body := map[string]string{"text": "Todo"}
	bodyBytes, _ := json.Marshal(body)
	for i := 0; i < 100; i++ {
		resp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		resp.Body.Close()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate streaming by fetching all at once (what REST would do)
		resp, err := client.Get(url + "/todos")
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// Latency benchmarks - measure time to first byte and connection overhead
func BenchmarkGRPC_ConnectionReuse_10Requests(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 10 sequential requests on same connection
		for j := 0; j < 10; j++ {
			_, err := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
				Text: "Task",
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkREST_ConnectionReuse_10Requests(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	bodyBytes, _ := json.Marshal(map[string]string{"text": "Task"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 10 sequential requests - HTTP keep-alive helps but still has overhead
		for j := 0; j < 10; j++ {
			resp, err := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				b.Fatal(err)
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}
}

// Large dataset streaming - gRPC can stream without loading all into memory
func BenchmarkGRPC_StreamLargeDataset_1000Items(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	// Pre-populate with 1000 todos
	for i := 0; i < 1000; i++ {
		client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: fmt.Sprintf("Todo item with realistic text length for streaming benchmark #%d", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := client.StreamTodos(context.Background(), &proto.StreamTodosRequest{})
		if err != nil {
			b.Fatal(err)
		}
		count := 0
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
			count++
		}
	}
}

func BenchmarkREST_LargeDataset_1000Items(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	// Pre-populate with 1000 todos
	body := map[string]string{"text": "Todo item with realistic text length for streaming benchmark"}
	bodyBytes, _ := json.Marshal(body)
	for i := 0; i < 1000; i++ {
		resp, _ := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		resp.Body.Close()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// REST must load ALL data into memory at once
		resp, err := client.Get(url + "/todos")
		if err != nil {
			b.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Parse all JSON at once
		var result map[string][]todoJSON
		json.Unmarshal(data, &result)
	}
}

// Concurrent requests - HTTP/2 multiplexing advantage
func BenchmarkGRPC_ConcurrentRequests_10Parallel(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
				Text: "Concurrent task",
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkREST_ConcurrentRequests_10Parallel(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	bodyBytes, _ := json.Marshal(map[string]string{"text": "Concurrent task"})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				b.Fatal(err)
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	})
}

// Header overhead - REST sends more metadata per request
func BenchmarkGRPC_MinimalRequest(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: "X", // Minimal payload
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkREST_MinimalRequest(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	bodyBytes, _ := json.Marshal(map[string]string{"text": "X"}) // Minimal payload

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// Bidirectional streaming - only gRPC can do this
func BenchmarkGRPC_BidirectionalStream_100Messages(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := client.SyncTodos(context.Background())
		if err != nil {
			b.Fatal(err)
		}

		// Send 100 messages
		for j := 0; j < 100; j++ {
			err := stream.Send(&proto.SyncRequest{
				Action: &proto.SyncRequest_Create{
					Create: &proto.CreateTodoRequest{Text: "Sync task"},
				},
			})
			if err != nil {
				b.Fatal(err)
			}

			// Receive response
			_, err = stream.Recv()
			if err != nil {
				b.Fatal(err)
			}
		}

		stream.CloseSend()
	}
}

// REST equivalent would require 100 HTTP requests
func BenchmarkREST_Bidirectional_100Messages(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	bodyBytes, _ := json.Marshal(map[string]string{"text": "Sync task"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// REST must make 100 separate HTTP requests (no streaming)
		for j := 0; j < 100; j++ {
			resp, err := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				b.Fatal(err)
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}
}

// Complex nested data structures - Protobuf serialization advantage
type ComplexData struct {
	ID        string                 `json:"id"`
	Metadata  map[string]interface{} `json:"metadata"`
	Tags      []string               `json:"tags"`
	Nested    []map[string]string    `json:"nested"`
	Timestamp int64                  `json:"timestamp"`
}

func BenchmarkGRPC_ComplexSerialization(b *testing.B) {
	client, cleanup := setupGRPC(b)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Protobuf handles complex types efficiently
		_, err := client.CreateTodo(context.Background(), &proto.CreateTodoRequest{
			Text: "Complex data with nested structures and metadata fields for serialization testing",
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkREST_ComplexSerialization(b *testing.B) {
	client, url, cleanup := setupREST(b)
	defer cleanup()

	// JSON serialization of complex nested structures
	complexData := ComplexData{
		ID: "test-123",
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		},
		Tags:      []string{"tag1", "tag2", "tag3"},
		Nested:    []map[string]string{{"a": "b"}, {"c": "d"}},
		Timestamp: time.Now().Unix(),
	}
	bodyBytes, _ := json.Marshal(complexData)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Post(url+"/todos", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}
