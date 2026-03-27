//go:build !js && !wasm

//lint:file-ignore SA1019 grpc.DialContext is retained to preserve 1.x dial semantics and WithBlock behavior.

package grpctunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// ClientOption configures the WebSocket client behavior.
type ClientOption func(*clientOptions)

type clientOptions struct {
	tlsConfig               *tls.Config
	setTunnelHeaders        http.Header
	setTunnelSubprotocols   []string
	setTunnelProxy          func(*http.Request) (*url.URL, error)
	setTunnelReconnect      *ReconnectConfig
	setTunnelTimeout        time.Duration
	isUseTLS                bool
	shouldEnableCompression bool
}

// WithTLS enables secure WebSocket connections (wss://).
// This sets the TLS configuration for the WebSocket dialer.
// Passing nil still forces wss:// URL inference while using default TLS settings.
//
// Example:
//
//	conn, _ := grpctunnel.Dial("localhost:8080",
//	    grpctunnel.WithTLS(&tls.Config{InsecureSkipVerify: true}),
//	)
func WithTLS(parseConfig *tls.Config) ClientOption {
	return func(parseO *clientOptions) {
		parseO.isUseTLS = true
		parseO.tlsConfig = parseConfig
	}
}

// WithHeaders configures websocket handshake headers.
func WithHeaders(parseHeaders http.Header) ClientOption {
	return func(parseO *clientOptions) {
		parseO.setTunnelHeaders = parseHeaders.Clone()
	}
}

// WithHeader appends one websocket handshake header value.
func WithHeader(parseKey string, parseValue string) ClientOption {
	return func(parseO *clientOptions) {
		if parseO.setTunnelHeaders == nil {
			parseO.setTunnelHeaders = make(http.Header)
		}
		parseO.setTunnelHeaders.Add(parseKey, parseValue)
	}
}

// WithSubprotocols configures websocket subprotocol negotiation.
func WithSubprotocols(parseSubprotocols ...string) ClientOption {
	return func(parseO *clientOptions) {
		parseO.setTunnelSubprotocols = append([]string{}, parseSubprotocols...)
	}
}

// WithProxy configures a proxy selector for websocket dialing.
func WithProxy(parseProxy func(*http.Request) (*url.URL, error)) ClientOption {
	return func(parseO *clientOptions) {
		parseO.setTunnelProxy = parseProxy
	}
}

// WithHandshakeTimeout configures the websocket handshake timeout.
func WithHandshakeTimeout(parseTimeout time.Duration) ClientOption {
	return func(parseO *clientOptions) {
		parseO.setTunnelTimeout = parseTimeout
	}
}

// WithDialCompression enables websocket per-message compression for client dialing.
func WithDialCompression() ClientOption {
	return func(parseO *clientOptions) {
		parseO.shouldEnableCompression = true
	}
}

// WithReconnectPolicy configures optional gRPC reconnect backoff behavior.
func WithReconnectPolicy(parseConfig ReconnectConfig) ClientOption {
	return func(parseO *clientOptions) {
		parseReconnectConfig := parseConfig
		parseO.setTunnelReconnect = &parseReconnectConfig
	}
}

// splitDialOptions separates grpctunnel client options from grpc dial options.
func splitDialOptions(parseOpts []interface{}) ([]ClientOption, []grpc.DialOption, error) {
	var parseTunnelOpts []ClientOption
	var parseGrpcOpts []grpc.DialOption

	for _, parseOpt := range parseOpts {
		// Keep tunnel-specific options separate so they affect websocket dialing,
		// while grpc options continue through to grpc.DialContext untouched.
		switch parseTypedOption := parseOpt.(type) {
		case ClientOption:
			parseTunnelOpts = append(parseTunnelOpts, parseTypedOption)
		case grpc.DialOption:
			parseGrpcOpts = append(parseGrpcOpts, parseTypedOption)
		default:
			return nil, nil, fmt.Errorf("grpctunnel: unsupported dial option type %T", parseOpt)
		}
	}

	return parseTunnelOpts, parseGrpcOpts, nil
}

