# Development Guide

## Quick Start

```bash
# Clone the repo
git clone https://github.com/monstercameron/GoGRPCBridge.git
cd GoGRPCBridge

# Install golangci-lint (if not already installed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run tests
make test

# Make changes, then run pre-commit check
make check
```

## Pre-Commit Workflow

The repository has **strict pre-commit hooks** that will **block commits** if:
- Code is not formatted with `gofmt`
- Code has linting errors from `golangci-lint`

### Commands

```bash
# Run all pre-commit checks (format + lint + tests)
make check

# Format code
make fmt

# Run linter
make lint

# Auto-fix lint issues (when possible)
make lint-fix

# Run tests
make test

# Quick tests (no race detector)
make test-short
```

## Testing

```bash
# Unit tests with race detector and coverage
make test

# Quick tests
make test-short

# Fuzz tests (1 minute each)
make fuzz

# Quick fuzz validation (5 seconds each) - same as CI
make fuzz-quick

# Run specific fuzz test
go test ./pkg/bridge -v -fuzz=FuzzWebSocketConnWrite -fuzztime=60s

# E2E tests (requires Playwright)
make e2e

# Run specific test
go test ./pkg/bridge -v -run TestWebSocketConn
```

### Important: Fuzz Test Usage

**❌ Don't use:** `go test ./pkg/bridge -fuzz=. -fuzztime=10s`

This will fail with: `testing: will not fuzz, -fuzz matches more than one fuzz test`

**✅ Instead use:**
- `make fuzz` - Runs each fuzzer individually for 1 minute
- `make fuzz-quick` - Quick validation (5s each, same as CI)
- Individual fuzzer: `go test ./pkg/bridge -fuzz=FuzzWebSocketConnWrite -fuzztime=60s`

The `-fuzz` flag must match exactly ONE fuzz function.

## CI/CD Pipeline

When you push to `main`, GitHub Actions runs:

1. **Lint** - `golangci-lint` with `.golangci.yml` config
2. **Test** - Unit tests with race detector (Go 1.24.x and 1.23.x)
3. **Edge Tests** - Edge case scenarios
4. **Fuzz Tests** - Each fuzzer runs for 5 seconds
5. **E2E Tests** - End-to-end browser tests with Playwright
6. **Security Scan** - Gosec scans for vulnerabilities (informational)

### Auto-Release

**Only if all tests pass**, a new version tag is automatically created:
- Increments patch version (v0.0.5 → v0.0.6)
- Creates GitHub release with binaries
- No release if tests fail

## Makefile Targets

```bash
make help          # Show all available commands
make test          # Run tests with coverage
make test-short    # Quick tests without race detector
make fmt           # Format all code
make lint          # Run linter
make lint-fix      # Auto-fix lint issues
make check         # Format + lint + tests (pre-commit)
make pre-commit    # Same as check
make fuzz          # Run fuzz tests (1 min each)
make e2e           # Run end-to-end tests
make build         # Build example binaries
make clean         # Clean caches and artifacts
```

## Common Issues

### "golangci-lint: command not found"

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Pre-commit hook not executable

```bash
chmod +x .git/hooks/pre-commit
# Or
make install-hooks
```

### Tests fail with "playwright not found"

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium
```

### Want to commit despite lint errors (not recommended)

```bash
git commit --no-verify -m "Your message"
```

## Security Scan

The `security` job runs [Gosec](https://github.com/securego/gosec) to detect:
- SQL injection vulnerabilities
- Hardcoded credentials
- Weak crypto algorithms (MD5, SHA1, etc.)
- File path traversal
- Unhandled errors that could cause security issues
- HTTP servers without timeouts
- Insecure file permissions

### Severity Levels

Gosec categorizes issues by **severity** (LOW, MEDIUM, HIGH) and **confidence** (LOW, MEDIUM, HIGH):

- **HIGH severity + HIGH confidence**: Critical security issues - **CI WILL FAIL** ❌
  - Examples: SQL injection, hardcoded secrets, weak crypto
- **MEDIUM severity**: Important but not critical - **CI passes but issues logged** ⚠️
  - Examples: Missing timeouts, file permissions
- **LOW severity**: Informational - **CI passes** ℹ️
  - Examples: Unsafe calls in generated code

### CI Behavior

The security scan will **fail the build** if it finds issues with:
- `severity=high` AND `confidence=high`

This prevents deploying code with serious security vulnerabilities.

**Note:** G103 (unsafe calls) is excluded as it appears in auto-generated protobuf code.

### What is SARIF?

**SARIF** (Static Analysis Results Interchange Format) is a standard JSON format for security scan results. It allows GitHub to:
- Display security issues in the "Security" tab
- Show warnings on pull requests
- Track security trends over time

**Note:** SARIF upload to GitHub Security tab requires **GitHub Advanced Security** (not available on free public repos). 

### How to View Results

1. **CI Workflow:** Results are uploaded as artifacts and shown in the workflow logs
2. **Local Scan:** Run `gosec ./...` to see all security issues
3. **Strict Check:** Run `gosec -severity=high -confidence=high -exclude=G103 ./...` (same as CI)
4. **CI Artifacts:** Download `gosec-results` artifact from workflow runs (contains SARIF file)

### Current Security Issues

As of now, there are only **4 low-severity issues** (all in auto-generated protobuf code):
- **Generated Code:** Unsafe calls in protobuf files (G103) - can't fix, excluded from CI

**Production code** (`pkg/`) has **zero security issues**. ✅

## Code Style

- Use `gofmt` with simplify flag (`-s`)
- Follow standard Go conventions
- Add tests for new features
- Update README for API changes
- Keep test coverage above 80%

## Pull Request Checklist

- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] All tests pass (`make test`)
- [ ] Added tests for new features
- [ ] Updated README if needed
- [ ] No security issues from Gosec
