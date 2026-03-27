package grpctunnel

import (
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"
	grpcbackoff "google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// TunnelConfig configures typed client connection creation for grpctunnel.
type TunnelConfig struct {
	// Target is the connection target. In non-WASM builds this should be a
	// host:port, :port, ws:// URL, or wss:// URL. In WASM builds it may also be
	// empty or a path (for same-origin inference).
	Target string
	// TLSConfig configures the TLS settings for non-WASM websocket dialing.
	TLSConfig *tls.Config
	// ShouldUseTLS forces wss:// inference for non-WASM target normalization.
	ShouldUseTLS bool
	// Headers configures optional HTTP headers for the websocket handshake.
	Headers http.Header
	// Subprotocols configures optional websocket subprotocol negotiation.
	Subprotocols []string
	// Proxy configures an optional proxy selector for non-WASM websocket dialing.
	Proxy func(*http.Request) (*url.URL, error)
	// HandshakeTimeout limits websocket handshake duration for non-WASM clients.
	HandshakeTimeout time.Duration
	// ShouldEnableCompression enables websocket per-message compression where supported.
	ShouldEnableCompression bool
	// ReconnectConfig configures optional gRPC reconnect and backoff behavior.
	ReconnectConfig *ReconnectConfig
	// GRPCOptions passes through grpc.DialOption values.
	GRPCOptions []grpc.DialOption
}

// BridgeConfig configures typed server handler creation for grpctunnel.
type BridgeConfig struct {
	// CheckOrigin validates websocket upgrade origins.
	// If nil, gorilla/websocket applies its default same-origin policy.
	CheckOrigin func(r *http.Request) bool
	// ReadBufferSize configures websocket read buffer size. Zero uses defaults.
	ReadBufferSize int
	// WriteBufferSize configures websocket write buffer size. Zero uses defaults.
	WriteBufferSize int
	// ReadLimitBytes configures an optional websocket read limit.
	// Zero applies a secure default limit.
	ReadLimitBytes int64
	// ShouldDisableReadLimit disables websocket read-size limiting.
	// Use this only when an upstream boundary enforces strict payload limits.
	ShouldDisableReadLimit bool
	// PingInterval configures optional server-initiated websocket ping cadence.
	PingInterval time.Duration
	// IdleTimeout configures how long the bridge waits for client activity or pong frames.
	IdleTimeout time.Duration
	// ShouldEnableCompression enables websocket per-message compression where supported.
	ShouldEnableCompression bool
	// MaxActiveConnections limits total concurrent websocket tunnel connections.
	// Zero disables this guard.
	MaxActiveConnections int
	// MaxConnectionsPerClient limits concurrent websocket tunnel connections per client key.
	// Client key is derived from request remote address host. Zero disables this guard.
	MaxConnectionsPerClient int
	// MaxUpgradesPerClientPerMinute limits websocket upgrade attempts per client key over a 1-minute window.
	// Zero disables this guard.
	MaxUpgradesPerClientPerMinute int
	// OnConnect is called when a websocket client connects.
	OnConnect func(r *http.Request)
	// OnDisconnect is called when a websocket client disconnects.
	OnDisconnect func(r *http.Request)
}

// ReconnectConfig configures optional gRPC reconnect backoff behavior.
type ReconnectConfig struct {
	// InitialDelay configures the first reconnect delay. Zero uses gRPC defaults.
	InitialDelay time.Duration
	// MaxDelay configures the maximum reconnect delay. Zero uses gRPC defaults.
	MaxDelay time.Duration
	// Multiplier configures exponential backoff growth. Zero uses gRPC defaults.
	Multiplier float64
	// Jitter configures reconnect jitter. Zero uses gRPC defaults.
	Jitter float64
	// MinConnectTimeout configures the minimum connection timeout. Zero uses gRPC defaults.
	MinConnectTimeout time.Duration
}

// ToolingConfig configures the optional direct gRPC tooling server helpers.
type ToolingConfig struct {
	// ShouldEnableReflection registers the gRPC reflection service when absent.
	ShouldEnableReflection bool
	// ShouldEnableHealthService registers the standard gRPC health service when absent.
	ShouldEnableHealthService bool
	// ShouldEnablePprof exposes net/http/pprof handlers under DebugPathPrefix.
	ShouldEnablePprof bool
	// DebugPathPrefix configures the pprof route prefix. Empty uses /debug/pprof/.
	DebugPathPrefix string
}

// ApplyTunnelInsecureCredentials appends insecure transport credentials to the
// provided grpc dial option slice and returns the resulting slice.
func ApplyTunnelInsecureCredentials(parseDialOptions []grpc.DialOption) []grpc.DialOption {
	parseResult := append([]grpc.DialOption{}, parseDialOptions...)
	parseResult = append(parseResult, grpc.WithTransportCredentials(insecure.NewCredentials()))
	return parseResult
}

// GetReconnectConfigError validates optional reconnect policy settings.
func GetReconnectConfigError(parseConfig ReconnectConfig) error {
	if parseConfig.InitialDelay < 0 {
		return fmt.Errorf("grpctunnel: reconnect InitialDelay must be >= 0")
	}
	if parseConfig.MaxDelay < 0 {
		return fmt.Errorf("grpctunnel: reconnect MaxDelay must be >= 0")
	}
	if parseConfig.MinConnectTimeout < 0 {
		return fmt.Errorf("grpctunnel: reconnect MinConnectTimeout must be >= 0")
	}
	if parseConfig.Multiplier < 0 {
		return fmt.Errorf("grpctunnel: reconnect Multiplier must be >= 0")
	}
	if math.IsNaN(parseConfig.Multiplier) || math.IsInf(parseConfig.Multiplier, 0) {
		return fmt.Errorf("grpctunnel: reconnect Multiplier must be finite")
	}
	if parseConfig.Jitter < 0 {
		return fmt.Errorf("grpctunnel: reconnect Jitter must be >= 0")
	}
	if math.IsNaN(parseConfig.Jitter) || math.IsInf(parseConfig.Jitter, 0) {
		return fmt.Errorf("grpctunnel: reconnect Jitter must be finite")
	}
	return nil
}

// ApplyTunnelReconnectPolicy appends reconnect dial options onto an option slice.
func ApplyTunnelReconnectPolicy(parseDialOptions []grpc.DialOption, parseConfig ReconnectConfig) ([]grpc.DialOption, error) {
	if parseErr := GetReconnectConfigError(parseConfig); parseErr != nil {
		return nil, parseErr
	}

	parseBackoffConfig := grpcbackoff.DefaultConfig
	if parseConfig.InitialDelay > 0 {
		parseBackoffConfig.BaseDelay = parseConfig.InitialDelay
	}
	if parseConfig.MaxDelay > 0 {
		parseBackoffConfig.MaxDelay = parseConfig.MaxDelay
	}
	if parseConfig.Multiplier > 0 {
		parseBackoffConfig.Multiplier = parseConfig.Multiplier
	}
	if parseConfig.Jitter > 0 {
		parseBackoffConfig.Jitter = parseConfig.Jitter
	}

	parseConnectParams := grpc.ConnectParams{
		Backoff: parseBackoffConfig,
	}
	if parseConfig.MinConnectTimeout > 0 {
		parseConnectParams.MinConnectTimeout = parseConfig.MinConnectTimeout
	}

	parseResult := append([]grpc.DialOption{}, parseDialOptions...)
	parseResult = append(parseResult, grpc.WithConnectParams(parseConnectParams))
	return parseResult, nil
}

// buildTunnelGRPCDialTarget normalizes gRPC dial target values for custom websocket dialers.
func buildTunnelGRPCDialTarget(parseTarget string, parseTunnelURL string) string {
	parseTrimmedTarget := strings.TrimSpace(parseTarget)
	if parseTrimmedTarget == "" {
		return parseTunnelURL
	}

	parseTargetURL, parseErr := url.Parse(parseTrimmedTarget)
	if parseErr == nil && (parseTargetURL.Scheme == "ws" || parseTargetURL.Scheme == "wss") && parseTargetURL.Host != "" {
		return parseTargetURL.Host
	}

	return parseTrimmedTarget
}
