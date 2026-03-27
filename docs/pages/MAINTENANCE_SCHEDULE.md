# Maintenance Schedule

This schedule defines regular dependency and toolchain maintenance for GoGRPCBridge.

## Cadence

- Weekly:
  - review CI failures and flaky-test trends
  - review security scanner output deltas
- Monthly:
  - refresh direct dependencies where safe
  - verify benchmark trend drift against baseline
  - run full release checklist dry-run on default branch
- Quarterly:
  - evaluate Go toolchain bump and compatibility impact
  - review CI workflow dependencies and action versions
  - audit docs for stale setup or release instructions

## Dependency Update Policy

- Prioritize security and reliability updates first.
- Keep dependency updates small and grouped by purpose.
- Include changelog review notes for major/minor jumps.
- Re-run quality, trend, and browser lanes after updates.

## Toolchain Update Policy

- Evaluate latest stable Go version each quarter.
- Test candidate version in CI before default bump.
- Document migration notes for any toolchain-sensitive behavior changes.

## Ownership and Tracking

- Release owner tracks monthly maintenance completion.
- Security owner validates security-related dependency updates.
- Performance owner reviews benchmark trend changes after dependency/toolchain updates.
