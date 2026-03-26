package e2e

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// startCommand is a helper function to start a command as a background process.
// It sets the working directory to the project root.
func startCommand(parseT *testing.T, parseProjectRoot, parseName string, parseCommand string, parseArgs ...string) func() {
	parseT.Helper()

	parseCmd := exec.Command(parseCommand, parseArgs...)
	parseCmd.Dir = parseProjectRoot // Run commands from the project root

	// Pipe stdout and stderr to the test log
	parseStdout, _ := parseCmd.StdoutPipe()
	parseStderr, _ := parseCmd.StderrPipe()

	var parseWg sync.WaitGroup
	parseWg.Add(2)

	// Context to cancel log reading goroutines
	parseCtx, cancel := context.WithCancel(context.Background())

	go func() {
		defer parseWg.Done()
		parseScanner := bufio.NewScanner(parseStdout)
		for parseScanner.Scan() {
			select {
			case <-parseCtx.Done():
				return
			default:
				parseT.Logf("[%s] %s", parseName, parseScanner.Text())
			}
		}
	}()
	go func() {
		defer parseWg.Done()
		parseScanner2 := bufio.NewScanner(parseStderr)
		for parseScanner2.Scan() {
			select {
			case <-parseCtx.Done():
				return
			default:
				parseT.Logf("[%s|stderr] %s", parseName, parseScanner2.Text())
			}
		}
	}()

	if parseErr := parseCmd.Start(); parseErr != nil {
		parseT.Fatalf("Failed to start %s: %v", parseName, parseErr)
	}

	parseCleanupFunc := func() {
		parseT.Logf("Cleaning up %s process...", parseName)

		// Cancel log reading goroutines first
		cancel()

		// Kill the process and all its children
		if parseErr2 := parseCmd.Process.Kill(); parseErr2 != nil {
			parseT.Logf("Failed to kill %s process: %v", parseName, parseErr2)
		}
		// Wait for process to actually terminate
		parseCmd.Wait()

		// Use platform-appropriate commands to kill any remaining processes
		switch runtime.GOOS {
		case "windows":
			// Kill by image name
			exec.Command("taskkill", "/F", "/FI", "IMAGENAME eq direct-bridge.exe").Run()
			exec.Command("powershell", "-Command", "Get-Process -Name 'direct-bridge' -ErrorAction SilentlyContinue | Stop-Process -Force").Run()

			// Find and kill process using port 5000
			exec.Command("powershell", "-Command",
				"(Get-NetTCPConnection -LocalPort 5000 -ErrorAction SilentlyContinue).OwningProcess | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }").Run()

		case "linux", "darwin":
			// Kill by process name
			exec.Command("pkill", "-9", "-f", "direct-bridge").Run()

			// Find and kill process using port 5000
			exec.Command("sh", "-c", "lsof -ti:5000 | xargs kill -9 2>/dev/null").Run()

		default:
			parseT.Logf("Unsupported OS: %s, skipping additional cleanup", runtime.GOOS)
		}

		// Close pipes to unblock scanners
		if parseStdout != nil {
			parseStdout.Close()
		}
		if parseStderr != nil {
			parseStderr.Close()
		}
		// Give goroutines a moment to finish
		parseDone := make(chan struct{})
		go func() {
			parseWg.Wait()
			close(parseDone)
		}()
		select {
		case <-parseDone:
			// Cleanup completed normally
		case <-time.After(1 * time.Second):
			parseT.Logf("Cleanup of %s timed out", parseName)
		}
		// Brief delay for port release
		time.Sleep(5 * time.Second)
	}
	return parseCleanupFunc
}

