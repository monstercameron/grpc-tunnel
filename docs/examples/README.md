# Examples Catalog

This catalog marks canonical, compatibility, and support examples so contributors avoid extending stale paths.

## Status Guide

- `canonical`: preferred examples for new integrations
- `compatibility`: legacy-compatible path, kept for migration and maintenance
- `support`: helper example used to run or validate other demos

## Example Matrix

| Path | Status | Notes |
| --- | --- | --- |
| `direct-bridge` | canonical | Main bridge reference for new server-side integration patterns. |
| `wasm-client` | canonical | Main browser WASM client reference. |
| `external-consumer` | canonical | Standalone consumer module proving non-local dependency integration. |
| `grpc-server` | support | Backend service used by bridge examples; not a bridge API showcase. |
| `simple-bridge` | compatibility | Minimal legacy helper path; overlaps production shape, keep small and migration-focused. |
| `custom-router` | compatibility | Legacy helper path for existing mux integrations; avoid new feature investment. |
| `production-bridge` | compatibility | Legacy helper path with production options; use for migration references only. |
| `_shared` | support | Shared protobufs, helper code, and fixtures for multiple examples. |

## Maintenance Rules

- Add new bridge examples using `pkg/grpctunnel` and mark them `canonical`.
- Do not add new behavior to compatibility examples unless required for bugfix parity.
- If two examples represent the same flow, keep the canonical one and document migration from the compatibility path.
