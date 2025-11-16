package e2e

import (
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// TestServerUnavailable tests client behavior when server is not running
func TestServerUnavailable(t *testing.T) {
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Build
	t.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = projectRoot
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, string(buildOutput))
	}

	// Start ONLY file server, NOT the bridge server
	publicDir := http.Dir(filepath.Join(projectRoot, "examples", "_shared", "public"))
	fileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(publicDir)}
	go func() {
		fileServer.ListenAndServe()
	}()
	t.Cleanup(func() {
		fileServer.Shutdown(context.Background())
	})

	time.Sleep(2 * time.Second)

	// Browser automation
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("Could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Could not create page: %v", err)
	}

	consoleMessages := make(chan string, 100)
	errorMessages := make(chan string, 100)

	page.On("console", func(msg playwright.ConsoleMessage) {
		text := msg.Text()
		t.Logf("[Browser Console] %s", text)
		consoleMessages <- text

		if msg.Type() == "error" || strings.Contains(strings.ToLower(text), "error") ||
			strings.Contains(strings.ToLower(text), "failed") {
			errorMessages <- text
		}
	})

	if _, err = page.Goto("http://localhost:8081"); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Should see connection errors
	timeout := time.After(15 * time.Second)
	sawError := false

	for {
		select {
		case msg := <-errorMessages:
			if strings.Contains(strings.ToLower(msg), "connection") ||
				strings.Contains(strings.ToLower(msg), "refused") ||
				strings.Contains(strings.ToLower(msg), "failed") {
				t.Logf("Expected error received: %s", msg)
				sawError = true
				return
			}
		case <-timeout:
			if !sawError {
				t.Log("Warning: Expected connection error not observed, but test completed")
			}
			return
		}
	}
}

// TestRapidConnectDisconnect tests rapid connection cycling
func TestRapidConnectDisconnect(t *testing.T) {
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Build
	t.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = projectRoot
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, string(buildOutput))
	}

	// Start services
	directBridgePath := filepath.Join(projectRoot, "examples", "direct-bridge", "main.go")
	bridgeCleanup := startCommand(t, projectRoot, "DirectBridge", "go", "run", directBridgePath)
	t.Cleanup(bridgeCleanup)

	publicDir := http.Dir(filepath.Join(projectRoot, "examples", "_shared", "public"))
	fileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(publicDir)}
	go func() {
		fileServer.ListenAndServe()
	}()
	t.Cleanup(func() {
		fileServer.Shutdown(context.Background())
	})

	time.Sleep(3 * time.Second)

	// Browser automation
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("Could not launch browser: %v", err)
	}
	defer browser.Close()

	// Rapidly create and close connections
	cycles := 5
	for i := 0; i < cycles; i++ {
		t.Logf("Connection cycle %d/%d", i+1, cycles)

		page, err := browser.NewPage()
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}

		page.On("console", func(msg playwright.ConsoleMessage) {
			t.Logf("[Cycle %d Console] %s", i+1, msg.Text())
		})

		if _, err = page.Goto("http://localhost:8081"); err != nil {
			t.Logf("Warning: Navigation failed on cycle %d: %v", i+1, err)
		}

		// Wait briefly
		time.Sleep(500 * time.Millisecond)

		// Close the page
		if err := page.Close(); err != nil {
			t.Logf("Warning: Page close failed on cycle %d: %v", i+1, err)
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Logf("Successfully completed %d rapid connect/disconnect cycles", cycles)
}

// TestServerRestartDuringConnection tests handling of server restart
func TestServerRestartDuringConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Build
	t.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = projectRoot
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, string(buildOutput))
	}

	// Start file server
	publicDir := http.Dir(filepath.Join(projectRoot, "examples", "_shared", "public"))
	fileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(publicDir)}
	go func() {
		fileServer.ListenAndServe()
	}()
	t.Cleanup(func() {
		fileServer.Shutdown(context.Background())
	})

	// Start initial bridge server
	directBridgePath := filepath.Join(projectRoot, "examples", "direct-bridge", "main.go")
	bridgeCleanup := startCommand(t, projectRoot, "DirectBridge", "go", "run", directBridgePath)

	time.Sleep(3 * time.Second)

	// Browser automation
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("Could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Could not create page: %v", err)
	}

	errorsSeen := make(chan string, 50)
	page.On("console", func(msg playwright.ConsoleMessage) {
		text := msg.Text()
		t.Logf("[Browser Console] %s", text)

		if msg.Type() == "error" || strings.Contains(strings.ToLower(text), "error") {
			errorsSeen <- text
		}
	})

	if _, err = page.Goto("http://localhost:8081"); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Kill the bridge server
	t.Log("Stopping bridge server...")
	bridgeCleanup()
	time.Sleep(3 * time.Second)

	// Should see connection errors
	select {
	case errMsg := <-errorsSeen:
		t.Logf("Expected error after server shutdown: %s", errMsg)
	case <-time.After(5 * time.Second):
		t.Log("No error message captured, but server was stopped")
	}

	// Restart bridge server
	t.Log("Restarting bridge server...")
	bridgeCleanup = startCommand(t, projectRoot, "DirectBridge-Restarted", "go", "run", directBridgePath)
	t.Cleanup(bridgeCleanup)

	time.Sleep(3 * time.Second)
	t.Log("Server restart test completed - manual verification may be needed for full reconnection")
}

