package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// resetBuildBridgeBinaryState resets the bridge build cache used by helper tests.
func resetBuildBridgeBinaryState() {
	parseBuildBridgeBinaryMu = sync.Mutex{}
	parseBuildBridgeBinaryPath = ""
	parseBuildBridgeBinaryOutput = ""
}

// TestFindWasmExecPath_PrefersLibPath verifies lib/wasm is preferred when both candidates exist.
func TestFindWasmExecPath_PrefersLibPath(parseT *testing.T) {
	parseGoRoot := parseT.TempDir()
	parseLibPath := filepath.Join(parseGoRoot, "lib", "wasm", "wasm_exec.js")
	parseMiscPath := filepath.Join(parseGoRoot, "misc", "wasm", "wasm_exec.js")

	if parseErr := os.MkdirAll(filepath.Dir(parseLibPath), 0750); parseErr != nil {
		parseT.Fatalf("MkdirAll() lib path error: %v", parseErr)
	}
	if parseErr := os.MkdirAll(filepath.Dir(parseMiscPath), 0750); parseErr != nil {
		parseT.Fatalf("MkdirAll() misc path error: %v", parseErr)
	}
	if parseErr := os.WriteFile(parseLibPath, []byte("lib"), 0644); parseErr != nil {
		parseT.Fatalf("WriteFile() lib path error: %v", parseErr)
	}
	if parseErr := os.WriteFile(parseMiscPath, []byte("misc"), 0644); parseErr != nil {
		parseT.Fatalf("WriteFile() misc path error: %v", parseErr)
	}

	parseResolvedPath := findWasmExecPath(parseT, parseGoRoot)
	if parseResolvedPath != parseLibPath {
		parseT.Fatalf("findWasmExecPath() = %s, want %s", parseResolvedPath, parseLibPath)
	}
}

// TestFindWasmExecPath_FallsBackToMiscPath verifies misc/wasm is used when lib/wasm is absent.
func TestFindWasmExecPath_FallsBackToMiscPath(parseT *testing.T) {
	parseGoRoot := parseT.TempDir()
	parseMiscPath := filepath.Join(parseGoRoot, "misc", "wasm", "wasm_exec.js")

	if parseErr := os.MkdirAll(filepath.Dir(parseMiscPath), 0750); parseErr != nil {
		parseT.Fatalf("MkdirAll() misc path error: %v", parseErr)
	}
	if parseErr := os.WriteFile(parseMiscPath, []byte("misc"), 0644); parseErr != nil {
		parseT.Fatalf("WriteFile() misc path error: %v", parseErr)
	}

	parseResolvedPath := findWasmExecPath(parseT, parseGoRoot)
	if parseResolvedPath != parseMiscPath {
		parseT.Fatalf("findWasmExecPath() = %s, want %s", parseResolvedPath, parseMiscPath)
	}
}

// TestCopyBuildFile_CreatesParentDirectory verifies file copies create missing directories.
func TestCopyBuildFile_CreatesParentDirectory(parseT *testing.T) {
	parseTempDir := parseT.TempDir()
	parseSourcePath := filepath.Join(parseTempDir, "source.txt")
	parseDestPath := filepath.Join(parseTempDir, "nested", "path", "dest.txt")

	if parseErr := os.WriteFile(parseSourcePath, []byte("copied-data"), 0644); parseErr != nil {
		parseT.Fatalf("WriteFile() source error: %v", parseErr)
	}

	copyBuildFile(parseT, parseSourcePath, parseDestPath)

	parseCopiedData, parseErr := os.ReadFile(parseDestPath)
	if parseErr != nil {
		parseT.Fatalf("ReadFile() destination error: %v", parseErr)
	}
	if string(parseCopiedData) != "copied-data" {
		parseT.Fatalf("copied contents = %q, want %q", string(parseCopiedData), "copied-data")
	}
}

// TestGetGoRoot_ReturnsExistingPath verifies the helper resolves a usable Go toolchain root.
func TestGetGoRoot_ReturnsExistingPath(parseT *testing.T) {
	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Abs(..) error: %v", parseErr)
	}

	parseGoRoot := getGoRoot(parseT, parseProjectRoot)
	if parseGoRoot == "" {
		parseT.Fatal("getGoRoot() returned an empty path")
	}
	if _, parseErr = os.Stat(parseGoRoot); parseErr != nil {
		parseT.Fatalf("Stat(%s) error: %v", parseGoRoot, parseErr)
	}
}

