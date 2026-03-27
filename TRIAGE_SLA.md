# Triage SLA and Severity Policy

This policy defines issue severity levels and response-time targets for GoGRPCBridge.

## Severity Levels

- `sev-1` critical
  - production outage, severe security risk, or broad functionality breakage
- `sev-2` high
  - major feature regression with significant user impact but partial workaround
- `sev-3` medium
  - non-critical bug with localized impact and acceptable workaround
- `sev-4` low
  - minor bug, polish issue, or low-impact edge case

## SLA Targets

Initial acknowledgment:

- `sev-1`: within 1 business day
- `sev-2`: within 2 business days
- `sev-3`: within 3 business days
- `sev-4`: within 5 business days

Triage classification (severity + scope labels):

- `sev-1`: within 1 business day
- `sev-2`: within 2 business days
- `sev-3`: within 4 business days
- `sev-4`: within 7 business days

## Mitigation Targets

- `sev-1`: immediate mitigation path (rollback/hotfix) started same day
- `sev-2`: mitigation plan and owner within 2 business days
- `sev-3`: mitigation plan in current or next planned release cycle
- `sev-4`: backlog prioritization by maintainer roadmap review

## Triage Workflow

1. Reproduce issue with minimal steps.
2. Assign severity and scope labels.
3. Link to incident/runbook path when operational.
4. Record mitigation owner and target release.
5. Update issue when status changes.

## Escalation

- `sev-1` and security-related issues escalate immediately to release and security owners.
- Repeated regressions in same area require prevention action items before next minor release.
