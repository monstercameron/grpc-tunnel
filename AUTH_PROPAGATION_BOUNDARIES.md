# Auth Propagation and Token-Handling Boundaries

This document defines what GoGRPCBridge should and should not do for authentication propagation.

## Boundary Definition

GoGRPCBridge is a transport layer.

- It carries authenticated requests.
- It does not decide user identity or authorization policy.
- It does not mint, refresh, or persist tokens.

Application code remains the authority for auth semantics.

## Recommended Propagation Model

For browser-based applications:

1. Prefer same-origin session cookies with secure server-side session validation.
2. Keep bearer-token logic in application-owned middleware/interceptors when required.
3. Validate user identity and permissions in backend gRPC handlers before side effects.

## Token-Handling Rules

- Do not place tokens in URL query strings.
- Do not log bearer tokens, cookie values, or full `Authorization` headers.
- Do not store long-lived tokens in unsafe browser-accessible storage without clear threat acceptance.
- Do use short-lived credentials and rotation where possible.
- Do scope credentials by audience and least privilege.

## Transport Integration Guidance

- Use transport metadata only as a carrier for already-issued app credentials.
- Keep auth extraction and validation in app/server interceptors above bridge transport.
- Treat `CheckOrigin` as browser-origin protection, not user authentication.
- Treat TLS/WSS as confidentiality and integrity controls, not identity controls.

## Server Responsibility Split

- Bridge layer:
  - websocket upgrade and tunnel forwarding
  - origin policy enforcement
  - connection lifecycle hooks
- Application layer:
  - credential validation
  - user identity resolution
  - authorization and policy enforcement
  - audit logging and token redaction

## Verification Checklist

- [ ] Auth validation occurs in application interceptors or service handlers.
- [ ] Logs redact auth headers and token-like values.
- [ ] Origin policy and TLS policy are both enabled in production.
- [ ] Threat model and release checklist reflect current auth design.
