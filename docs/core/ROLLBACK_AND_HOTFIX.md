# Rollback and Hotfix Process

This runbook defines the release rollback and hotfix process for GoGRPCBridge.

## When to Trigger

Trigger rollback or hotfix when any of the following occurs after release:

- critical functional regression in canonical API paths
- security issue requiring immediate mitigation
- severe performance regression beyond documented release tolerance
- release artifact integrity or packaging failure

## Decision Guide

1. If issue severity is critical and immediate user impact is ongoing, rollback first.
2. If rollback is not possible or older version has equal/worse risk, publish hotfix.
3. If impact is low and mitigations exist, schedule next patch release with documented risk.

## Rollback Procedure

1. Identify last known good tag.
2. Repoint deployment/package references to last known good tag.
3. Announce rollback scope and expected impact window.
4. Verify smoke checks and key integration paths on rolled-back version.
5. Open incident follow-up issue with root cause and corrective action owner.

## Hotfix Procedure

1. Branch from affected release tag:

```bash
git checkout -b hotfix/<issue-id> <release-tag>
```

2. Apply minimal root-cause fix.
3. Run required release checks:
   - quality gates
   - benchmark trend check
   - security release checklist
4. Update changelog with hotfix entry.
5. Tag patch release (`vX.Y.Z+1` semantic patch bump).
6. Publish release assets and notes with explicit hotfix scope.

## Validation Requirements

- release checklist completed (`RELEASE_CHECKLIST.md`)
- security checklist completed (`SECURITY_RELEASE_CHECKLIST.md`)
- release performance template filled (`RELEASE_PERFORMANCE_TEMPLATE.md`)
- migration notes updated if behavior changed

## Communication Template

- Incident summary: `<what failed>`
- User impact: `<who/how many>`
- Action taken: `<rollback|hotfix>`
- Targeted follow-up: `<issue link>`
- Next update time: `<timestamp>`

## Post-Incident Follow-Up

- Document root cause and timeline.
- Add regression test coverage for failure mode.
- Update troubleshooting, threat model, and release checklists as needed.
- Track prevention action to completion before next minor release.
