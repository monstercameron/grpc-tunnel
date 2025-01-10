package grpcwsserver

import (
	"compress/gzip"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"earlcameron.com/todos" // Adjust to match your module path
)

// -----------------------------------------------------------------------------
// Gzip Middleware
// -----------------------------------------------------------------------------

// gzipResponseWriter wraps http.ResponseWriter and redirects .Write(...) to a gzip.Writer.
type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (grw gzipResponseWriter) Write(b []byte) (int, error) {
	return grw.Writer.Write(b)
}

// gzipMiddleware checks if the client accepts gzip encoding. If so, it compresses the response.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the client supports gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// If not, just call the next handler
			next.ServeHTTP(w, r)
			return
		}

		// Mark the response as gzip-encoded
		w.Header().Set("Content-Encoding", "gzip")

		// Create a gzip writer and close it when done
		gz := gzip.NewWriter(w)
		defer gz.Close()

		// Wrap the ResponseWriter
		grw := gzipResponseWriter{
			ResponseWriter: w,
			Writer:         gz,
		}

		// Serve the request via the wrapped writer
		next.ServeHTTP(grw, r)
	})
}

// -----------------------------------------------------------------------------
// WebSocket + gRPC bridging
// -----------------------------------------------------------------------------

// Upgrader for WebSocket
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for simplicity; restrict in production.
		return true
	},
}

// StartGRPCServer starts a gRPC server on the specified address.
func StartGRPCServer(grpcServer *grpc.Server, addr string, wg *sync.WaitGroup) {
	defer wg.Done()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	log.Printf("gRPC server listening on %s", addr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

// ConnectGRPCClient dials the gRPC server and returns a TodoServiceClient.
func ConnectGRPCClient(addr string) (*grpc.ClientConn, todos.TodoServiceClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	client := todos.NewTodoServiceClient(conn)
	return conn, client, nil
}

// StartWebSocketServer starts an HTTP server with WebSocket support and Gzip-compressed static files.
func StartWebSocketServer(todoClient todos.TodoServiceClient, addr, staticDir string, wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. Create a FileServer for serving static files
	fs := http.FileServer(http.Dir(staticDir))

	// 2. Wrap it with our gzipMiddleware
	compressedFS := gzipMiddleware(fs)

	// 3. Serve the compressed file server on "/"
	http.Handle("/", compressedFS)

	// 4. WebSocket endpoint on "/ws"
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		handleWebSocketConnection(conn, todoClient)
	})

	log.Printf("WebSocket server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("WebSocket server failed: %v", err)
	}
}

// handleWebSocketConnection processes incoming WebSocket messages.
func handleWebSocketConnection(conn *websocket.Conn, client todos.TodoServiceClient) {
	defer conn.Close()

	log.Println("WebSocket connection established.")
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		if msgType != websocket.BinaryMessage || len(data) < 1 {
			log.Println("Invalid WebSocket message received.")
			continue
		}

		methodID := data[0]
		payload := data[1:]
		log.Printf("Processing method ID %d", methodID)

		switch methodID {
		case 0: // CreateTodo
			var req todos.CreateTodoRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal CreateTodoRequest: %v", err)
				continue
			}
			resp, err := client.CreateTodo(context.Background(), &req)
			if err != nil {
				log.Printf("CreateTodo failed: %v", err)
				continue
			}
			sendResponse(conn, 0, resp)

		case 1: // ListTodos
			var req todos.ListTodosRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal ListTodosRequest: %v", err)
				continue
			}
			resp, err := client.ListTodos(context.Background(), &req)
			if err != nil {
				log.Printf("ListTodos failed: %v", err)
				continue
			}
			sendResponse(conn, 1, resp)

		case 2: // UpdateTodo
			var req todos.UpdateTodoRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal UpdateTodoRequest: %v", err)
				continue
			}
			resp, err := client.UpdateTodo(context.Background(), &req)
			if err != nil {
				log.Printf("UpdateTodo failed: %v", err)
				continue
			}
			sendResponse(conn, 2, resp)

		case 3: // DeleteTodo
			var req todos.DeleteTodoRequest
			if err := proto.Unmarshal(payload, &req); err != nil {
				log.Printf("Failed to unmarshal DeleteTodoRequest: %v", err)
				continue
			}
			resp, err := client.DeleteTodo(context.Background(), &req)
			if err != nil {
				log.Printf("DeleteTodo failed: %v", err)
				continue
			}
			sendResponse(conn, 3, resp)

		default:
			log.Printf("Unknown method ID: %d", methodID)
		}
	}
}

// sendResponse marshals the gRPC response and sends it back via WebSocket.
func sendResponse(conn *websocket.Conn, methodID byte, resp proto.Message) {
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	finalData := append([]byte{methodID}, respBytes...)
	if err := conn.WriteMessage(websocket.BinaryMessage, finalData); err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}
