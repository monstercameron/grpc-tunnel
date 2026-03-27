# External Consumer Example

This example is a standalone Go module that imports:

- `github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel`

without any local `replace` directives.

Current upstream note:
- this module uses a remote replace mapping to `github.com/monstercameron/grpc-tunnel v0.0.10`
- this is a temporary publish-path workaround until `go get github.com/monstercameron/GoGRPCBridge@latest` is directly resolvable

## Verify

From this directory:

```bash
go test ./...
go run .
```
