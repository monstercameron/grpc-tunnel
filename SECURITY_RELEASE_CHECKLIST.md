# Security Release Sign-Off Checklist

Use this checklist before creating a release tag.

## CI and Scanner Gates

- [ ] `go run ./tools/runner.go quality` passed on release commit.
- [ ] Security scan completed with no high-severity/high-confidence findings.
- [ ] Deep fuzz workflow completed per `SECURITY_FUZZ_PROCESS.md`.
- [ ] Browser lane and e2e lane passed for release candidate.

## Dependency and Supply Chain

- [ ] `go.mod` and `go.sum` reviewed for unexpected dependency drift.
- [ ] Build and CI toolchains satisfy the `go.mod` `toolchain` minimum patch level.
- [ ] Dependency updates include security-relevant changelog notes where applicable.
- [ ] No unreviewed source or binary artifacts are included in release assets.

## Transport Security Configuration

- [ ] Production docs still require strict `CheckOrigin` allow-list.
- [ ] TLS/WSS deployment guidance is present and unchanged or intentionally updated.
- [ ] Reconnect, timeout, and read-limit defaults are reviewed for DoS resilience.

## Threat Model and Docs

- [ ] `THREAT_MODEL.md` reviewed for new trust-boundary or attack-surface changes.
- [ ] `TROUBLESHOOTING.md` includes current security misconfiguration guidance.
- [ ] Migration and compatibility docs include any security-impacting API changes.

## Release Notes and Approval

- [ ] Release notes include a security-impact summary (or explicit "no security-impact changes").
- [ ] Any known residual risk is documented with mitigation or follow-up owner.
- [ ] Final release sign-off recorded by release owner and security owner.
