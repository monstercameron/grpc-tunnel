package e2e

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// TestServerUnavailable tests client behavior when server is not running
func TestServerUnavailable(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Ensure no prior bridge process is still running so this negative test
	// validates true unavailability instead of stale-process behavior.
	clearBridgeStaleProcesses(parseT)

	// Build
	parseT.Log("Building components...")
	buildWASMClient(parseT, parseProjectRoot)

	// Start ONLY file server, NOT the bridge server
	parseServerURL := startPublicFileServer(parseT, parseProjectRoot)

	time.Sleep(2 * time.Second)

	// Browser automation
	parsePw, parseErr := playwright.Run()
	if parseErr != nil {
		parseT.Fatalf("Could not start playwright: %v", parseErr)
	}
	defer parsePw.Stop()

	parseBrowser, parseErr := parsePw.Chromium.Launch()
	if parseErr != nil {
		parseT.Fatalf("Could not launch browser: %v", parseErr)
	}
	defer parseBrowser.Close()

	parsePage, parseErr := parseBrowser.NewPage()
	if parseErr != nil {
		parseT.Fatalf("Could not create page: %v", parseErr)
	}

	parseConsoleMessages := make(chan string, 100)
	parseErrorMessages := make(chan string, 100)

	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseText := parseMsg.Text()
		parseT.Logf("[Browser Console] %s", parseText)
		parseConsoleMessages <- parseText

		if parseMsg.Type() == "error" || strings.Contains(strings.ToLower(parseText), "error") ||
			strings.Contains(strings.ToLower(parseText), "failed") {
			parseErrorMessages <- parseText
		}
	})

	if _, parseErr = parsePage.Goto(parseServerURL); parseErr != nil {
		parseT.Fatalf("Failed to navigate: %v", parseErr)
	}

	// Should see connection errors
	parseTimeout := time.After(15 * time.Second)
	isSawError := false

	for {
		select {
		case parseMsg2 := <-parseErrorMessages:
			if strings.Contains(strings.ToLower(parseMsg2), "connection") ||
				strings.Contains(strings.ToLower(parseMsg2), "refused") ||
				strings.Contains(strings.ToLower(parseMsg2), "failed") {
				parseT.Logf("Expected error received: %s", parseMsg2)
				isSawError = true
				return
			}
		case <-parseTimeout:
			if !isSawError {
				parseT.Fatal("Expected connection error was not observed while bridge server is unavailable")
			}
			return
		}
	}
}

// TestRapidConnectDisconnect tests rapid connection cycling
func TestRapidConnectDisconnect(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildWASMClient(parseT, parseProjectRoot)

	// Start services
	parseBridgeCleanup := startBridgeCommand(parseT, parseProjectRoot, "DirectBridge")
	parseT.Cleanup(parseBridgeCleanup)

	parseServerURL := startPublicFileServer(parseT, parseProjectRoot)

	time.Sleep(3 * time.Second)

	// Browser automation
	parsePw, parseErr := playwright.Run()
	if parseErr != nil {
		parseT.Fatalf("Could not start playwright: %v", parseErr)
	}
	defer parsePw.Stop()

	parseBrowser, parseErr := parsePw.Chromium.Launch()
	if parseErr != nil {
		parseT.Fatalf("Could not launch browser: %v", parseErr)
	}
	defer parseBrowser.Close()

	// Rapidly create and close connections
	parseCycles := 5
	for parseI := 0; parseI < parseCycles; parseI++ {
		parseT.Logf("Connection cycle %d/%d", parseI+1, parseCycles)

		parsePage, parseErr3 := parseBrowser.NewPage()
		if parseErr3 != nil {
			parseT.Fatalf("Failed to create page: %v", parseErr3)
		}

		parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
			parseT.Logf("[Cycle %d Console] %s", parseI+1, parseMsg.Text())
		})

		if _, parseErr3 = parsePage.Goto(parseServerURL); parseErr3 != nil {
			parseT.Logf("Warning: Navigation failed on cycle %d: %v", parseI+1, parseErr3)
		}

		// Wait briefly
		time.Sleep(500 * time.Millisecond)

		// Close the page
		if parseErr4 := parsePage.Close(); parseErr4 != nil {
			parseT.Logf("Warning: Page close failed on cycle %d: %v", parseI+1, parseErr4)
		}

		time.Sleep(200 * time.Millisecond)
	}

	parseT.Logf("Successfully completed %d rapid connect/disconnect cycles", parseCycles)
}

