# Docs Freshness Policy

This policy defines how documentation freshness is reviewed and maintained.

## Review Cadence

- Monthly:
  - quick audit of README, Quick Start, and troubleshooting pages
- Quarterly:
  - full docs index review including migration, security, and operations docs
- Release-bound:
  - verify release checklist, changelog, and performance/security templates before tag

## Freshness Checklist

- links resolve to existing files and sections
- commands run as documented or are explicitly marked as examples
- API names and examples match current canonical surface
- security and ops guidance still reflect current deployment expectations

## Ownership

- Docs owner performs cadence reviews and tracks updates.
- API owner validates API accuracy in docs after interface changes.
- Release owner confirms release-bound docs are complete before tagging.

## Stale Content Handling

- If guidance is obsolete, update or explicitly mark as deprecated.
- If immediate update is not possible, add a warning note and owner.
- Track larger doc rewrites as roadmap items with target completion date.