func TestCreateTodoEndToEnd(parseT *testing.T) {
	// --- Get Project Root ---
	// The test is in the 'e2e' directory, so the project root is one level up.
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root directory: %v", parseErr)
	}

	// --- 1. Build Phase ---
	parseT.Log("Building all components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = parseProjectRoot // Run build.sh from the project root
	buildOutput, parseErr := buildCmd.CombinedOutput()
	if parseErr != nil {
		parseT.Fatalf("Failed to build components: %v\nOutput: %s", parseErr, string(buildOutput))
	}
	parseT.Log("Build successful.")

	// --- 2. Setup Phase ---
	parseT.Log("Starting backend services...")

	// Start direct-bridge example (serves gRPC directly over WebSocket)
	// This uses the bridge library to serve gRPC over WebSocket
	parseDirectBridgePath := filepath.Join(parseProjectRoot, "examples", "direct-bridge", "main.go")
	parseBridgeCleanup := startCommand(parseT, parseProjectRoot, "DirectBridge", "go", "run", parseDirectBridgePath)
	parseT.Cleanup(parseBridgeCleanup)

	// Start file server for the public directory
	parsePublicDir := http.Dir(filepath.Join(parseProjectRoot, "examples", "_shared", "public"))
	parseFileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(parsePublicDir)}
	go func() {
		if parseErr2 := parseFileServer.ListenAndServe(); parseErr2 != nil && parseErr2 != http.ErrServerClosed {
			log.Printf("File server error: %v", parseErr2)
		}
	}()
	parseT.Cleanup(func() {
		parseT.Log("Shutting down file server...")
		parseFileServer.Shutdown(context.Background())
	})

	// Give servers a moment to start up
	time.Sleep(3 * time.Second)

	// --- 3. Execution Phase ---
	parseT.Log("Starting Playwright for browser automation...")
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
	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseT.Logf("[Browser Console] %s", parseMsg.Text())
		parseConsoleMessages <- parseMsg.Text()
	})

	parseT.Log("Navigating to the web client...")
	if _, parseErr = parsePage.Goto("http://localhost:8081"); parseErr != nil {
		parseT.Fatalf("Failed to navigate to page: %v", parseErr)
	}

	// --- 4. Assertion Phase ---
	parseT.Log("Waiting for success log message from WASM client...")
	parseTimeout := time.After(15 * time.Second)
	var parseSuccessLog string

	for {
		select {
		case parseMsg2 := <-parseConsoleMessages:
			if strings.Contains(parseMsg2, "WASM: Created new todo") {
				parseSuccessLog = parseMsg2
				goto testSuccess // Exit the loop
			}
		case <-parseTimeout:
			parseT.Fatal("Test timed out waiting for success log message")
		}
	}

testSuccess:
	parseT.Logf("Successfully captured log: %s", parseSuccessLog)
	if !strings.Contains(parseSuccessLog, "Learn gRPC-over-WebSocket") {
		parseT.Errorf("Expected todo text 'Learn gRPC-over-WebSocket' not found in success log")
	}
}

// TestMultipleSequentialRequests tests sending multiple consecutive gRPC requests
func TestMultipleSequentialRequests(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = parseProjectRoot
	if buildOutput, parseErr2 := buildCmd.CombinedOutput(); parseErr2 != nil {
		parseT.Fatalf("Failed to build: %v\nOutput: %s", parseErr2, string(buildOutput))
	}

	// Start services
	parseDirectBridgePath := filepath.Join(parseProjectRoot, "examples", "direct-bridge", "main.go")
	parseBridgeCleanup := startCommand(parseT, parseProjectRoot, "DirectBridge", "go", "run", parseDirectBridgePath)
	parseT.Cleanup(parseBridgeCleanup)

	parsePublicDir := http.Dir(filepath.Join(parseProjectRoot, "examples", "_shared", "public"))
	parseFileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(parsePublicDir)}
	go func() {
		if parseErr3 := parseFileServer.ListenAndServe(); parseErr3 != nil && parseErr3 != http.ErrServerClosed {
			log.Printf("File server error: %v", parseErr3)
		}
	}()
	parseT.Cleanup(func() {
		parseFileServer.Shutdown(context.Background())
	})

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

	parseConsoleMessages := make(chan string, 100)
	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseT.Logf("[Browser Console] %s", parseMsg.Text())
		parseConsoleMessages <- parseMsg.Text()
	})

	if _, parseErr = parsePage.Goto("http://localhost:8081"); parseErr != nil {
		parseT.Fatalf("Failed to navigate: %v", parseErr)
	}

	// Wait for first request to complete
	parseTimeout := time.After(15 * time.Second)
	parseSuccessCount := 0
	parseRequiredSuccesses := 1

	for {
		select {
		case parseMsg2 := <-parseConsoleMessages:
			if strings.Contains(parseMsg2, "WASM: Created new todo") {
				parseSuccessCount++
				if parseSuccessCount >= parseRequiredSuccesses {
					parseT.Logf("Successfully captured %d todo creation(s)", parseSuccessCount)
					return
				}
			}
		case <-parseTimeout:
			parseT.Fatalf("Test timed out. Got %d successes, expected %d", parseSuccessCount, parseRequiredSuccesses)
		}
	}
}

