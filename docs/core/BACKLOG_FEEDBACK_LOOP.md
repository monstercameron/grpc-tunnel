# Backlog Feedback Loop

This process defines how production incidents and user feedback feed backlog prioritization.

## Input Sources

- production incidents and postmortems
- support issues and discussions
- CI failure trends
- adoption/friction monthly reviews

## Intake Workflow

1. Capture signal in issue tracker with source label:
   - `source:incident`
   - `source:user-feedback`
   - `source:ci`
2. Add severity and scope labels.
3. Link to supporting evidence (logs, repro, benchmark trend, discussion).

## Prioritization Rules

- `source:incident` items with reliability/security impact are highest priority.
- recurring friction points are prioritized over one-off low-impact requests.
- backlog entries without reproducible evidence remain in discovery state.

## Backlog Item Minimum Fields

- problem statement
- impact assessment
- reproduction status
- proposed fix scope
- owner
- target milestone/release

## Review Cadence

- weekly quick triage for new inputs
- monthly backlog review tied to adoption/friction metrics
- post-incident immediate review for corrective actions

## Closure Criteria

Backlog item closes only when:

- fix is released
- docs/tests are updated where applicable
- incident or feedback thread is linked with resolution summary
