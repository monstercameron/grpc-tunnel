//go:build !js && !wasm

package grpctunnel

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

const parseDefaultGrpctunnelLogLevel = "INFO"
const parseDefaultGrpctunnelLogEvent = "grpctunnel_event"
const parseDefaultGrpctunnelLogComponent = "grpctunnel"

// logGrpctunnelEvent emits one structured grpctunnel log event line.
func logGrpctunnelEvent(parseComponent string, parseLevel string, parseEvent string, parseRequest *http.Request, parseErr error, parseMessage string) {
	log.Printf("%s", buildGrpctunnelLogLine(parseComponent, parseLevel, parseEvent, parseRequest, parseErr, parseMessage))
}

// buildGrpctunnelLogLine builds a structured grpctunnel log line with optional request and OTel context fields.
func buildGrpctunnelLogLine(parseComponent string, parseLevel string, parseEvent string, parseRequest *http.Request, parseErr error, parseMessage string) string {
	parseComponent = strings.TrimSpace(parseComponent)
	if parseComponent == "" {
		parseComponent = parseDefaultGrpctunnelLogComponent
	}
	parseLevel = strings.TrimSpace(parseLevel)
	if parseLevel == "" {
		parseLevel = parseDefaultGrpctunnelLogLevel
	}
	parseEvent = strings.TrimSpace(parseEvent)
	if parseEvent == "" {
		parseEvent = parseDefaultGrpctunnelLogEvent
	}

	parseBuilder := strings.Builder{}
	appendGrpctunnelLogField(&parseBuilder, "component", parseComponent)
	appendGrpctunnelLogField(&parseBuilder, "level", parseLevel)
	appendGrpctunnelLogField(&parseBuilder, "event", parseEvent)
	appendGrpctunnelLogField(&parseBuilder, "msg", parseMessage)

	if parseRequest != nil {
		appendGrpctunnelLogField(&parseBuilder, "request_id", getGrpctunnelRequestID(parseRequest))
		appendGrpctunnelLogField(&parseBuilder, "remote_addr", strings.TrimSpace(parseRequest.RemoteAddr))
		appendGrpctunnelLogField(&parseBuilder, "origin", strings.TrimSpace(parseRequest.Header.Get("Origin")))
		appendGrpctunnelLogField(&parseBuilder, "method", strings.TrimSpace(parseRequest.Method))
		if parseRequest.URL != nil {
			appendGrpctunnelLogField(&parseBuilder, "path", strings.TrimSpace(parseRequest.URL.Path))
		}

		parseTraceID, parseSpanID := getGrpctunnelTraceSpanIDs(parseRequest.Context())
		appendGrpctunnelLogField(&parseBuilder, "trace_id", parseTraceID)
		appendGrpctunnelLogField(&parseBuilder, "span_id", parseSpanID)
	}

	if parseErr != nil {
		appendGrpctunnelLogField(&parseBuilder, "error", strings.TrimSpace(parseErr.Error()))
	}

	return parseBuilder.String()
}

// appendGrpctunnelLogField appends one key/value field to a structured grpctunnel log line.
func appendGrpctunnelLogField(parseBuilder *strings.Builder, parseKey string, parseValue string) {
	if parseBuilder == nil {
		return
	}
	parseKey = strings.TrimSpace(parseKey)
	parseValue = strings.TrimSpace(parseValue)
	if parseKey == "" || parseValue == "" {
		return
	}
	if parseBuilder.Len() > 0 {
		parseBuilder.WriteByte(' ')
	}
	parseBuilder.WriteString(parseKey)
	parseBuilder.WriteByte('=')
	parseBuilder.WriteString(strconv.Quote(parseValue))
}

// getGrpctunnelRequestID resolves a correlation/request identifier from common ingress headers.
func getGrpctunnelRequestID(parseRequest *http.Request) string {
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

// getGrpctunnelTraceSpanIDs returns OTel trace/span identifiers from context when available.
func getGrpctunnelTraceSpanIDs(parseContext context.Context) (string, string) {
	if parseContext == nil {
		return "", ""
	}
	parseSpanContext := trace.SpanContextFromContext(parseContext)
	if !parseSpanContext.IsValid() {
		return "", ""
	}
	return parseSpanContext.TraceID().String(), parseSpanContext.SpanID().String()
}
