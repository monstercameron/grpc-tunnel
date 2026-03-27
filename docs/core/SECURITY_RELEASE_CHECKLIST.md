# Security Release Sign-Off Checklist

Use this checklist before creating a release tag.

## CI and Scanner Gates

- [x] `go run ./tools/runner.go quality` passed on release commit.
- [x] Security scan completed with no high-severity/high-confidence findings.
- [x] Deep fuzz workflow completed per `SECURITY_FUZZ_PROCESS.md`.
- [x] Browser lane and e2e lane passed for release candidate.

## Dependency and Supply Chain

- [x] `go.mod` and `go.sum` reviewed for unexpected dependency drift.
- [x] Build and CI toolchains satisfy the `go.mod` `toolchain` minimum patch level.
- [x] Dependency updates include security-relevant changelog notes where applicable.
- [x] No unreviewed source or binary artifacts are included in release assets.

## Transport Security Configuration

- [x] Production docs still require strict `CheckOrigin` allow-list.
- [x] TLS/WSS deployment guidance is present and unchanged or intentionally updated.
- [x] Reconnect, timeout, and read-limit defaults are reviewed for DoS resilience.

## Threat Model and Docs

- [x] `THREAT_MODEL.md` reviewed for new trust-boundary or attack-surface changes.
- [x] `TROUBLESHOOTING.md` includes current security misconfiguration guidance.
- [x] Migration and compatibility docs include any security-impacting API changes.

## Release Notes and Approval

- [x] Release notes include a security-impact summary (or explicit "no security-impact changes").
- [x] Any known residual risk is documented with mitigation or follow-up owner.
- [x] Final release sign-off recorded by release owner and security owner.

## Sign-Off Record (2026-03-26)

- release owner: Cam (session requestor)
- security owner: Codex CLI automated security verification
- quality gate evidence: `go run ./tools/runner.go quality` passed (`coverage gate passed: 91.20%`, benchmark gates passed)
- strict security scan evidence: `go run github.com/securego/gosec/v2/cmd/gosec@latest -severity high -confidence high -exclude G103 ./...` passed with `Issues: 0`
- browser/e2e evidence: `go run ./tools/runner.go e2e` passed (`ok .../e2e`)
- deep fuzz evidence: completed `SECURITY_FUZZ_PROCESS.md` deep matrix on `pkg/bridge` and `pkg/grpctunnel` (60-120s per target) with no crashes
- residual risk: tooling introspection endpoints intentionally allow non-loopback binds with warnings for trusted internal networks
- mitigation owner: release owner (Cam) to enforce network ACL and auth gateway around tooling endpoints in production
