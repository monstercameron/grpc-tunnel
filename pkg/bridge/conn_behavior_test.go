package bridge

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketConn_EnforcesReadDeadline verifies Read returns on timeout when a read deadline is set.
func TestWebSocketConn_EnforcesReadDeadline(parseT *testing.T) {
	parseUpgrader := websocket.Upgrader{
		CheckOrigin: func(parseR *http.Request) bool { return true },
	}

	parseServer := httptest.NewServer(http.HandlerFunc(func(parseW http.ResponseWriter, parseR *http.Request) {
		parseWs, parseErr := parseUpgrader.Upgrade(parseW, parseR, nil)
		if parseErr != nil {
			return
		}
		defer parseWs.Close()

		// Keep the socket open without sending frames so the client must rely on deadline behavior.
		time.Sleep(500 * time.Millisecond)
	}))
	defer parseServer.Close()

	parseWsURL := "ws" + strings.TrimPrefix(parseServer.URL, "http")
	parseClientWs, _, parseErr := websocket.DefaultDialer.Dial(parseWsURL, nil)
	if parseErr != nil {
		parseT.Fatalf("Dial failed: %v", parseErr)
	}
	defer parseClientWs.Close()

	parseConn := NewWebSocketConn(parseClientWs)
	defer parseConn.Close()

	if parseErr = parseConn.SetReadDeadline(time.Now().Add(75 * time.Millisecond)); parseErr != nil {
		parseT.Fatalf("SetReadDeadline failed: %v", parseErr)
	}

	parseStartTime := time.Now()
	parseBuffer := make([]byte, 1)
	_, parseReadErr := parseConn.Read(parseBuffer)
	if parseReadErr == nil {
		parseT.Fatal("Expected read deadline error, got nil")
	}

	parseElapsed := time.Since(parseStartTime)
	if parseElapsed > 350*time.Millisecond {
		parseT.Fatalf("Read deadline not enforced promptly, elapsed=%s err=%v", parseElapsed, parseReadErr)
	}

	parseNetErr, isNetErr := parseReadErr.(net.Error)
	if !isNetErr && !strings.Contains(strings.ToLower(parseReadErr.Error()), "timeout") {
		parseT.Fatalf("Expected timeout-style error, got %v", parseReadErr)
	}
	if isNetErr && !parseNetErr.Timeout() {
		parseT.Fatalf("Expected timeout net.Error, got %v", parseReadErr)
	}
}

// TestWebSocketConn_ConcurrentWriteSafety verifies concurrent Write calls do not panic.
func TestWebSocketConn_ConcurrentWriteSafety(parseT *testing.T) {
	parseUpgrader := websocket.Upgrader{
		CheckOrigin: func(parseR *http.Request) bool { return true },
	}

	parseServer := httptest.NewServer(http.HandlerFunc(func(parseW http.ResponseWriter, parseR *http.Request) {
		parseWs, parseErr := parseUpgrader.Upgrade(parseW, parseR, nil)
		if parseErr != nil {
			return
		}
		defer parseWs.Close()

		for {
			if _, _, parseErr = parseWs.ReadMessage(); parseErr != nil {
				return
			}
		}
	}))
	defer parseServer.Close()

	parseWsURL := "ws" + strings.TrimPrefix(parseServer.URL, "http")
	parseClientWs, _, parseErr := websocket.DefaultDialer.Dial(parseWsURL, nil)
	if parseErr != nil {
		parseT.Fatalf("Dial failed: %v", parseErr)
	}
	defer parseClientWs.Close()

	parseConn := NewWebSocketConn(parseClientWs)
	defer parseConn.Close()

	parsePanicChannel := make(chan interface{}, 2)
	parseErrorChannel := make(chan error, 2)

	// Run overlapping write loops to exercise concurrent writer behavior.
	parseRunWriter := func() {
		defer func() {
			if parseRecovered := recover(); parseRecovered != nil {
				parsePanicChannel <- parseRecovered
			}
		}()

		for parseIndex := 0; parseIndex < 300; parseIndex++ {
			if _, parseWriteErr := parseConn.Write([]byte("x")); parseWriteErr != nil {
				parseErrorChannel <- parseWriteErr
				return
			}
		}
	}

	var parseWg sync.WaitGroup
	parseWg.Add(2)
	go func() {
		defer parseWg.Done()
		parseRunWriter()
	}()
	go func() {
		defer parseWg.Done()
		parseRunWriter()
	}()
	parseWg.Wait()

	select {
	case parseRecovered := <-parsePanicChannel:
		parseT.Fatalf("Concurrent write panicked: %v", parseRecovered)
	default:
	}

	select {
	case parseWriteErr := <-parseErrorChannel:
		parseT.Fatalf("Concurrent write returned error: %v", parseWriteErr)
	default:
	}
}