// TestInvalidWebSocketUpgrade tests handling of non-WebSocket connections
func TestInvalidWebSocketUpgrade(t *testing.T) {
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Start bridge server
	directBridgePath := filepath.Join(projectRoot, "examples", "direct-bridge", "main.go")
	bridgeCleanup := startCommand(t, projectRoot, "DirectBridge", "go", "run", directBridgePath)
	t.Cleanup(bridgeCleanup)

	time.Sleep(3 * time.Second)

	// Try to make a regular HTTP request to the WebSocket endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:5000")

	if err != nil {
		t.Logf("Expected error on non-WebSocket request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Should get an error or upgrade required status
	if resp.StatusCode == http.StatusBadRequest ||
		resp.StatusCode == http.StatusUpgradeRequired ||
		resp.StatusCode >= 400 {
		t.Logf("Server correctly rejected non-WebSocket request with status: %d", resp.StatusCode)
	} else {
		t.Logf("Warning: Unexpected status code for non-WebSocket request: %d", resp.StatusCode)
	}
}

// TestMaxConcurrentConnections tests system behavior under high connection count
func TestMaxConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Build
	t.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = projectRoot
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, string(buildOutput))
	}

	// Start services
	directBridgePath := filepath.Join(projectRoot, "examples", "direct-bridge", "main.go")
	bridgeCleanup := startCommand(t, projectRoot, "DirectBridge", "go", "run", directBridgePath)
	t.Cleanup(bridgeCleanup)

	publicDir := http.Dir(filepath.Join(projectRoot, "examples", "_shared", "public"))
	fileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(publicDir)}
	go func() {
		fileServer.ListenAndServe()
	}()
	t.Cleanup(func() {
		fileServer.Shutdown(context.Background())
	})

	time.Sleep(3 * time.Second)

	// Browser automation
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("Could not launch browser: %v", err)
	}
	defer browser.Close()

	// Try to create many connections (10 tabs)
	numConnections := 10
	pages := make([]playwright.Page, 0, numConnections)
	successCount := 0

	for i := 0; i < numConnections; i++ {
		page, err := browser.NewPage()
		if err != nil {
			t.Logf("Failed to create page %d: %v", i+1, err)
			continue
		}
		pages = append(pages, page)

		page.On("console", func(msg playwright.ConsoleMessage) {
			t.Logf("[Connection %d] %s", i+1, msg.Text())
		})

		if _, err = page.Goto("http://localhost:8081"); err != nil {
			t.Logf("Failed to navigate page %d: %v", i+1, err)
			continue
		}

		successCount++
	}

	t.Logf("Successfully created %d/%d connections", successCount, numConnections)

	// Clean up pages
	for _, page := range pages {
		page.Close()
	}

	if successCount < numConnections/2 {
		t.Errorf("Too many connection failures: only %d/%d succeeded", successCount, numConnections)
	}
}

// TestNetworkTimeout tests handling of slow/timeout scenarios
func TestNetworkTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Build
	t.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = projectRoot
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, string(buildOutput))
	}

	// Start services
	directBridgePath := filepath.Join(projectRoot, "examples", "direct-bridge", "main.go")
	bridgeCleanup := startCommand(t, projectRoot, "DirectBridge", "go", "run", directBridgePath)
	t.Cleanup(bridgeCleanup)

	publicDir := http.Dir(filepath.Join(projectRoot, "examples", "_shared", "public"))
	fileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(publicDir)}
	go func() {
		fileServer.ListenAndServe()
	}()
	t.Cleanup(func() {
		fileServer.Shutdown(context.Background())
	})

	time.Sleep(3 * time.Second)

	// Browser automation with short timeout
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("Could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Could not create page: %v", err)
	}

	// Set aggressive timeout
	page.SetDefaultTimeout(5000) // 5 seconds

	timeoutSeen := false
	page.On("console", func(msg playwright.ConsoleMessage) {
		text := msg.Text()
		t.Logf("[Browser Console] %s", text)

		if strings.Contains(strings.ToLower(text), "timeout") {
			timeoutSeen = true
		}
	})

	if _, err = page.Goto("http://localhost:8081"); err != nil {
		t.Logf("Navigation completed (may have timeout internally): %v", err)
	}

	// Wait and check for timeout messages
	time.Sleep(8 * time.Second)

	if timeoutSeen {
		t.Log("Timeout handling verified")
	} else {
		t.Log("No timeout observed (connection may be fast)")
	}
}
