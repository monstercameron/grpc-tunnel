# Community Workflow

This document defines how community support and collaboration works for GoGRPCBridge.

## Channels

- Bug reports and feature requests: GitHub Issues
- Design and usage questions: GitHub Discussions
- Security-sensitive reports: private coordinated disclosure path (do not post exploit details publicly)

## Issue Intake Expectations

When opening an issue, include:

- environment details (OS, Go version, build target)
- minimal reproduction steps
- expected behavior
- actual behavior and error output
- affected version/tag

## Triage and Response Targets

- Initial maintainer acknowledgment:
  - critical issues: within 1 business day
  - standard issues: within 3 business days
- First triage classification:
  - severity label
  - scope label (`api`, `docs`, `ci`, `security`, `performance`, `examples`)
- Escalation:
  - critical regressions and security issues are prioritized over feature work

## Discussion Workflow

- Use Discussions for API design questions and integration strategy.
- Promote resolved high-signal discussions into docs updates where appropriate.
- Close stale discussions when answers are finalized and documented.

## PR Review Expectations

- Contributors should link related issue/discussion in PR description.
- Maintainers should request focused changes with reproducible evidence.
- Merge requires passing required CI checks and checklist compliance.

## Community Conduct

- Keep reports factual, reproducible, and respectful.
- Prefer actionable evidence over speculation.
- Document final resolution for future contributors.
