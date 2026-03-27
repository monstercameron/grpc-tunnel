//go:build !js && !wasm

package grpctunnel

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// GetToolingConfigError validates additive tooling helper configuration.
func GetToolingConfigError(parseConfig ToolingConfig) error {
	if parseConfig.DebugPathPrefix != "" {
		if !strings.HasPrefix(parseConfig.DebugPathPrefix, "/") {
			return fmt.Errorf("grpctunnel: DebugPathPrefix must start with /")
		}
		if !strings.HasSuffix(parseConfig.DebugPathPrefix, "/") {
			return fmt.Errorf("grpctunnel: DebugPathPrefix must end with /")
		}
	}
	return nil
}

// BuildToolingHandler builds an optional direct gRPC tooling handler for grpcurl, grpcui, and pprof.
func BuildToolingHandler(parseGrpcServer *grpc.Server, parseConfig ToolingConfig) (http.Handler, *health.Server, error) {
	if parseGrpcServer == nil {
		return nil, nil, fmt.Errorf("grpctunnel: grpc server is required")
	}
	if parseErr := GetToolingConfigError(parseConfig); parseErr != nil {
		return nil, nil, parseErr
	}

	parseHealthServer := ensureToolingServices(parseGrpcServer, parseConfig)
	warnToolingExposure(parseConfig)
	parseMux := http.NewServeMux()
	if parseConfig.ShouldEnablePprof {
		registerToolingPprofHandlers(parseMux, parseConfig)
	}
	parseMux.Handle("/", h2c.NewHandler(parseGrpcServer, &http2.Server{}))
	return parseMux, parseHealthServer, nil
}

// warnToolingExposure logs security-sensitive tooling exposure warnings.
func warnToolingExposure(parseConfig ToolingConfig) {
	logGrpctunnelEvent("grpctunnel.tooling", "WARN", "tooling_exposure", nil, nil, "Tooling handler exposes direct gRPC access; bind tooling endpoints to trusted networks only")
	if parseConfig.ShouldEnableReflection {
		logGrpctunnelEvent("grpctunnel.tooling", "WARN", "tooling_reflection_enabled", nil, nil, "gRPC reflection is enabled on tooling handler")
	}
	if parseConfig.ShouldEnablePprof {
		logGrpctunnelEvent("grpctunnel.tooling", "WARN", "tooling_pprof_enabled", nil, nil, "pprof is enabled on tooling handler")
	}
}

// ListenAndServeTooling starts an additive direct gRPC tooling server on a separate address.
func ListenAndServeTooling(parseAddr string, parseGrpcServer *grpc.Server, parseConfig ToolingConfig) error {
	if parseErr := getToolingListenAddressError(parseAddr, parseConfig); parseErr != nil {
		return parseErr
	}
	if shouldWarnToolingNonLoopbackBind(parseAddr, parseConfig) {
		logGrpctunnelEvent(
			"grpctunnel.tooling",
			"WARN",
			"tooling_bind_non_loopback",
			nil,
			nil,
			fmt.Sprintf(
				"Tooling server with reflection/pprof is bound to non-loopback address %q; restrict access with trusted network boundaries and auth controls",
				parseAddr,
			),
		)
	}

	parseHandler, _, parseErr := BuildToolingHandler(parseGrpcServer, parseConfig)
	if parseErr != nil {
		return parseErr
	}

	parseServer := &http.Server{
		Addr:         parseAddr,
		Handler:      parseHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return parseServer.ListenAndServe()
}

// ensureToolingServices registers optional reflection and health services when absent.
func ensureToolingServices(parseGrpcServer *grpc.Server, parseConfig ToolingConfig) *health.Server {
	parseServiceInfo := parseGrpcServer.GetServiceInfo()
	if parseConfig.ShouldEnableReflection && !hasToolingService(parseServiceInfo, "grpc.reflection.v1alpha.ServerReflection") {
		reflection.Register(parseGrpcServer)
	}

	if parseConfig.ShouldEnableHealthService && !hasToolingService(parseServiceInfo, grpc_health_v1.Health_ServiceDesc.ServiceName) {
		parseHealthServer := health.NewServer()
		parseHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		grpc_health_v1.RegisterHealthServer(parseGrpcServer, parseHealthServer)
		return parseHealthServer
	}

	return nil
}

// hasToolingService reports whether the gRPC server already exposes a named service.
func hasToolingService(parseServices map[string]grpc.ServiceInfo, parseServiceName string) bool {
	_, hasService := parseServices[parseServiceName]
	return hasService
}

// registerToolingPprofHandlers mounts pprof handlers under the configured path prefix.
func registerToolingPprofHandlers(parseMux *http.ServeMux, parseConfig ToolingConfig) {
	parseDebugPathPrefix := parseConfig.DebugPathPrefix
	if parseDebugPathPrefix == "" {
		parseDebugPathPrefix = "/debug/pprof/"
	}

	parseDebugPathBase := strings.TrimSuffix(parseDebugPathPrefix, "/")
	parseMux.Handle(parseDebugPathPrefix, http.HandlerFunc(pprof.Index))
	parseMux.Handle(parseDebugPathBase+"/cmdline", http.HandlerFunc(pprof.Cmdline))
	parseMux.Handle(parseDebugPathBase+"/profile", http.HandlerFunc(pprof.Profile))
	parseMux.Handle(parseDebugPathBase+"/symbol", http.HandlerFunc(pprof.Symbol))
	parseMux.Handle(parseDebugPathBase+"/trace", http.HandlerFunc(pprof.Trace))
}

// getToolingListenAddressError blocks wildcard tooling binds when reflection/pprof are enabled.
func getToolingListenAddressError(parseAddr string, parseConfig ToolingConfig) error {
	if !parseConfig.ShouldEnableReflection && !parseConfig.ShouldEnablePprof {
		return nil
	}
	if !shouldWarnToolingWildcardBind(parseAddr) {
		return nil
	}
	return fmt.Errorf(
		"grpctunnel: refusing tooling listen address %q with reflection/pprof enabled; bind to loopback (127.0.0.1 or ::1) or disable introspection features",
		parseAddr,
	)
}

// shouldWarnToolingWildcardBind reports whether a listen address resolves to wildcard interfaces.
func shouldWarnToolingWildcardBind(parseAddr string) bool {
	parseTrimmedAddress := strings.TrimSpace(parseAddr)
	if parseTrimmedAddress == "" || strings.HasPrefix(parseTrimmedAddress, ":") {
		return true
	}

	parseHost, _, parseErr := net.SplitHostPort(parseTrimmedAddress)
	if parseErr != nil {
		return false
	}
	parseHost = strings.TrimSpace(parseHost)
	return parseHost == "" || parseHost == "0.0.0.0" || parseHost == "::" || parseHost == "[::]"
}

// shouldWarnToolingNonLoopbackBind reports whether a tooling listen address is non-loopback.
func shouldWarnToolingNonLoopbackBind(parseAddr string, parseConfig ToolingConfig) bool {
	if !parseConfig.ShouldEnableReflection && !parseConfig.ShouldEnablePprof {
		return false
	}
	if shouldWarnToolingWildcardBind(parseAddr) {
		return false
	}

	parseHost, _, parseErr := net.SplitHostPort(strings.TrimSpace(parseAddr))
	if parseErr != nil {
		return false
	}
	parseHost = strings.Trim(parseHost, "[]")
	if strings.EqualFold(parseHost, "localhost") {
		return false
	}

	parseHostIP := net.ParseIP(parseHost)
	if parseHostIP != nil {
		return !parseHostIP.IsLoopback()
	}

	return parseHost != ""
}
