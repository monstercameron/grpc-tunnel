# Threat Model and Trust Boundaries

This document defines the security model for GoGRPCBridge.

## Scope

In scope:

- WebSocket bridge transport between browser clients and Go gRPC servers
- Browser-facing upgrade endpoint behavior
- Tunnel connection lifecycle and transport-level failure modes

Out of scope:

- Application business authorization policy
- Application identity provider implementation
- Server-side model/prompt/inference policy in downstream services

## Assets

- Auth/session credentials propagated by the application layer
- gRPC request and response payloads
- Connection metadata (origin, headers, client address)
- Service availability and tunnel reliability

## Trust Boundaries

1. Browser runtime to public network boundary.
2. Public network to bridge endpoint boundary.
3. Bridge process to backend gRPC service boundary.
4. Application auth policy boundary above transport layer.

## Threat Actors

- Untrusted internet clients
- Cross-site browser contexts attempting unauthorized websocket upgrades
- Internal or external actors attempting denial-of-service via connection or frame abuse
- Misconfigured deployments that unintentionally disable transport protections

## Entry Points

- WebSocket upgrade endpoint (`/grpc` or equivalent)
- Bridge transport configuration (`CheckOrigin`, keepalive, read limits, timeouts)
- Client dialing configuration (`Target`, reconnect policy, TLS usage)

## Primary Threats and Controls

| Threat | Risk | Required Control |
| --- | --- | --- |
| Cross-origin websocket abuse | Unauthorized browser origins connect | Enforce strict `CheckOrigin` allow-list in production |
| Plaintext transport exposure | Traffic interception or tampering | Serve bridge over HTTPS/WSS with TLS termination policy |
| Tooling endpoint overexposure | Reflection/pprof reveals service and runtime internals | Bind tooling to loopback or trusted internal network; keep auth/network ACL in front |
| Token leakage in logs | Credential disclosure | Keep auth data out transport logs and sanitize application logging |
| Connection flood / slow consumer | Resource exhaustion | Set server timeouts, read limits, keepalive, bounded buffering, and abuse controls (`MaxActiveConnections`, `MaxConnectionsPerClient`, `MaxUpgradesPerClientPerMinute`) |
| Malformed or protocol-invalid frames | Transport instability or parser abuse | Maintain strict binary-frame handling and malformed-frame test coverage |
| Reconnect storm behavior | Availability degradation | Use bounded reconnect backoff (`ReconnectConfig`) with finite numeric inputs and monitor failures |

## Security Assumptions

- Applications enforce authentication and authorization before business-side effects.
- Deployments terminate TLS correctly and do not expose unsecured bridge endpoints publicly.
- For compatibility `pkg/bridge` backend transport uses plaintext h2c; production deployments should keep this hop on loopback/private network boundaries and enable `Config.ShouldRequireLoopbackBackend` to fail non-loopback targets at startup.
- Origin policy is explicitly configured for production environments (never rely on permissive development stubs).
- Operators keep default websocket read-limit protection enabled unless an upstream boundary enforces stricter limits.

## Operational Security Requirements

- Run CI security scanning and fail on high-severity/high-confidence findings.
- Keep dependency versions current and review release notes for security fixes.
- Document security sign-off before release tags.

## Residual Risks

- Transport cannot prevent misuse when application auth policy is weak or bypassed.
- Browser-side compromised environments can still exfiltrate user-held credentials.
- DoS resistance is bounded by infrastructure-level rate limiting and capacity controls.