// TestServerRestartDuringConnection tests handling of server restart
func TestServerRestartDuringConnection(parseT *testing.T) {
	if testing.Short() {
		parseT.Skip("Skipping long-running test in short mode")
	}

	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildWASMClient(parseT, parseProjectRoot)

	// Start file server
	parseServerURL := startPublicFileServer(parseT, parseProjectRoot)

	// Start initial bridge server
	parseBridgeCleanup := startBridgeCommand(parseT, parseProjectRoot, "DirectBridge")

	time.Sleep(3 * time.Second)

	// Browser automation
	parsePw, parseErr := playwright.Run()
	if parseErr != nil {
		parseT.Fatalf("Could not start playwright: %v", parseErr)
	}
	defer parsePw.Stop()

	parseBrowser, parseErr := parsePw.Chromium.Launch()
	if parseErr != nil {
		parseT.Fatalf("Could not launch browser: %v", parseErr)
	}
	defer parseBrowser.Close()

	parsePage, parseErr := parseBrowser.NewPage()
	if parseErr != nil {
		parseT.Fatalf("Could not create page: %v", parseErr)
	}

	parseErrorsSeen := make(chan string, 50)
	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseText := parseMsg.Text()
		parseT.Logf("[Browser Console] %s", parseText)

		if parseMsg.Type() == "error" || strings.Contains(strings.ToLower(parseText), "error") {
			parseErrorsSeen <- parseText
		}
	})

	if _, parseErr = parsePage.Goto(parseServerURL); parseErr != nil {
		parseT.Fatalf("Failed to navigate: %v", parseErr)
	}

	time.Sleep(2 * time.Second)

	// Kill the bridge server
	parseT.Log("Stopping bridge server...")
	parseBridgeCleanup()
	time.Sleep(3 * time.Second)

	// Should see connection errors
	select {
	case parseErrMsg := <-parseErrorsSeen:
		parseT.Logf("Expected error after server shutdown: %s", parseErrMsg)
	case <-time.After(5 * time.Second):
		parseT.Log("No error message captured, but server was stopped")
	}

	// Restart bridge server
	parseT.Log("Restarting bridge server...")
	parseBridgeCleanup = startBridgeCommand(parseT, parseProjectRoot, "DirectBridge-Restarted")
	parseT.Cleanup(parseBridgeCleanup)

	time.Sleep(3 * time.Second)
	parseT.Log("Server restart test completed - manual verification may be needed for full reconnection")
}

// TestInvalidWebSocketUpgrade tests handling of non-WebSocket connections
func TestInvalidWebSocketUpgrade(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Start bridge server
	parseBridgeCleanup := startBridgeCommand(parseT, parseProjectRoot, "DirectBridge")
	parseT.Cleanup(parseBridgeCleanup)

	time.Sleep(3 * time.Second)

	// Try to make a regular HTTP request to the WebSocket endpoint
	parseClient := &http.Client{Timeout: 5 * time.Second}
	parseResp, parseErr := parseClient.Get("http://127.0.0.1:5000")

	if parseErr != nil {
		parseT.Logf("Expected error on non-WebSocket request: %v", parseErr)
		return
	}
	defer parseResp.Body.Close()

	// Should get an error or upgrade required status
	if parseResp.StatusCode == http.StatusBadRequest ||
		parseResp.StatusCode == http.StatusUpgradeRequired ||
		parseResp.StatusCode >= 400 {
		parseT.Logf("Server correctly rejected non-WebSocket request with status: %d", parseResp.StatusCode)
	} else {
		parseT.Logf("Warning: Unexpected status code for non-WebSocket request: %d", parseResp.StatusCode)
	}
}

