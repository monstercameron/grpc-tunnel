# Dashboard Queries

This file provides baseline query wiring for the dashboard views defined in `OBSERVABILITY_CONTRACT.md`.

## Connection Health

- Active connections:
  - `sum(bridge_connections_active)`
- Accepted connections per minute:
  - `sum(rate(bridge_connections_total[1m]))`
- Upgrade failures per minute:
  - `sum(rate(bridge_upgrade_failures_total[1m]))`

## Upgrade Quality

- Upgrade latency p50:
  - `histogram_quantile(0.50, sum(rate(bridge_request_latency_ms_bucket[5m])) by (le))`
- Upgrade latency p95:
  - `histogram_quantile(0.95, sum(rate(bridge_request_latency_ms_bucket[5m])) by (le))`
- Upgrade failure ratio:
  - `sum(rate(bridge_upgrade_failures_total[5m])) / clamp_min(sum(rate(bridge_connections_total[5m])), 1)`

## Service Layer Extension

The bridge transport exports upgrade/session metrics and spans.
Service-layer middleware should add:

- `bridge_rpc_errors_total`
- `bridge_streams_active`
- business endpoint latency and error metrics
