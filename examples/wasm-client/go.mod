module grpc-tunnel/client

go 1.24.0

toolchain go1.24.10

replace grpc-tunnel => ../../

require (
	google.golang.org/grpc v1.76.0
	grpc-tunnel v0.0.0-00010101000000-000000000000
)

require (
	github.com/deckarep/golang-set/v2 v2.7.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.4 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/playwright-community/playwright-go v0.5200.1 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
