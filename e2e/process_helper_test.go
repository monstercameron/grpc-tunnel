package e2e

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	parseBuildBridgeBinaryMu     sync.Mutex
	parseBuildBridgeBinaryPath   string
	parseBuildBridgeBinaryOutput string
)

// buildBridgeBinary builds the direct-bridge server binary used by e2e tests.
func buildBridgeBinary(parseT *testing.T, parseProjectRoot string) string {
	parseT.Helper()

	parseBuildBridgeBinaryMu.Lock()
	defer parseBuildBridgeBinaryMu.Unlock()

	parseBinaryName := "direct-bridge-e2e"
	if runtime.GOOS == "windows" {
		parseBinaryName += ".exe"
	}

	parseBinaryDir := filepath.Join(parseProjectRoot, "bin")
	parseBuildBridgeBinaryPath = filepath.Join(parseBinaryDir, parseBinaryName)
	if parseErr := os.MkdirAll(parseBinaryDir, 0750); parseErr != nil {
		parseT.Fatalf("failed to create bridge binary directory: %v", parseErr)
	}

	if _, parseErr := os.Stat(parseBuildBridgeBinaryPath); parseErr == nil {
		return parseBuildBridgeBinaryPath
	} else if !os.IsNotExist(parseErr) {
		parseT.Fatalf("failed to stat bridge binary %s: %v", parseBuildBridgeBinaryPath, parseErr)
	}

	parseBuildCmd := exec.Command("go", "build", "-o", parseBuildBridgeBinaryPath, "./examples/direct-bridge")
	parseBuildCmd.Dir = parseProjectRoot
	parseOutput, parseErr := parseBuildCmd.CombinedOutput()
	parseBuildBridgeBinaryOutput = string(parseOutput)
	if parseErr != nil {
		parseT.Fatalf("failed to build direct-bridge binary: %v\nOutput: %s", parseErr, parseBuildBridgeBinaryOutput)
	}

	if _, parseErr = os.Stat(parseBuildBridgeBinaryPath); parseErr != nil {
		parseT.Fatalf("bridge binary was not created at %s: %v\nOutput: %s", parseBuildBridgeBinaryPath, parseErr, parseBuildBridgeBinaryOutput)
	}

	return parseBuildBridgeBinaryPath
}

// startBridgeCommand starts the direct-bridge server with stale process cleanup.
func startBridgeCommand(parseT *testing.T, parseProjectRoot, parseName string) func() {
	parseT.Helper()

	const parseBridgeAddress = "127.0.0.1:5000"
	const parseBridgeStartAttempts = 2

	parseBridgeBinaryPath := buildBridgeBinary(parseT, parseProjectRoot)
	for parseAttempt := 1; parseAttempt <= parseBridgeStartAttempts; parseAttempt++ {
		// E2E failures can leave orphaned bridge processes; clear those first so each
		// test starts from a known-clean network/process state.
		clearBridgeStaleProcesses(parseT)
		parsePortFreeErr := waitBridgePortFree(parseBridgeAddress, 6*time.Second)
		if parsePortFreeErr != nil {
			parseT.Logf("Bridge startup attempt %d/%d could not free port: %v", parseAttempt, parseBridgeStartAttempts, parsePortFreeErr)
			continue
		}

		parseCleanup := startCommand(parseT, parseProjectRoot, parseName, parseBridgeBinaryPath)
		parseReadyErr := waitBridgeReady(parseBridgeAddress, 6*time.Second)
		if parseReadyErr != nil {
			parseT.Logf("Bridge startup attempt %d/%d was not ready: %v", parseAttempt, parseBridgeStartAttempts, parseReadyErr)
			parseCleanup()
			continue
		}

		parseStableErr := waitBridgeStable(parseBridgeAddress, 1200*time.Millisecond)
		if parseStableErr != nil {
			parseT.Logf("Bridge startup attempt %d/%d was unstable: %v", parseAttempt, parseBridgeStartAttempts, parseStableErr)
			parseCleanup()
			continue
		}

		return parseCleanup
	}

	parseT.Fatalf("failed to start stable bridge process after %d attempts", parseBridgeStartAttempts)
	return func() {}
}

