# API Migration Guide

This guide covers migration to the typed `grpctunnel` API.

## Quick Migration Checklist

1. Replace legacy client dial calls with `BuildTunnelConn`.
2. Replace legacy server wrapping with `BuildBridgeHandler` or `HandleBridgeMux`.
3. Move ad-hoc options into `TunnelConfig` and `BridgeConfig`.
4. Add startup validation with `GetTunnelConfigError` and `GetBridgeConfigError`.
5. Keep legacy wrappers only where migration is intentionally deferred.

For known module-resolution issues during dependency setup, see [TROUBLESHOOTING.md](./TROUBLESHOOTING.md).

## Recommended API

Use these typed entry points for new code:

- `BuildTunnelConn(ctx, TunnelConfig)`
- `BuildBridgeHandler(grpcServer, BridgeConfig)`
- `HandleBridgeMux(mux, path, grpcServer, BridgeConfig)`
- `GetTunnelConfigError(TunnelConfig)`
- `GetBridgeConfigError(BridgeConfig)`

Legacy wrappers (`Dial`, `DialContext`, `Wrap`, `Serve`, `ListenAndServe`) remain supported.

## Old to New Mapping

| Old API | New API |
| --- | --- |
| `Dial` / `DialContext` | `BuildTunnelConn` |
| `Wrap` | `BuildBridgeHandler` |
| `mux.Handle(path, Wrap(...))` | `HandleBridgeMux` |
| ad-hoc TLS/options | `TunnelConfig` fields + `GRPCOptions` |

## Client Example (Non-WASM)

```go
parseConnection, parseConnectionError := grpctunnel.BuildTunnelConn(parseContext, grpctunnel.TunnelConfig{
    Target: "wss://api.example.com/grpc",
    GRPCOptions: []grpc.DialOption{
        grpc.WithTransportCredentials(credentials.NewTLS(parseTLSConfig)),
        grpc.WithBlock(),
    },
})
if parseConnectionError != nil {
    return parseConnectionError
}
defer parseConnection.Close()
```

## Client Example (WASM)

```go
parseConnection, parseConnectionError := grpctunnel.BuildTunnelConn(parseContext, grpctunnel.TunnelConfig{
    Target: "", // same-origin inference
    GRPCOptions: grpctunnel.ApplyTunnelInsecureCredentials(nil),
})
if parseConnectionError != nil {
    return parseConnectionError
}
defer parseConnection.Close()
```

Important:

- In WASM, `TLSConfig` and `ShouldUseTLS` are rejected by validation.
- Browser TLS is controlled by page origin and browser networking.

## Server Example (Mux)

```go
parseMux := http.NewServeMux()

if parseHandleError := grpctunnel.HandleBridgeMux(parseMux, "/grpc", parseGrpcServer, grpctunnel.BridgeConfig{
    CheckOrigin: parseCheckOrigin,
}); parseHandleError != nil {
    return parseHandleError
}
```

## Validation Before Startup

```go
if parseTunnelError := grpctunnel.GetTunnelConfigError(parseTunnelConfig); parseTunnelError != nil {
    return parseTunnelError
}
if parseBridgeError := grpctunnel.GetBridgeConfigError(parseBridgeConfig); parseBridgeError != nil {
    return parseBridgeError
}
```
