# Observability Contract

This document defines the minimum observability contract for GoGRPCBridge deployments.

## Logging Contract

Required structured log fields:

- `timestamp`
- `level`
- `component` (for example `bridge`, `backend`, `auth`)
- `event` (for example `ws_upgrade`, `tunnel_connect`, `tunnel_disconnect`, `rpc_error`)
- `request_id` or equivalent correlation id
- `remote_addr` (where available)
- `origin` (for browser requests where relevant)

Required event coverage:

- websocket upgrade success/failure
- connection lifecycle (connect/disconnect)
- backend dial and proxy failures
- authentication/authorization rejection (application layer)

### Logging Surface Review

Primary runtime logging surfaces:

1. `pkg/bridge` (compatibility proxy path)
   - configuration validation warnings
   - websocket upgrade success/failure
   - tunnel connect/disconnect lifecycle
   - backend proxy and dial-path failures
2. `pkg/grpctunnel` (canonical API path)
   - websocket upgrade success/failure
   - tunnel connect/disconnect lifecycle
   - bridge handler initialization failures
   - tooling exposure warnings and unsafe bind guards
3. application/service layer (outside transport package)
   - authn/authz rejection reasons
   - business RPC failures and domain audit trails

Recommended event names:

- `ws_upgrade_succeeded`
- `ws_upgrade_failed`
- `tunnel_connect`
- `tunnel_disconnect`
- `backend_proxy_error`
- `bridge_config_invalid`
- `bridge_request_rejected`
- `tooling_exposure`
- `tooling_bind_non_loopback`

OTel compatibility requirement:

- When request context carries OpenTelemetry span context, logs should include:
  - `trace_id`
  - `span_id`

## Metrics Contract

Minimum metric set:

- `bridge_connections_active` (gauge)
- `bridge_connections_total` (counter)
- `bridge_upgrade_failures_total` (counter)
- `bridge_rpc_errors_total` (counter)
- `bridge_streams_active` (gauge)
- `bridge_request_latency_ms` (histogram)
- `bridge_backend_dial_failures_total` (counter)

Current transport instrumentation status:

- `pkg/grpctunnel` emits OTel metrics for:
  - `bridge_connections_active`
  - `bridge_connections_total`
  - `bridge_upgrade_failures_total`
  - `bridge_request_latency_ms`
- `pkg/grpctunnel` starts server/session OTel spans:
  - `grpctunnel.bridge.request`
  - `grpctunnel.bridge.session`
- Remaining metrics in the minimum set should be emitted by service-layer RPC middleware and backend transport instrumentation.

Metric dimensions (labels/tags) should include:

- environment
- service
- endpoint/path
- status/result class

## Health Check Contract

Expose health checks that prove:

- process is alive
- bridge handler is serving
- backend dependency path is reachable (or explicit degraded status)

Recommended checks:

- liveness endpoint (`/healthz/live`)
- readiness endpoint (`/healthz/ready`)
- optional deep check (`/healthz/deep`) for backend dependency state

## Dashboard Contract

Required dashboard views:

1. Connection health:
   - active connections
   - connect/disconnect rate
   - upgrade failure rate
2. Request quality:
   - request latency percentiles
   - RPC error rate
   - backend dial failure rate
3. Incident view:
   - deploy markers overlaid on error/latency charts
   - top error signatures by count

Dashboard query templates:

- `observability/DASHBOARD_QUERIES.md`

## Alerting Baseline

Initial alert conditions should include:

- sustained upgrade failure rate spike
- sustained RPC error rate spike
- sudden active-connection collapse
- readiness probe failures

Alert routing should map critical signals to on-call ownership with runbook links.

Alert rule template:

- `observability/PROMETHEUS_ALERT_RULES.yaml`
