# Roadmap to 10/10 Across All Categories

Target completion date: **May 8, 2026**

Scoring model reference:

- `PROFESSIONAL_RUBRIC.md`

Execution rule:

- Complete one checklist item at a time.
- Validate after each item.
- Record a checkpoint in this file after each completed item.

## Phase 1: Baseline and Hard Gates

- [x] Define enforceable quality gates and thresholds (`QUALITY_GATES.md`).
- [x] Add runner-based quality gate command (`go run ./tools/runner.go quality`).
- [x] Add benchmark baseline capture command (`go run ./tools/runner.go quality-baseline`).
- [x] Wire `quality` gate into CI workflows.
- [x] Add release-blocking gate summary output to CI artifacts.

## Phase 2: API and DX Unification

- [ ] Promote `pkg/grpctunnel` as canonical API entrypoint in docs and examples.
- [ ] Deprecate mixed-option legacy paths in docs with clear migration notices.
- [ ] Add typed helper functions for common client/server setup paths.
- [ ] Ensure all exported APIs have precise GoDoc and migration notes.

## Phase 3: Reliability and Security Hardening

- [ ] Add failure-mode tests for reconnect, cancellation, and malformed frame scenarios.
- [ ] Add explicit secure-default guidance and unsafe-mode opt-in docs.
- [ ] Add threat model document and security checklist.
- [ ] Ensure CI blocks on high severity/high confidence security findings.

## Phase 4: Performance Program

- [ ] Add benchmark trend tracking in CI using JSON snapshots.
- [ ] Identify and land one targeted optimization with benchmark evidence.
- [ ] Define per-release performance acceptance criteria in docs.
- [ ] Add benchmark regression notes to release documentation.

## Phase 5: Docs, Ops, and Governance

- [ ] Publish production operations guide (metrics, pprof, diagnostics, runbooks).
- [ ] Publish API compatibility/deprecation policy (semver + support window).
- [ ] Add release checklist (quality, docs, perf, security sign-off).
- [ ] Ensure README navigation points to all core operational and migration docs.

## Checkpoints

### Checkpoint 2026-03-26A

- Completed todo:
  - Completed Phase 1 baseline and hard-gate implementation.
- Files changed:
  - `.github/workflows/test.yml`
  - `.gitignore`
  - `benchmarks/comparison_test.go`
  - `benchmarks/quality_baseline.json`
  - `CONTRIBUTING.md`
  - `README.md`
  - `QUALITY_GATES.md`
  - `ROADMAP_TO_10S.md`
  - `tools/runner.go`
  - `tools/runner.go`
  - `PROFESSIONAL_RUBRIC.md`
- Validation run:
  - `go test ./tools -run '^$'`
  - `go run ./tools/runner.go help`
  - `go run ./tools/runner.go quality-baseline`
  - `go run ./tools/runner.go quality`
- Result:
  - Phase 1 quality gates are executable locally and wired into CI test workflow with artifacted gate summaries.
- Residual risk:
  - Race detector fallback can mask race execution in local non-CGO environments; CI must remain strict.
- Next suggested todo:
  - Start Phase 2 by deprecating mixed-option legacy API paths in docs.
