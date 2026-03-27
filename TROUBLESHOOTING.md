# Troubleshooting

This page lists common setup and runtime failures for GoGRPCBridge and the fastest fix path.

## 1) `go get github.com/monstercameron/GoGRPCBridge@latest` fails

Symptom:
- `Repository not found` for `github.com/monstercameron/GoGRPCBridge`

Cause:
- current published source mapping still routes through `github.com/monstercameron/grpc-tunnel`

Workaround:
- use a remote module replace (not a local filesystem replace) until direct `@latest` resolution is verified

```go
replace github.com/monstercameron/GoGRPCBridge => github.com/monstercameron/grpc-tunnel v0.0.10
```

Track:
- see root roadmap item `S10.3` for direct consumer `go get` verification

## 2) Browser client fails with TLS config error

Symptom:
- validation rejects `TunnelConfig.TLSConfig` or `TunnelConfig.ShouldUseTLS` in WASM

Cause:
- browser networking owns TLS in `js/wasm` builds

Fix:
- use same-origin target (`""` or `"/grpc"`) in WASM
- remove direct TLS config fields from WASM tunnel config

## 3) Bridge rejects connection with origin error

Symptom:
- WebSocket upgrade fails or clients disconnect immediately

Cause:
- `CheckOrigin` denies incoming `Origin` header

Fix:
- add explicit allow-list logic for trusted origins
- confirm exact browser `Origin` value in server logs before tightening rules

## 4) gRPC calls fail after tunnel connects

Symptom:
- tunnel opens but RPC requests fail or hang

Cause:
- backend gRPC server is not reachable from bridge target address

Fix:
- confirm bridge target host and port
- verify backend gRPC service is listening
- test backend with a direct local client before routing through tunnel

## 5) Build or codegen tools missing

Symptom:
- build/bootstrap reports missing `protoc` or plugin binaries

Fix:
- install required tools and ensure they are on `PATH`:
  - `go`
  - `git`
  - `protoc`
  - `protoc-gen-go`
  - `protoc-gen-go-grpc`

From repo root you can run:

```powershell
.\scripts\bootstrap-gogrpcbridge.ps1
```