// ParseTunnelTargetURL normalizes a tunnel target into a websocket URL.
func ParseTunnelTargetURL(parseTarget string, shouldTunnelUseTLS bool) (string, error) {
	parseTarget = strings.TrimSpace(parseTarget)
	if parseTarget == "" {
		return "", fmt.Errorf("grpctunnel: target is required")
	}

	if strings.Contains(parseTarget, "://") &&
		!strings.HasPrefix(parseTarget, "ws://") &&
		!strings.HasPrefix(parseTarget, "wss://") {
		parseTargetURL, parseErr := url.Parse(parseTarget)
		if parseErr != nil {
			return "", fmt.Errorf("grpctunnel: invalid target %q: %w", parseTarget, parseErr)
		}
		return "", fmt.Errorf("grpctunnel: unsupported target scheme %q", parseTargetURL.Scheme)
	}

	if strings.HasPrefix(parseTarget, ":") {
		parseTarget = "localhost" + parseTarget
	}

	if !strings.HasPrefix(parseTarget, "ws://") && !strings.HasPrefix(parseTarget, "wss://") {
		parseScheme := "ws"
		if shouldTunnelUseTLS {
			parseScheme = "wss"
		}
		parseTarget = parseScheme + "://" + parseTarget
	}

	parseTargetURL, parseErr := url.Parse(parseTarget)
	if parseErr != nil {
		return "", fmt.Errorf("grpctunnel: invalid target %q: %w", parseTarget, parseErr)
	}
	if parseTargetURL.Scheme != "ws" && parseTargetURL.Scheme != "wss" {
		return "", fmt.Errorf("grpctunnel: unsupported target scheme %q", parseTargetURL.Scheme)
	}
	if parseTargetURL.Host == "" {
		return "", fmt.Errorf("grpctunnel: target host is required")
	}
	return parseTargetURL.String(), nil
}

// inferWebSocketURL converts a target address to a WebSocket URL.
// It keeps compatibility with legacy tests and wrappers.
func inferWebSocketURL(parseTarget string, isUseTLS bool) string {
	parseResult, parseErr := ParseTunnelTargetURL(parseTarget, isUseTLS)
	if parseErr != nil {
		return parseTarget
	}
	return parseResult
}

// buildTunnelDialer creates a custom gRPC dialer that establishes WebSocket connections.
func buildTunnelDialer(parseConfig TunnelConfig) func(context.Context, string) (net.Conn, error) {
	parseTargetURL, parseTargetErr := url.Parse(parseConfig.Target)
	if parseTargetErr != nil {
		return func(context.Context, string) (net.Conn, error) {
			return nil, parseTargetErr
		}
	}

	// Reuse parsed target and dialer configuration for each reconnect attempt.
	parseDialURL := parseTargetURL.String()
	parseHeadersTemplate := http.Header(nil)
	if parseConfig.Headers != nil {
		parseHeadersTemplate = parseConfig.Headers.Clone()
	}
	parseDialer := websocket.Dialer{
		TLSClientConfig:   parseConfig.TLSConfig,
		Subprotocols:      append([]string{}, parseConfig.Subprotocols...),
		Proxy:             parseConfig.Proxy,
		HandshakeTimeout:  parseConfig.HandshakeTimeout,
		WriteBufferPool:   buildWebSocketWriteBufferPool(parseDefaultWebSocketBufferSize),
		EnableCompression: parseConfig.ShouldEnableCompression,
	}

	return func(parseCtx context.Context, parseAddr string) (net.Conn, error) {
		parseWebsocket, _, parseErr := parseDialer.DialContext(parseCtx, parseDialURL, parseHeadersTemplate)
		if parseErr != nil {
			return nil, parseErr
		}
		return newWebSocketConn(parseWebsocket), nil
	}
}

// getTunnelConfigErrorWithoutTarget validates non-target TunnelConfig fields for non-WASM builds.
func getTunnelConfigErrorWithoutTarget(parseConfig TunnelConfig) error {
	if parseConfig.HandshakeTimeout < 0 {
		return fmt.Errorf("grpctunnel: HandshakeTimeout must be >= 0")
	}
	if parseConfig.ReconnectConfig != nil {
		if parseErr := GetReconnectConfigError(*parseConfig.ReconnectConfig); parseErr != nil {
			return parseErr
		}
	}
	return nil
}

// buildTunnelTargetURL normalizes websocket target URL from TunnelConfig.
func buildTunnelTargetURL(parseConfig TunnelConfig) (string, error) {
	shouldTunnelUseTLS := parseConfig.ShouldUseTLS || parseConfig.TLSConfig != nil
	return ParseTunnelTargetURL(parseConfig.Target, shouldTunnelUseTLS)
}