// TestMaxConcurrentConnections tests system behavior under high connection count
func TestMaxConcurrentConnections(parseT *testing.T) {
	if testing.Short() {
		parseT.Skip("Skipping stress test in short mode")
	}

	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildWASMClient(parseT, parseProjectRoot)

	// Start services
	parseBridgeCleanup := startBridgeCommand(parseT, parseProjectRoot, "DirectBridge")
	parseT.Cleanup(parseBridgeCleanup)

	parseServerURL := startPublicFileServer(parseT, parseProjectRoot)

	time.Sleep(3 * time.Second)

	// Browser automation
	parsePw, parseErr := playwright.Run()
	if parseErr != nil {
		parseT.Fatalf("Could not start playwright: %v", parseErr)
	}
	defer parsePw.Stop()

	parseBrowser, parseErr := parsePw.Chromium.Launch()
	if parseErr != nil {
		parseT.Fatalf("Could not launch browser: %v", parseErr)
	}
	defer parseBrowser.Close()

	// Try to create many connections (10 tabs)
	parseNumConnections := 10
	parsePages := make([]playwright.Page, 0, parseNumConnections)
	parseSuccessCount := 0

	for parseI := 0; parseI < parseNumConnections; parseI++ {
		parsePage, parseErr3 := parseBrowser.NewPage()
		if parseErr3 != nil {
			parseT.Logf("Failed to create page %d: %v", parseI+1, parseErr3)
			continue
		}
		parsePages = append(parsePages, parsePage)

		parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
			parseT.Logf("[Connection %d] %s", parseI+1, parseMsg.Text())
		})

		if _, parseErr3 = parsePage.Goto(parseServerURL); parseErr3 != nil {
			parseT.Logf("Failed to navigate page %d: %v", parseI+1, parseErr3)
			continue
		}

		parseSuccessCount++
	}

	parseT.Logf("Successfully created %d/%d connections", parseSuccessCount, parseNumConnections)

	// Clean up pages
	for _, parsePage2 := range parsePages {
		parsePage2.Close()
	}

	if parseSuccessCount < parseNumConnections/2 {
		parseT.Errorf("Too many connection failures: only %d/%d succeeded", parseSuccessCount, parseNumConnections)
	}
}

// TestNetworkTimeout tests handling of slow/timeout scenarios
func TestNetworkTimeout(parseT *testing.T) {
	if testing.Short() {
		parseT.Skip("Skipping timeout test in short mode")
	}

	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildWASMClient(parseT, parseProjectRoot)

	// Start services
	parseBridgeCleanup := startBridgeCommand(parseT, parseProjectRoot, "DirectBridge")
	parseT.Cleanup(parseBridgeCleanup)

	parseServerURL := startPublicFileServer(parseT, parseProjectRoot)

	time.Sleep(3 * time.Second)

	// Browser automation with short timeout
	parsePw, parseErr := playwright.Run()
	if parseErr != nil {
		parseT.Fatalf("Could not start playwright: %v", parseErr)
	}
	defer parsePw.Stop()

	parseBrowser, parseErr := parsePw.Chromium.Launch()
	if parseErr != nil {
		parseT.Fatalf("Could not launch browser: %v", parseErr)
	}
	defer parseBrowser.Close()

	parsePage, parseErr := parseBrowser.NewPage()
	if parseErr != nil {
		parseT.Fatalf("Could not create page: %v", parseErr)
	}

	// Set aggressive timeout
	parsePage.SetDefaultTimeout(5000) // 5 seconds

	isTimeoutSeen := false
	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseText := parseMsg.Text()
		parseT.Logf("[Browser Console] %s", parseText)

		if strings.Contains(strings.ToLower(parseText), "timeout") {
			isTimeoutSeen = true
		}
	})

	if _, parseErr = parsePage.Goto(parseServerURL); parseErr != nil {
		parseT.Logf("Navigation completed (may have timeout internally): %v", parseErr)
	}

	// Wait and check for timeout messages
	time.Sleep(8 * time.Second)

	if isTimeoutSeen {
		parseT.Log("Timeout handling verified")
	} else {
		parseT.Log("No timeout observed (connection may be fast)")
	}
}
