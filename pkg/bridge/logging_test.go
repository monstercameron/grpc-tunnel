package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// TestBuildBridgeLogLine_IncludesRequestAndTraceFields verifies structured logs include request metadata and OTel identifiers.
func TestBuildBridgeLogLine_IncludesRequestAndTraceFields(parseT *testing.T) {
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
	parseReq.Header.Set("X-Request-Id", "req-123")
	parseReq = parseReq.WithContext(trace.ContextWithSpanContext(context.Background(), parseSpanContext))

	parseLogLine := buildBridgeLogLine("WARN", "ws_upgrade_failed", parseReq, nil, "WebSocket upgrade failed")

	for _, parseExpected := range []string{
		`component=bridge`,
		`level="WARN"`,
		`event="ws_upgrade_failed"`,
		`request_id="req-123"`,
		`remote_addr="127.0.0.1:32123"`,
		`origin="https://app.example.com"`,
		`method="GET"`,
		`path="/grpc"`,
		`trace_id="4bf92f3577b34da6a3ce929d0e0e4736"`,
		`span_id="00f067aa0ba902b7"`,
	} {
		if !strings.Contains(parseLogLine, parseExpected) {
			parseT.Fatalf("buildBridgeLogLine() missing %q in %q", parseExpected, parseLogLine)
		}
	}
}

// TestBuildBridgeLogLine_Defaults verifies empty level/event values resolve to safe defaults.
func TestBuildBridgeLogLine_Defaults(parseT *testing.T) {
	parseLogLine := buildBridgeLogLine("", "", nil, nil, "Bridge message")
	if !strings.Contains(parseLogLine, `level="INFO"`) {
		parseT.Fatalf("buildBridgeLogLine() level missing default: %q", parseLogLine)
	}
	if !strings.Contains(parseLogLine, `event="bridge_event"`) {
		parseT.Fatalf("buildBridgeLogLine() event missing default: %q", parseLogLine)
	}
}

// TestGetBridgeRequestID verifies request-id extraction precedence across common header names.
func TestGetBridgeRequestID(parseT *testing.T) {
	parseReq := httptest.NewRequest(http.MethodGet, "/", nil)
	parseReq.Header.Set("X-Correlation-ID", "corr-1")
	parseReq.Header.Set("X-Request-ID", "req-2")

	parseRequestID := getBridgeRequestID(parseReq)
	if parseRequestID != "req-2" {
		parseT.Fatalf("getBridgeRequestID() = %q, want %q", parseRequestID, "req-2")
	}
}
