//go:build !js && !wasm

package grpctunnel

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// TestBuildGrpctunnelLogLine_IncludesRequestAndTraceFields verifies structured grpctunnel logs include request metadata and OTel IDs.
func TestBuildGrpctunnelLogLine_IncludesRequestAndTraceFields(parseT *testing.T) {
	parseTraceID, parseTraceErr := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if parseTraceErr != nil {
		parseT.Fatalf("TraceIDFromHex() error: %v", parseTraceErr)
	}
	parseSpanID, parseSpanErr := trace.SpanIDFromHex("00f067aa0ba902b7")
	if parseSpanErr != nil {
		parseT.Fatalf("SpanIDFromHex() error: %v", parseSpanErr)
	}
	parseSpanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    parseTraceID,
		SpanID:     parseSpanID,
		TraceFlags: trace.FlagsSampled,
	})

	parseReq := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseReq.RemoteAddr = "127.0.0.1:32123"
	parseReq.Header.Set("Origin", "https://app.example.com")
	parseReq.Header.Set("X-Request-ID", "req-abc")
	parseReq = parseReq.WithContext(trace.ContextWithSpanContext(context.Background(), parseSpanContext))

	parseLogLine := buildGrpctunnelLogLine("grpctunnel.bridge", "WARN", "ws_upgrade_failed", parseReq, nil, "WebSocket upgrade failed")

	for _, parseExpected := range []string{
		`component="grpctunnel.bridge"`,
		`level="WARN"`,
		`event="ws_upgrade_failed"`,
		`request_id="req-abc"`,
		`remote_addr="127.0.0.1:32123"`,
		`origin="https://app.example.com"`,
		`method="GET"`,
		`path="/grpc"`,
		`trace_id="4bf92f3577b34da6a3ce929d0e0e4736"`,
		`span_id="00f067aa0ba902b7"`,
	} {
		if !strings.Contains(parseLogLine, parseExpected) {
			parseT.Fatalf("buildGrpctunnelLogLine() missing %q in %q", parseExpected, parseLogLine)
		}
	}
}

// TestBuildGrpctunnelLogLine_Defaults verifies default field values are applied when level/event/component are empty.
func TestBuildGrpctunnelLogLine_Defaults(parseT *testing.T) {
	parseLogLine := buildGrpctunnelLogLine("", "", "", nil, nil, "message")
	for _, parseExpected := range []string{
		`component="grpctunnel"`,
		`level="INFO"`,
		`event="grpctunnel_event"`,
	} {
		if !strings.Contains(parseLogLine, parseExpected) {
			parseT.Fatalf("buildGrpctunnelLogLine() missing %q in %q", parseExpected, parseLogLine)
		}
	}
}

// TestBuildBridgeHandler_LogsUpgradeFailureStructured verifies bridge handler emits structured logs for websocket upgrade failures.
func TestBuildBridgeHandler_LogsUpgradeFailureStructured(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	parseHandler, parseErr := BuildBridgeHandler(parseGrpcServer, BridgeConfig{})
	if parseErr != nil {
		parseT.Fatalf("BuildBridgeHandler() error: %v", parseErr)
	}

	var parseLogBuffer bytes.Buffer
	parseOriginalWriter := log.Writer()
	log.SetOutput(&parseLogBuffer)
	defer log.SetOutput(parseOriginalWriter)

	parseReq := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseW := httptest.NewRecorder()
	parseHandler.ServeHTTP(parseW, parseReq)

	parseLogOutput := parseLogBuffer.String()
	if !strings.Contains(parseLogOutput, `event="ws_upgrade_failed"`) {
		parseT.Fatalf("expected structured ws_upgrade_failed log, got %q", parseLogOutput)
	}
	if !strings.Contains(parseLogOutput, `component="grpctunnel.bridge"`) {
		parseT.Fatalf("expected structured component field, got %q", parseLogOutput)
	}
}
