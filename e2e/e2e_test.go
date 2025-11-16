package e2e

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// startCommand is a helper function to start a command as a background process.
// It sets the working directory to the project root.
func startCommand(t *testing.T, projectRoot, name string, command string, args ...string) func() {
	t.Helper()

	cmd := exec.Command(command, args...)
	cmd.Dir = projectRoot // Run commands from the project root

	// Pipe stdout and stderr to the test log
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			t.Logf("[%s] %s", name, scanner.Text())
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.Logf("[%s|stderr] %s", name, scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start %s: %v", name, err)
	}

	cleanupFunc := func() {
		t.Logf("Cleaning up %s process...", name)
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("Failed to kill %s process: %v", name, err)
		}
		wg.Wait() // Wait for logging goroutines to finish
	}
	return cleanupFunc
}

func TestCreateTodoEndToEnd(t *testing.T) {
	// --- Get Project Root ---
	// The test is in the 'e2e' directory, so the project root is one level up.
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root directory: %v", err)
	}

	// --- 1. Build Phase ---
	t.Log("Building all components...")
	buildCmd := exec.Command("bash", "build.sh")
	buildCmd.Dir = projectRoot // Run build.sh from the project root
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build components: %v\nOutput: %s", err, string(buildOutput))
	}
	t.Log("Build successful.")

	// --- 2. Setup Phase ---
	t.Log("Starting backend services...")

	// Start direct-bridge example (serves gRPC directly over WebSocket)
	// This uses the bridge library to serve gRPC over WebSocket
	directBridgePath := filepath.Join(projectRoot, "examples", "direct-bridge", "main.go")
	bridgeCleanup := startCommand(t, projectRoot, "DirectBridge", "go", "run", directBridgePath)
	t.Cleanup(bridgeCleanup)

	// Start file server for the public directory
	publicDir := http.Dir(filepath.Join(projectRoot, "public"))
	fileServer := &http.Server{Addr: ":8081", Handler: http.FileServer(publicDir)}
	go func() {
		if err := fileServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("File server error: %v", err)
		}
	}()
	t.Cleanup(func() {
		t.Log("Shutting down file server...")
		fileServer.Shutdown(context.Background())
	})

	// Give servers a moment to start up
	time.Sleep(3 * time.Second)

	// --- 3. Execution Phase ---
	t.Log("Starting Playwright for browser automation...")
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
	page.On("console", func(msg playwright.ConsoleMessage) {
		t.Logf("[Browser Console] %s", msg.Text())
		consoleMessages <- msg.Text()
	})

	t.Log("Navigating to the web client...")
	if _, err = page.Goto("http://localhost:8081"); err != nil {
		t.Fatalf("Failed to navigate to page: %v", err)
	}

	// --- 4. Assertion Phase ---
	t.Log("Waiting for success log message from WASM client...")
	timeout := time.After(15 * time.Second)
	var successLog string

	for {
		select {
		case msg := <-consoleMessages:
			if strings.Contains(msg, "WASM: Created new todo") {
				successLog = msg
				goto testSuccess // Exit the loop
			}
		case <-timeout:
			t.Fatal("Test timed out waiting for success log message")
		}
	}

testSuccess:
	t.Logf("Successfully captured log: %s", successLog)
	if !strings.Contains(successLog, "Learn gRPC-over-WebSocket") {
		t.Errorf("Expected todo text 'Learn gRPC-over-WebSocket' not found in success log")
	}
}
