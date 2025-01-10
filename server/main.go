package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"earlcameron.com/myservice" // Replace with your actual package path
)

// ---- gRPC Service Implementation ----

type echoServer struct {
	myservice.UnimplementedEchoServiceServer
}

func (s *echoServer) Echo(ctx context.Context, req *myservice.EchoRequest) (*myservice.EchoResponse, error) {
	log.Printf("Received gRPC request: %s\n", req.Message)
	return &myservice.EchoResponse{Message: "Echo: " + req.Message}, nil
}

// ---- WebSocket Tunnel Implementation ----

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for simplicity (consider restricting in production)
		return true
	},
}

func handleWebSocketConnection(conn *websocket.Conn, grpcClient myservice.EchoServiceClient) {
	defer conn.Close()

	for {
		// Read a binary message from the WebSocket
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v\n", err)
			break
		}

		if mt != websocket.BinaryMessage {
			log.Printf("Ignoring non-binary message.\n")
			continue
		}

		// Parse the gRPC request
		var req myservice.EchoRequest
		if err := proto.Unmarshal(message, &req); err != nil {
			log.Printf("Failed to unmarshal EchoRequest: %v\n", err)
			continue
		}

		// Call the gRPC server
		resp, err := grpcClient.Echo(context.Background(), &req)
		if err != nil {
			log.Printf("gRPC call failed: %v\n", err)
			continue
		}

		// Serialize the gRPC response
		respBytes, err := proto.Marshal(resp)
		if err != nil {
			log.Printf("Failed to marshal EchoResponse: %v\n", err)
			continue
		}

		// Send the response back over the WebSocket
		if err := conn.WriteMessage(websocket.BinaryMessage, respBytes); err != nil {
			log.Printf("WebSocket write error: %v\n", err)
			break
		}
	}
}

func startWebSocketServer(grpcClient myservice.EchoServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v\n", err)
			return
		}
		handleWebSocketConnection(conn, grpcClient)
	})

	// Static file server for the "public" folder
	publicDir := "./public"
	fs := http.FileServer(http.Dir(publicDir))
	http.Handle("/", fs)

	log.Printf("Serving static files from %s", publicDir)
	log.Println("WebSocket server listening on :8080")

	// Start the HTTP server
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("WebSocket and file server failed: %v\n", err)
	}
}

// ---- Main Function ----

func main() {
	var wg sync.WaitGroup

	// Start the gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()

		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("Failed to listen on port 50051: %v\n", err)
		}

		grpcServer := grpc.NewServer()
		myservice.RegisterEchoServiceServer(grpcServer, &echoServer{})

		log.Println("gRPC server listening on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v\n", err)
		}
	}()

	// Connect to the gRPC server for the WebSocket tunnel
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v\n", err)
	}
	defer conn.Close()
	grpcClient := myservice.NewEchoServiceClient(conn)

	// Start the WebSocket server and file server
	wg.Add(1)
	go startWebSocketServer(grpcClient, &wg)

	// Wait indefinitely
	wg.Wait()
}
