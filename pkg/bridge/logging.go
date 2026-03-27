package bridge

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

const parseDefaultBridgeLogLevel = "INFO"
const parseDefaultBridgeLogEvent = "bridge_event"

// logBridgeEvent emits a structured bridge log line with optional request and error context.
func logBridgeEvent(parseLogger Logger, parseLevel string, parseEvent string, parseRequest *http.Request, parseErr error, parseMessage string) {
	if parseLogger == nil {
		return
	}
	parseLogger.Printf("%s", buildBridgeLogLine(parseLevel, parseEvent, parseRequest, parseErr, parseMessage))
}

// buildBridgeLogLine builds a single structured bridge log line.
func buildBridgeLogLine(parseLevel string, parseEvent string, parseRequest *http.Request, parseErr error, parseMessage string) string {
	parseLevel = strings.TrimSpace(parseLevel)
	if parseLevel == "" {
		parseLevel = parseDefaultBridgeLogLevel
	}
	parseEvent = strings.TrimSpace(parseEvent)
	if parseEvent == "" {
		parseEvent = parseDefaultBridgeLogEvent
	}

	parseBuilder := strings.Builder{}
	parseBuilder.WriteString("component=bridge")
	appendBridgeLogField(&parseBuilder, "level", parseLevel)
	appendBridgeLogField(&parseBuilder, "event", parseEvent)
	appendBridgeLogField(&parseBuilder, "msg", parseMessage)

	if parseRequest != nil {
		appendBridgeLogField(&parseBuilder, "request_id", getBridgeRequestID(parseRequest))
		appendBridgeLogField(&parseBuilder, "remote_addr", strings.TrimSpace(parseRequest.RemoteAddr))
		appendBridgeLogField(&parseBuilder, "origin", strings.TrimSpace(parseRequest.Header.Get("Origin")))
		appendBridgeLogField(&parseBuilder, "method", strings.TrimSpace(parseRequest.Method))
		if parseRequest.URL != nil {
			appendBridgeLogField(&parseBuilder, "path", strings.TrimSpace(parseRequest.URL.Path))
		}

		parseTraceID, parseSpanID := getBridgeTraceSpanIDs(parseRequest.Context())
		appendBridgeLogField(&parseBuilder, "trace_id", parseTraceID)
		appendBridgeLogField(&parseBuilder, "span_id", parseSpanID)
	}

	if parseErr != nil {
		appendBridgeLogField(&parseBuilder, "error", strings.TrimSpace(parseErr.Error()))
	}

	return parseBuilder.String()
}

// appendBridgeLogField appends one key/value field to a structured bridge log line.
func appendBridgeLogField(parseBuilder *strings.Builder, parseKey string, parseValue string) {
	if parseBuilder == nil {
		return
	}
	parseKey = strings.TrimSpace(parseKey)
	parseValue = strings.TrimSpace(parseValue)
	if parseKey == "" || parseValue == "" {
		return
	}
	parseBuilder.WriteByte(' ')
	parseBuilder.WriteString(parseKey)
	parseBuilder.WriteByte('=')
	parseBuilder.WriteString(strconv.Quote(parseValue))
}

// getBridgeRequestID resolves a correlation/request identifier from common ingress headers.
func getBridgeRequestID(parseRequest *http.Request) string {
	if parseRequest == nil {
		return ""
	}

	parseRequestIDHeaders := []string{
		"X-Request-Id",
		"X-Request-ID",
		"X-Correlation-Id",
		"X-Correlation-ID",
	}
	for _, parseHeaderName := range parseRequestIDHeaders {
		parseHeaderValue := strings.TrimSpace(parseRequest.Header.Get(parseHeaderName))
		if parseHeaderValue != "" {
			return parseHeaderValue
		}
	}
	return ""
}

// getBridgeTraceSpanIDs returns OTel trace/span identifiers from a context when present.
func getBridgeTraceSpanIDs(parseContext context.Context) (string, string) {
	if parseContext == nil {
		return "", ""
	}
	parseSpanContext := trace.SpanContextFromContext(parseContext)
	if !parseSpanContext.IsValid() {
		return "", ""
	}
	return parseSpanContext.TraceID().String(), parseSpanContext.SpanID().String()
}
