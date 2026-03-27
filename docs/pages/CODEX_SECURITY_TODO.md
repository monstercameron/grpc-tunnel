# GoGRPCBridge Security Todo (Codex CLI)

This list tracks targeted security hardening work from the March 26, 2026 review.

## Todo

- [x] Pin patched Go toolchain for stdlib CVE coverage and add release-facing guidance.
- [x] Add runtime warning guard when plaintext h2c backend bridging targets non-loopback hosts.
- [x] Add backend dial timeout guard in legacy helper proxy transport.
- [x] Add tooling bind-safety guard for reflection/pprof exposure on wildcard/public addresses.

## Checkpoints

### 2026-03-26 21:42 -04:00

- completed todo: Pin patched Go toolchain for stdlib CVE coverage and add release-facing guidance.
- files changed: `go.mod`, `SECURITY_RELEASE_CHECKLIST.md`, `CODEX_SECURITY_TODO.md`
- validation run: `go test ./pkg/grpctunnel -run TestBuildToolingHandler_Errors -count=1`
- result: Passed. Added `toolchain go1.25.8` to require a patched Go stdlib baseline and added a release checklist gate for minimum patch-level toolchain compliance.
- residual risk: Existing consumers may still build older tags with vulnerable toolchains until they update.
- next suggested todo: Add runtime warning guard when plaintext h2c backend bridging targets non-loopback hosts.

### 2026-03-26 21:43 -04:00

- completed todo: Add runtime warning guard when plaintext h2c backend bridging targets non-loopback hosts.
- files changed: `pkg/bridge/bridge.go`, `examples/_shared/helpers/proxy.go`, `CODEX_SECURITY_TODO.md`
- validation run: `go test ./pkg/bridge -run TestDefaultCheckOrigin -count=1`; `go test ./examples/_shared/helpers -run "TestServerConfig_CheckOrigin|TestProxyConfig_CheckOrigin|TestServerConfig_Defaults" -count=1`
- result: Passed. Both canonical and legacy bridge constructors now emit explicit security warnings when configured backend targets are non-loopback and therefore more likely to traverse untrusted plaintext network paths.
- residual risk: Warning-only behavior does not block insecure deployment when operators ignore logs.
- next suggested todo: Add backend dial timeout guard in legacy helper proxy transport.

### 2026-03-26 21:44 -04:00

- completed todo: Add backend dial timeout guard in legacy helper proxy transport.
- files changed: `examples/_shared/helpers/proxy.go`, `CODEX_SECURITY_TODO.md`
- validation run: `go test ./examples/_shared/helpers -count=1`
- result: Passed. Legacy helper proxy backend transport now uses a bounded dial timeout (`10s`) to prevent unbounded hangs during backend connectivity failures.
- residual risk: Timeout is fixed in helper compatibility code and not yet externally configurable.
- next suggested todo: Add tooling bind-safety guard for reflection/pprof exposure on wildcard/public addresses.

### 2026-03-26 21:45 -04:00

- completed todo: Add tooling bind-safety guard for reflection/pprof exposure on wildcard/public addresses.
- files changed: `pkg/grpctunnel/tooling.go`, `pkg/grpctunnel/additive_features_test.go`, `CODEX_SECURITY_TODO.md`
- validation run: `go test ./pkg/grpctunnel -run "TestListenAndServeTooling_InvalidAddress|TestListenAndServeTooling_NilServer|TestListenAndServeTooling_WildcardBindWithPprofReturnsError|TestGetToolingListenAddressError_WildcardAndLoopback" -count=1`
- result: Passed. Tooling listener now rejects wildcard binds when reflection/pprof are enabled and logs an explicit warning when introspection features are bound on non-loopback addresses.
- residual risk: Explicit non-loopback binds are still allowed (with warning) to support controlled internal tooling networks.
- next suggested todo: none.

### 2026-03-26 22:24 -04:00

- completed todo: Finish remaining security release checklist and docs alignment tasks.
- files changed: `SECURITY_RELEASE_CHECKLIST.md`, `SECURITY_FUZZ_PROCESS.md`, `DOCS_INDEX.md`, `README.md`, `THREAT_MODEL.md`, `TROUBLESHOOTING.md`, `MIGRATION.md`, `CHANGELOG.md`, `CODEX_SECURITY_TODO.md`
- validation run:
  - `go run ./tools/runner.go quality`
  - `go run github.com/securego/gosec/v2/cmd/gosec@latest -severity high -confidence high -exclude G103 ./...`
  - `go run ./tools/runner.go e2e`
- result: Passed. Security release checklist now has recorded gate evidence and sign-off metadata; migration/troubleshooting/threat docs now explicitly cover current origin/read-limit/tooling/reconnect hardening behavior; changelog includes security impact and residual-risk summary.
- residual risk: Tooling introspection endpoints remain intentionally usable on explicit non-loopback internal binds; operators must enforce network ACL/auth boundaries.
- next suggested todo: none.