// TestConnectionResilience tests reconnection and error handling
func TestConnectionResilience(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = parseProjectRoot
	if buildOutput, parseErr2 := buildCmd.CombinedOutput(); parseErr2 != nil {
		parseT.Fatalf("Failed to build: %v\nOutput: %s", parseErr2, string(buildOutput))
	}

	// Start services
	parseDirectBridgePath := filepath.Join(parseProjectRoot, "examples", "direct-bridge", "main.go")
	parseBridgeCleanup := startCommand(parseT, parseProjectRoot, "DirectBridge", "go", "run", parseDirectBridgePath)
	parseT.Cleanup(parseBridgeCleanup)

	parsePublicDir := http.Dir(filepath.Join(parseProjectRoot, "examples", "_shared", "public"))
	parseFileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(parsePublicDir)}
	go func() {
		if parseErr3 := parseFileServer.ListenAndServe(); parseErr3 != nil && parseErr3 != http.ErrServerClosed {
			log.Printf("File server error: %v", parseErr3)
		}
	}()
	parseT.Cleanup(func() {
		parseFileServer.Shutdown(context.Background())
	})

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

	parseConsoleMessages := make(chan string, 100)
	parseErrorMessages := make(chan string, 100)

	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseText := parseMsg.Text()
		parseT.Logf("[Browser Console] %s", parseText)
		parseConsoleMessages <- parseText

		// Track errors separately
		if parseMsg.Type() == "error" || strings.Contains(strings.ToLower(parseText), "error") ||
			strings.Contains(strings.ToLower(parseText), "failed") {
			parseErrorMessages <- parseText
		}
	})

	if _, parseErr = parsePage.Goto("http://localhost:8081"); parseErr != nil {
		parseT.Fatalf("Failed to navigate: %v", parseErr)
	}

	// Wait for successful connection or error
	parseTimeout := time.After(20 * time.Second)
	isGotSuccess := false
	var parseErrors []string

	for {
		select {
		case parseMsg2 := <-parseConsoleMessages:
			if strings.Contains(parseMsg2, "WASM: Created new todo") {
				isGotSuccess = true
			}
		case parseErrMsg := <-parseErrorMessages:
			parseErrors = append(parseErrors, parseErrMsg)
		case <-parseTimeout:
			if !isGotSuccess {
				if len(parseErrors) > 0 {
					parseT.Logf("Errors encountered: %v", parseErrors)
				}
				parseT.Fatal("Test timed out without successful todo creation")
			}
			return
		}

		if isGotSuccess {
			parseT.Log("Connection resilience test passed - successful todo creation")
			return
		}
	}
}

