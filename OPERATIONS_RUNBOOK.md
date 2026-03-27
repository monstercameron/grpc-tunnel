# Operations Runbook

This runbook covers deployment, rollback, and incident triage for GoGRPCBridge-operated services.

## Deployment Runbook

1. Confirm release checks are complete:
   - `RELEASE_CHECKLIST.md`
   - `SECURITY_RELEASE_CHECKLIST.md`
   - `RELEASE_PERFORMANCE_TEMPLATE.md`
2. Deploy release artifact to target environment.
3. Validate startup logs and tunnel endpoint registration.
4. Run smoke checks against bridge and backend service.

## Smoke Verification

From deployment environment or trusted test client:

- Verify bridge endpoint responds to WebSocket upgrade path.
- Verify one unary RPC request over the tunnel.
- Verify one streaming RPC request over the tunnel.
- Verify origin policy behavior for both allowed and rejected origins.

## Rollback Entry Point

If release impact is critical, follow:

- `ROLLBACK_AND_HOTFIX.md`

Rollback should restore the last known good release tag and rerun smoke checks.

## Incident Triage Flow

1. Classify severity (`critical`, `high`, `medium`, `low`).
2. Identify blast radius:
   - affected endpoints
   - affected tenants/users
   - affected regions/environments
3. Capture immediate diagnostics:
   - bridge logs around connect/disconnect and upgrade failures
   - backend service errors and latency spikes
   - current deployment/tag identifiers
   - transition mapping from `TUNNEL_STATE_DIAGNOSTICS.md`
4. Decide mitigation path:
   - rollback
   - hotfix
   - temporary feature disablement
5. Communicate status update with next checkpoint timestamp.

## Required Incident Data

- release tag and commit SHA
- error signatures and first-seen timestamp
- impact statement with scope
- mitigation action and owner
- follow-up issue link

## Post-Incident Actions

- Add or update regression tests for root cause.
- Update threat model and troubleshooting docs as needed.
- Update this runbook if triage flow gaps were discovered.