// waitBridgeReady waits until the bridge listener accepts TCP connections.
func waitBridgeReady(parseAddress string, parseTimeout time.Duration) error {
	parseDeadline := time.Now().Add(parseTimeout)
	for time.Now().Before(parseDeadline) {
		parseConn, parseErr := net.DialTimeout("tcp", parseAddress, 250*time.Millisecond)
		if parseErr == nil {
			_ = parseConn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("bridge %s did not become ready within %s", parseAddress, parseTimeout)
}

// waitBridgeStable verifies the bridge remains reachable during a short stability window.
func waitBridgeStable(parseAddress string, parseWindow time.Duration) error {
	parseDeadline := time.Now().Add(parseWindow)
	for time.Now().Before(parseDeadline) {
		parseConn, parseErr := net.DialTimeout("tcp", parseAddress, 250*time.Millisecond)
		if parseErr != nil {
			return fmt.Errorf("bridge %s became unreachable: %w", parseAddress, parseErr)
		}
		_ = parseConn.Close()
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// waitBridgePortFree waits until the bridge address can be bound.
func waitBridgePortFree(parseAddress string, parseTimeout time.Duration) error {
	parseDeadline := time.Now().Add(parseTimeout)
	for time.Now().Before(parseDeadline) {
		parseListener, parseErr := net.Listen("tcp", parseAddress)
		if parseErr == nil {
			_ = parseListener.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("bridge %s remained in use for %s", parseAddress, parseTimeout)
}

// buildProcessTreeKillCommands returns the commands used to terminate a process tree.
func buildProcessTreeKillCommands(parsePID int) []*exec.Cmd {
	if parsePID <= 0 {
		return nil
	}

	if runtime.GOOS == "windows" {
		return []*exec.Cmd{
			exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(parsePID)),
		}
	}

	return []*exec.Cmd{
		exec.Command("pkill", "-TERM", "-P", strconv.Itoa(parsePID)),
		exec.Command("kill", "-9", strconv.Itoa(parsePID)),
	}
}

// killProcessTreeByPID terminates a process and its descendants.
func killProcessTreeByPID(parseT *testing.T, parsePID int) {
	parseT.Helper()

	for _, parseKillCmd := range buildProcessTreeKillCommands(parsePID) {
		if runtime.GOOS == "windows" {
			// taskkill /T ensures the entire child tree is terminated, not just the
			// parent process handle tracked by the test.
			if parseOutput, parseErr := parseKillCmd.CombinedOutput(); parseErr != nil {
				parseOutputText := strings.TrimSpace(string(parseOutput))
				if parseOutputText != "" {
					parseT.Logf("taskkill failed for pid %d: %v (%s)", parsePID, parseErr, parseOutputText)
				}
			}
			continue
		}

		_ = parseKillCmd.Run()
	}
}

// buildBridgeClearCommands returns the commands used to remove stale bridge processes.
func buildBridgeClearCommands() []*exec.Cmd {
	if runtime.GOOS == "windows" {
		return []*exec.Cmd{
			exec.Command("taskkill", "/F", "/T", "/IM", "direct-bridge-e2e.exe"),
			exec.Command("taskkill", "/F", "/T", "/IM", "direct-bridge.exe"),
			exec.Command("powershell", "-NoProfile", "-Command",
				"Get-CimInstance Win32_Process -Filter \"Name='go.exe'\" | "+
					"Where-Object { $_.CommandLine -like '*examples*direct-bridge*main.go*' } | "+
					"ForEach-Object { taskkill /F /T /PID $_.ProcessId | Out-Null }"),
			exec.Command("powershell", "-NoProfile", "-Command",
				"(Get-NetTCPConnection -State Listen -LocalPort 5000 -ErrorAction SilentlyContinue | "+
					"Select-Object -Expand OwningProcess -Unique) | "+
					"ForEach-Object { taskkill /F /T /PID $_ | Out-Null }"),
		}
	}

	return []*exec.Cmd{
		exec.Command("pkill", "-9", "-f", "direct-bridge-e2e"),
		exec.Command("pkill", "-9", "-f", "direct-bridge"),
		exec.Command("sh", "-c", "lsof -ti:5000 | xargs kill -9 2>/dev/null"),
	}
}

// clearBridgeStaleProcesses removes orphaned direct-bridge processes from prior test runs.
func clearBridgeStaleProcesses(parseT *testing.T) {
	parseT.Helper()

	for _, parseClearCmd := range buildBridgeClearCommands() {
		_ = parseClearCmd.Run()
	}

	if parseErr := waitBridgePortFree("127.0.0.1:5000", 6*time.Second); parseErr != nil {
		parseT.Logf("bridge port cleanup warning: %v", parseErr)
	}
}