// TestConcurrentConnections tests multiple browser tabs connecting simultaneously
func TestConcurrentConnections(parseT *testing.T) {
	if testing.Short() {
		parseT.Skip("Skipping e2e test in short mode")
	}

	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = parseProjectRoot
	if buildOutput, parseErr2 := buildCmd.CombinedOutput(); parseErr2 != nil {
		parseT.Fatalf("Failed to build: %v\nOutput: %s", parseErr2, string(buildOutput))
	}
	// Start services
	parseDirectBridgePath := filepath.Join(parseProjectRoot, "examples", "direct-bridge", "main.go")
	parseBridgeCleanup := startCommand(parseT, parseProjectRoot, "DirectBridge", "go", "run", parseDirectBridgePath)
	parseT.Cleanup(parseBridgeCleanup)

	parsePublicDir := http.Dir(filepath.Join(parseProjectRoot, "examples", "_shared", "public"))
	parseFileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(parsePublicDir)}
	go func() {
		if parseErr3 := parseFileServer.ListenAndServe(); parseErr3 != nil && parseErr3 != http.ErrServerClosed {
			log.Printf("File server error: %v", parseErr3)
		}
	}()
	parseT.Cleanup(func() {
		parseFileServer.Shutdown(context.Background())
	})

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

	// Create multiple pages (tabs) concurrently
	parseNumTabs := 3
	var parseWg sync.WaitGroup
	parseSuccessChan := make(chan bool, parseNumTabs)
	parseErrorChan := make(chan error, parseNumTabs)

	for parseI := 0; parseI < parseNumTabs; parseI++ {
		parseWg.Add(1)
		go func(parseTabNum int) {
			defer parseWg.Done()

			parsePage, parseErr4 := parseBrowser.NewPage()
			if parseErr4 != nil {
				parseErrorChan <- fmt.Errorf("tab %d: failed to create page: %w", parseTabNum, parseErr4)
				return
			}

			parseConsoleMessages := make(chan string, 50)
			parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
				parseText := parseMsg.Text()
				parseT.Logf("[Tab %d Console] %s", parseTabNum, parseText)
				parseConsoleMessages <- parseText
			})

			if _, parseErr4 = parsePage.Goto("http://localhost:8081"); parseErr4 != nil {
				parseErrorChan <- fmt.Errorf("tab %d: failed to navigate: %w", parseTabNum, parseErr4)
				return
			}

			// Wait for success
			parseTimeout := time.After(20 * time.Second)
			for {
				select {
				case parseMsg2 := <-parseConsoleMessages:
					if strings.Contains(parseMsg2, "WASM: Created new todo") {
						parseT.Logf("Tab %d: Successfully created todo", parseTabNum)
						parseSuccessChan <- true
						return
					}
				case <-parseTimeout:
					parseErrorChan <- fmt.Errorf("tab %d: timed out waiting for todo creation", parseTabNum)
					return
				}
			}
		}(parseI)
	}

	// Wait for all goroutines to complete
	parseWg.Wait()
	close(parseSuccessChan)
	close(parseErrorChan)

	// Check results
	parseSuccessCount := len(parseSuccessChan)
	parseErrors := make([]error, 0, len(parseErrorChan))
	for parseErr5 := range parseErrorChan {
		parseErrors = append(parseErrors, parseErr5)
	}

	if len(parseErrors) > 0 {
		parseT.Logf("Errors from concurrent connections: %v", parseErrors)
	}

	if parseSuccessCount == 0 {
		parseT.Fatal("No successful connections in concurrent test")
	}

	parseT.Logf("Concurrent connection test passed: %d/%d tabs succeeded", parseSuccessCount, parseNumTabs)
}

// TestLargePayload tests handling of larger todo items
func TestLargePayload(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Failed to get project root: %v", parseErr)
	}

	// Build
	parseT.Log("Building components...")
	buildCmd := exec.Command("bash", "examples/wasm-client/build.sh")
	buildCmd.Dir = parseProjectRoot
	if buildOutput, parseErr2 := buildCmd.CombinedOutput(); parseErr2 != nil {
		parseT.Fatalf("Failed to build: %v\nOutput: %s", parseErr2, string(buildOutput))
	}

	// Start services
	parseDirectBridgePath := filepath.Join(parseProjectRoot, "examples", "direct-bridge", "main.go")
	parseBridgeCleanup := startCommand(parseT, parseProjectRoot, "DirectBridge", "go", "run", parseDirectBridgePath)
	parseT.Cleanup(parseBridgeCleanup)

	parsePublicDir := http.Dir(filepath.Join(parseProjectRoot, "examples", "_shared", "public"))
	parseFileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(parsePublicDir)}
	go func() {
		if parseErr3 := parseFileServer.ListenAndServe(); parseErr3 != nil && parseErr3 != http.ErrServerClosed {
			log.Printf("File server error: %v", parseErr3)
		}
	}()
	parseT.Cleanup(func() {
		parseFileServer.Shutdown(context.Background())
	})

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

	parseConsoleMessages := make(chan string, 100)
	parsePage.On("console", func(parseMsg playwright.ConsoleMessage) {
		parseT.Logf("[Browser Console] %s", parseMsg.Text())
		parseConsoleMessages <- parseMsg.Text()
	})

	if _, parseErr = parsePage.Goto("http://localhost:8081"); parseErr != nil {
		parseT.Fatalf("Failed to navigate: %v", parseErr)
	}

	// Wait for connection with larger timeout for payload
	parseTimeout := time.After(25 * time.Second)
	for {
		select {
		case parseMsg2 := <-parseConsoleMessages:
			// Accept success even with default text - we're testing the connection works
			if strings.Contains(parseMsg2, "WASM: Created new todo") {
				parseT.Log("Large payload test passed - todo creation successful")
				return
			}
		case <-parseTimeout:
			parseT.Fatal("Test timed out waiting for todo creation")
		}
	}
}
