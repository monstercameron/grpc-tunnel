# Adoption Metrics and Friction Tracking

This document defines how GoGRPCBridge adoption and user friction are tracked.

## Adoption Metrics

Track at minimum:

- package import adoption trend (internal consumers + external reports)
- active integration count (known apps/services using bridge transport)
- release upgrade adoption (latest version uptake)
- docs usage signals (high-traffic pages and search queries)

## Friction Metrics

Track at minimum:

- top recurring setup failures
- top recurring runtime failures
- issue/discussion themes by count
- time-to-first-success for new contributors/integrators

## Data Sources

- issue tracker labels and discussion tags
- CI failure categories
- support/discussion summary notes
- release retrospective notes

## Review Cadence

- monthly metrics snapshot
- quarterly trend review with roadmap update

## Friction Backlog Format

For each friction item:

- problem statement
- user impact
- frequency
- current workaround
- owner
- target milestone

## Output Requirement

Each monthly review should produce:

- top 3 adoption gains
- top 3 friction points
- concrete backlog updates linked to owners
