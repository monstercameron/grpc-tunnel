package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	parseInstallPlaywrightOnce   sync.Once
	parseInstallPlaywrightErr    error
	parseInstallPlaywrightOutput string
)

// buildWASMClient builds the wasm client assets used by e2e tests.
func buildWASMClient(parseT *testing.T, parseProjectRoot string) {
	parseT.Helper()
	installPlaywrightBrowsers(parseT, parseProjectRoot)

	parseT.Log("Building WASM client example...")
	parseWasmClientDir := filepath.Join(parseProjectRoot, "examples", "wasm-client")
	parseWasmOutPath := filepath.Join(parseProjectRoot, "examples", "_shared", "public", "client.wasm")
	parseBuildCmd := exec.Command("go", "build", "-o", parseWasmOutPath)
	parseBuildCmd.Dir = parseWasmClientDir
	parseBuildCmd.Env = append(os.Environ(), "GO111MODULE=on", "GOOS=js", "GOARCH=wasm")
	if parseOutput, parseErr := parseBuildCmd.CombinedOutput(); parseErr != nil {
		parseT.Fatalf("Failed to build wasm client: %v\nOutput: %s", parseErr, string(parseOutput))
	}

	parseT.Log("Copying wasm_exec.js...")
	parseWasmExecDest := filepath.Join(parseProjectRoot, "examples", "_shared", "public", "wasm_exec.js")
	if _, parseErr := os.Stat(parseWasmExecDest); os.IsNotExist(parseErr) {
		parseGoRoot := getGoRoot(parseT, parseProjectRoot)
		parseWasmExecSource := findWasmExecPath(parseT, parseGoRoot)
		copyBuildFile(parseT, parseWasmExecSource, parseWasmExecDest)
	}
}

// installPlaywrightBrowsers installs Playwright browser binaries once per test process.
func installPlaywrightBrowsers(parseT *testing.T, parseProjectRoot string) {
	parseT.Helper()

	parseInstallPlaywrightOnce.Do(func() {
		parseInstallCmd := exec.Command("go", "run", "github.com/playwright-community/playwright-go/cmd/playwright", "install")
		parseInstallCmd.Dir = parseProjectRoot
		parseOutput, parseErr := parseInstallCmd.CombinedOutput()
		parseInstallPlaywrightOutput = string(parseOutput)
		if parseErr != nil {
			parseInstallPlaywrightErr = fmt.Errorf("failed to install Playwright browsers: %w", parseErr)
		}
	})

	if parseInstallPlaywrightErr != nil {
		parseT.Fatalf("%v\nOutput: %s", parseInstallPlaywrightErr, parseInstallPlaywrightOutput)
	}
}

// getGoRoot returns the Go installation root used for test builds.
func getGoRoot(parseT *testing.T, parseProjectRoot string) string {
	parseT.Helper()

	parseGoRootCmd := exec.Command("go", "env", "GOROOT")
	parseGoRootCmd.Dir = parseProjectRoot
	parseOutput, parseErr := parseGoRootCmd.CombinedOutput()
	if parseErr != nil {
		parseT.Fatalf("Failed to resolve GOROOT: %v\nOutput: %s", parseErr, string(parseOutput))
	}

	return strings.TrimSpace(string(parseOutput))
}

// findWasmExecPath returns the path to wasm_exec.js in the current Go toolchain.
func findWasmExecPath(parseT *testing.T, parseGoRoot string) string {
	parseT.Helper()

	parseCandidates := []string{
		filepath.Join(parseGoRoot, "lib", "wasm", "wasm_exec.js"),
		filepath.Join(parseGoRoot, "misc", "wasm", "wasm_exec.js"),
	}
	for _, parseCandidate := range parseCandidates {
		if _, parseErr := os.Stat(parseCandidate); parseErr == nil {
			return parseCandidate
		}
	}

	parseT.Fatalf("Failed to locate wasm_exec.js under GOROOT=%s", parseGoRoot)
	return ""
}

// copyBuildFile copies one file to another path, creating parent directories if needed.
func copyBuildFile(parseT *testing.T, parseSourcePath, parseDestPath string) {
	parseT.Helper()

	parseData, parseErr := os.ReadFile(parseSourcePath)
	if parseErr != nil {
		parseT.Fatalf("Failed to read source file %s: %v", parseSourcePath, parseErr)
	}

	if parseErr = os.MkdirAll(filepath.Dir(parseDestPath), 0750); parseErr != nil {
		parseT.Fatalf("Failed to create destination directory for %s: %v", parseDestPath, parseErr)
	}

	if parseErr = os.WriteFile(parseDestPath, parseData, 0644); parseErr != nil {
		parseT.Fatalf("Failed to copy %s to %s: %v", parseSourcePath, parseDestPath, parseErr)
	}
}