// GetTunnelConfigError validates TunnelConfig for non-WASM builds.
func GetTunnelConfigError(parseConfig TunnelConfig) error {
	if parseErr := getTunnelConfigErrorWithoutTarget(parseConfig); parseErr != nil {
		return parseErr
	}

	_, parseErr := buildTunnelTargetURL(parseConfig)
	return parseErr
}

// BuildTunnelConn creates a typed gRPC client connection over websocket transport.
func BuildTunnelConn(parseCtx context.Context, parseConfig TunnelConfig) (*grpc.ClientConn, error) {
	if parseErr := getTunnelConfigErrorWithoutTarget(parseConfig); parseErr != nil {
		return nil, parseErr
	}

	parseTunnelURL, parseErr := buildTunnelTargetURL(parseConfig)
	if parseErr != nil {
		return nil, parseErr
	}

	parseDialOptions := make([]grpc.DialOption, 0, len(parseConfig.GRPCOptions)+2)
	if parseConfig.ReconnectConfig != nil {
		parseReconnectOptions, parseErr := ApplyTunnelReconnectPolicy(nil, *parseConfig.ReconnectConfig)
		if parseErr != nil {
			return nil, parseErr
		}
		parseDialOptions = append(parseDialOptions, parseReconnectOptions...)
	}
	parseDialOptions = append(parseDialOptions, parseConfig.GRPCOptions...)
	parseDialOptions = append(parseDialOptions, grpc.WithContextDialer(buildTunnelDialer(TunnelConfig{
		Target:                  parseTunnelURL,
		TLSConfig:               parseConfig.TLSConfig,
		Headers:                 parseConfig.Headers,
		Subprotocols:            parseConfig.Subprotocols,
		Proxy:                   parseConfig.Proxy,
		HandshakeTimeout:        parseConfig.HandshakeTimeout,
		ShouldEnableCompression: parseConfig.ShouldEnableCompression,
	})))

	return grpc.DialContext(parseCtx, buildTunnelGRPCDialTarget(parseConfig.Target, parseTunnelURL), parseDialOptions...)
}

// Dial creates a gRPC client connection over WebSocket.
// The target can be:
//   - A WebSocket URL: "ws://localhost:8080" or "wss://api.example.com"
//   - A host:port: "localhost:8080" (infers ws://)
//   - A port: ":8080" (infers ws://localhost:8080)
//
// Additional options can include grpctunnel client options (e.g., WithTLS)
// and grpc.DialOption values (credentials, interceptors, etc.).
// Any option value that is neither ClientOption nor grpc.DialOption
// returns an error from Dial/DialContext.
//
// Example:
//
//	conn, err := grpctunnel.Dial("localhost:8080",
//	    grpctunnel.WithTLS(&tls.Config{InsecureSkipVerify: true}),
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//
//	client := proto.NewYourServiceClient(conn)
func Dial(parseTarget string, parseOpts ...interface{}) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), parseTarget, parseOpts...)
}

// DialContext creates a gRPC client connection over WebSocket with context.
// This is the context-aware version of Dial.
//
// The parseOpts list accepts a mix of:
//   - ClientOption values from this package (for tunnel behavior)
//   - grpc.DialOption values from google.golang.org/grpc
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	conn, err := grpctunnel.DialContext(ctx, "localhost:8080",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
func DialContext(parseCtx context.Context, parseTarget string, parseOpts ...interface{}) (*grpc.ClientConn, error) {
	parseTunnelOpts, parseGrpcOpts, parseErr := splitDialOptions(parseOpts)
	if parseErr != nil {
		return nil, parseErr
	}

	parseTunnelOptions := &clientOptions{}
	for _, parseTunnelOption := range parseTunnelOpts {
		parseTunnelOption(parseTunnelOptions)
	}

	return BuildTunnelConn(parseCtx, TunnelConfig{
		Target:                  parseTarget,
		TLSConfig:               parseTunnelOptions.tlsConfig,
		ShouldUseTLS:            parseTunnelOptions.isUseTLS,
		Headers:                 parseTunnelOptions.setTunnelHeaders,
		Subprotocols:            parseTunnelOptions.setTunnelSubprotocols,
		Proxy:                   parseTunnelOptions.setTunnelProxy,
		HandshakeTimeout:        parseTunnelOptions.setTunnelTimeout,
		ShouldEnableCompression: parseTunnelOptions.shouldEnableCompression,
		ReconnectConfig:         parseTunnelOptions.setTunnelReconnect,
		GRPCOptions:             parseGrpcOpts,
	})
}
