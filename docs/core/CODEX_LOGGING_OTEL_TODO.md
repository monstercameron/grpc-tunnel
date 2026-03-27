# GoGRPCBridge Logging + OTel Todo (Codex CLI)

This list tracks logging surface review and OTel-compatible log hardening.

## Todo

- [x] Document logging surface area and required event coverage in observability docs.
- [x] Add structured, OTel-trace-aware logging for `pkg/bridge` lifecycle and error events.
- [x] Add structured, OTel-trace-aware logging for `pkg/grpctunnel` bridge/tooling lifecycle and error events.

## Checkpoints

### 2026-03-26 21:52 -04:00

- completed todo: Document logging surface area and required event coverage in observability docs.
- files changed: `OBSERVABILITY_CONTRACT.md`, `CODEX_LOGGING_OTEL_TODO.md`
- validation run: documentation-only change (format + content review)
- result: Added explicit runtime logging surface review, recommended event names, and OTel trace/span field requirements.
- residual risk: Documentation cannot enforce runtime emission by itself.
- next suggested todo: Add structured, OTel-trace-aware logging for `pkg/bridge` lifecycle and error events.

### 2026-03-26 21:54 -04:00

- completed todo: Add structured, OTel-trace-aware logging for `pkg/bridge` lifecycle and error events.
- files changed: `pkg/bridge/logging.go`, `pkg/bridge/bridge.go`, `pkg/bridge/bridge_test.go`, `pkg/bridge/logging_test.go`, `go.mod`, `go.sum`, `CODEX_LOGGING_OTEL_TODO.md`
- validation run: `go test ./pkg/bridge -count=1`
- result: Passed. Bridge logs now emit structured fields (`component`, `level`, `event`, request metadata) and include OTel `trace_id`/`span_id` when present in request context.
- residual risk: If callers provide custom non-structured logger implementations, downstream sink shape still depends on that logger behavior.
- next suggested todo: Add structured, OTel-trace-aware logging for `pkg/grpctunnel` bridge/tooling lifecycle and error events.

### 2026-03-26 21:57 -04:00

- completed todo: Add structured, OTel-trace-aware logging for `pkg/grpctunnel` bridge/tooling lifecycle and error events.
- files changed: `pkg/grpctunnel/logging.go`, `pkg/grpctunnel/server.go`, `pkg/grpctunnel/tooling.go`, `pkg/grpctunnel/logging_test.go`, `go.mod`, `go.sum`, `CODEX_LOGGING_OTEL_TODO.md`
- validation run: `go test ./pkg/grpctunnel -count=1`; `go test ./pkg/bridge -count=1`
- result: Passed. Canonical bridge/tooling paths now emit structured log events and include OTel trace/span identifiers when a request context carries span context.
- residual risk: Tooling logs that are not request-scoped cannot include trace/span IDs by design.
- next suggested todo: none.
