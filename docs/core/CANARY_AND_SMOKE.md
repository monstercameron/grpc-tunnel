# Canary and Smoke Verification

This document defines canary rollout and smoke verification for production GoGRPCBridge deploys.

## Canary Rollout Policy

Recommended rollout stages:

1. Stage 1: `5%` traffic for 10-15 minutes.
2. Stage 2: `25%` traffic for 15-30 minutes.
3. Stage 3: `100%` traffic after successful checks.

Advance only if each stage meets SLO/error-budget guardrails.

## Smoke Verification Checklist

At each rollout stage:

- [ ] websocket upgrade path responds for valid origin requests
- [ ] invalid origin requests are rejected as expected
- [ ] unary RPC over tunnel succeeds
- [ ] streaming RPC over tunnel succeeds
- [ ] no abnormal spike in upgrade failures or RPC transport errors
- [ ] latency remains inside policy thresholds

## Abort Conditions

Abort canary and rollback if any occur:

- sustained increase in upgrade failure rate
- sustained increase in streaming transport failures
- readiness check instability
- severe customer-impacting incident reports tied to rollout

## Post-Canary Decision

If all checks pass:

- proceed to next canary stage or full rollout

If checks fail:

- execute rollback via `ROLLBACK_AND_HOTFIX.md`
- open incident follow-up with captured diagnostics

## Evidence Capture

Record for release notes and incident history:

- stage timestamps and traffic percentages
- smoke-check outcomes
- key metric snapshots
- final decision (`advance` or `rollback`)
