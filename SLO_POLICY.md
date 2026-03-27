# SLO Policy

This policy defines service-level objectives for GoGRPCBridge-operated tunnel services.

## Scope

Applies to production bridge deployments serving browser-to-gRPC tunnel traffic.

## SLI Definitions

1. Tunnel Availability SLI
   - numerator: successful websocket upgrade requests
   - denominator: total websocket upgrade requests

2. Streaming Reliability SLI
   - numerator: streaming RPC sessions completed without transport-level failure
   - denominator: total streaming RPC sessions started

3. Bridge Request Latency SLI (supporting)
   - p95 bridge request latency for unary tunnel traffic

## SLO Targets

- Tunnel availability: `99.9%` per rolling 30-day window
- Streaming reliability: `99.5%` per rolling 30-day window
- Unary tunnel latency p95: `< 250ms` per rolling 7-day window

## Error Budget Policy

- Tunnel availability error budget: `0.1%` over 30 days
- Streaming reliability error budget: `0.5%` over 30 days

When error budget burn exceeds policy thresholds:

1. Freeze non-critical feature rollout.
2. Prioritize reliability fixes and incident actions.
3. Require reliability sign-off before next feature release.

## Measurement and Reporting

- SLI data source must align with `OBSERVABILITY_CONTRACT.md` metrics.
- SLO dashboards should display:
  - current SLI values
  - rolling-window SLO status
  - error budget remaining and burn rate
- Release notes should include any SLO-impacting incidents in period.