// TestBuildBridgeBinary_CreatesBinary verifies the cached bridge build helper produces an executable.
func TestBuildBridgeBinary_CreatesBinary(parseT *testing.T) {
	if testing.Short() {
		parseT.Skip("Skipping bridge binary build in short mode")
	}

	parseProjectRoot, parseErr := filepath.Abs("..")
	if parseErr != nil {
		parseT.Fatalf("Abs(..) error: %v", parseErr)
	}

	resetBuildBridgeBinaryState()
	parseT.Cleanup(resetBuildBridgeBinaryState)

	parseBinaryPath := buildBridgeBinary(parseT, parseProjectRoot)
	parseT.Cleanup(func() {
		_ = os.Remove(parseBinaryPath)
	})

	if parseBinaryPath == "" {
		parseT.Fatal("buildBridgeBinary() returned an empty path")
	}
	if _, parseErr = os.Stat(parseBinaryPath); parseErr != nil {
		parseT.Fatalf("Stat(%s) error: %v", parseBinaryPath, parseErr)
	}
}

// TestKillProcessTreeByPID_NonPositivePID verifies invalid PIDs are ignored.
func TestKillProcessTreeByPID_NonPositivePID(parseT *testing.T) {
	killProcessTreeByPID(parseT, 0)
	killProcessTreeByPID(parseT, -1)
}

// TestBuildProcessTreeKillCommands_CurrentOS verifies kill commands match the active platform strategy.
func TestBuildProcessTreeKillCommands_CurrentOS(parseT *testing.T) {
	parseCommands := buildProcessTreeKillCommands(4242)

	if runtime.GOOS == "windows" {
		if len(parseCommands) != 1 {
			parseT.Fatalf("buildProcessTreeKillCommands() count = %d, want 1", len(parseCommands))
		}
		if len(parseCommands[0].Args) != 5 {
			parseT.Fatalf("taskkill args = %v, want 5 args", parseCommands[0].Args)
		}
		if parseCommands[0].Args[0] != "taskkill" || parseCommands[0].Args[4] != "4242" {
			parseT.Fatalf("taskkill args = %v, want taskkill /F /T /PID 4242", parseCommands[0].Args)
		}
		return
	}

	if len(parseCommands) != 2 {
		parseT.Fatalf("buildProcessTreeKillCommands() count = %d, want 2", len(parseCommands))
	}
	if parseCommands[0].Args[0] != "pkill" || parseCommands[0].Args[3] != "4242" {
		parseT.Fatalf("first kill command = %v, want pkill -TERM -P 4242", parseCommands[0].Args)
	}
	if parseCommands[1].Args[0] != "kill" || parseCommands[1].Args[2] != "4242" {
		parseT.Fatalf("second kill command = %v, want kill -9 4242", parseCommands[1].Args)
	}
}

// TestBuildBridgeClearCommands_CurrentOS verifies stale bridge cleanup commands match the active platform.
func TestBuildBridgeClearCommands_CurrentOS(parseT *testing.T) {
	parseCommands := buildBridgeClearCommands()

	if runtime.GOOS == "windows" {
		if len(parseCommands) != 4 {
			parseT.Fatalf("buildBridgeClearCommands() count = %d, want 4", len(parseCommands))
		}
		if parseCommands[0].Args[0] != "taskkill" || parseCommands[0].Args[4] != "direct-bridge-e2e.exe" {
			parseT.Fatalf("first clear command = %v, want taskkill direct-bridge-e2e.exe", parseCommands[0].Args)
		}
		if parseCommands[1].Args[0] != "taskkill" || parseCommands[1].Args[4] != "direct-bridge.exe" {
			parseT.Fatalf("second clear command = %v, want taskkill direct-bridge.exe", parseCommands[1].Args)
		}
		if parseCommands[2].Args[0] != "powershell" || parseCommands[3].Args[0] != "powershell" {
			parseT.Fatalf("powershell clear commands = %v / %v, want powershell invocations", parseCommands[2].Args, parseCommands[3].Args)
		}
		return
	}

	if len(parseCommands) != 3 {
		parseT.Fatalf("buildBridgeClearCommands() count = %d, want 3", len(parseCommands))
	}
	if parseCommands[0].Args[0] != "pkill" || parseCommands[0].Args[3] != "direct-bridge-e2e" {
		parseT.Fatalf("first clear command = %v, want pkill direct-bridge-e2e", parseCommands[0].Args)
	}
	if parseCommands[1].Args[0] != "pkill" || parseCommands[1].Args[3] != "direct-bridge" {
		parseT.Fatalf("second clear command = %v, want pkill direct-bridge", parseCommands[1].Args)
	}
	if parseCommands[2].Args[0] != "sh" {
		parseT.Fatalf("third clear command = %v, want sh -c cleanup", parseCommands[2].Args)
	}
}
