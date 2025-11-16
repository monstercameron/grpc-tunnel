# Development Guide

## Quick Start

```bash
# Clone the repo
git clone https://github.com/monstercameron/grpc-tunnel.git
cd grpc-tunnel

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

# E2E tests (requires Playwright)
make e2e

# Run specific test
go test ./pkg/bridge -v -run TestWebSocketConn
```

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
- Weak crypto algorithms
- File path traversal
- Unhandled errors that could cause security issues

Results appear in **GitHub → Security → Code scanning alerts**.

**Note:** Security scan is informational only - it won't block builds or commits.

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
