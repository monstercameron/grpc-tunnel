# Tunnel State Diagnostics

This document defines operational diagnostics for client and server tunnel state transitions.

## Server Tunnel States

- `upgrade_received`
- `upgrade_accepted`
- `upgrade_rejected`
- `tunnel_connected`
- `tunnel_disconnected`
- `backend_dial_failed`
- `stream_error`

## Client Tunnel States

- `dial_started`
- `dial_succeeded`
- `dial_failed`
- `reconnect_scheduled`
- `reconnect_attempt`
- `reconnect_succeeded`
- `reconnect_failed`
- `connection_closed`

## Required Diagnostic Fields

Every transition event should include:

- `timestamp`
- `component` (`client` or `server`)
- `state`
- `target` or endpoint path
- `request_id` / correlation id when available
- `error_class` and `error_message` for failures
- `retry_delay_ms` for reconnect events

## Failure Mapping

Common state-pattern diagnostics:

- `upgrade_rejected` spikes:
  - likely origin policy mismatch
  - verify `CheckOrigin` and incoming `Origin` headers
- repeated `dial_failed` then `reconnect_scheduled`:
  - backend or network reachability issue
  - verify DNS, routing, and endpoint health
- `dial_succeeded` with immediate `connection_closed`:
  - transport handshake accepted but RPC path unstable
  - inspect backend service logs and timeout policy

## Triage Usage

During incidents:

1. Build per-request transition timeline from logs.
2. Group failures by `error_class`.
3. Correlate transitions with deploy markers and canary stages.
4. Confirm whether failures are edge (`upgrade`) or backend-path (`stream_error`, `backend_dial_failed`).

## Integration Notes

- Align emitted events with `OBSERVABILITY_CONTRACT.md`.
- Use this mapping from `OPERATIONS_RUNBOOK.md` incident triage flow.
- Include transition anomalies in release/performance notes when regressions occur.
