//go:build !js && !wasm

package grpctunnel

import (
	"math"
	"net/url"
	"strings"
	"testing"
	"time"
)

// FuzzParseTunnelTargetURL fuzzes websocket target normalization for parser safety invariants.
func FuzzParseTunnelTargetURL(parseF *testing.F) {
	parseF.Add("", false)
	parseF.Add("%", false)
	parseF.Add("localhost:8080", false)
	parseF.Add(":8080", false)
	parseF.Add("ws://127.0.0.1:8080/grpc", false)
	parseF.Add("wss://api.example.com/grpc", true)
	parseF.Add("http://api.example.com", false)

	parseF.Fuzz(func(parseT *testing.T, parseTarget string, shouldTunnelUseTLS bool) {
		defer func() {
			if parseRecovered := recover(); parseRecovered != nil {
				parseT.Fatalf("ParseTunnelTargetURL panicked: %v", parseRecovered)
			}
		}()

		parseNormalizedTarget, parseErr := ParseTunnelTargetURL(parseTarget, shouldTunnelUseTLS)
		if parseErr != nil {
			return
		}

		parseTargetURL, parseParseErr := url.Parse(parseNormalizedTarget)
		if parseParseErr != nil {
			parseT.Fatalf("ParseTunnelTargetURL returned invalid URL %q: %v", parseNormalizedTarget, parseParseErr)
		}
		if parseTargetURL.Host == "" {
			parseT.Fatalf("ParseTunnelTargetURL returned empty host for %q", parseNormalizedTarget)
		}
		if parseTargetURL.Scheme != "ws" && parseTargetURL.Scheme != "wss" {
			parseT.Fatalf("ParseTunnelTargetURL returned unsupported scheme %q", parseTargetURL.Scheme)
		}
	})
}

// FuzzGetTunnelConfigError fuzzes tunnel config validation to ensure robust rejection without panics.
func FuzzGetTunnelConfigError(parseF *testing.F) {
	parseF.Add("localhost:8080", int64(time.Second), false, int64(time.Second), int64(2*time.Second), 1.6, 0.2, int64(time.Second), false)
	parseF.Add("ws://127.0.0.1:8080/grpc", int64(0), false, int64(0), int64(0), 0.0, 0.0, int64(0), false)
	parseF.Add("%", int64(-time.Second), true, int64(-time.Second), int64(-time.Second), -1.0, -1.0, int64(-time.Second), true)
	parseF.Add("http://bad.example", int64(time.Second), false, int64(time.Second), int64(2*time.Second), math.NaN(), math.Inf(1), int64(time.Second), true)

	parseF.Fuzz(func(parseT *testing.T, parseTarget string, parseHandshakeTimeoutNanos int64, shouldTunnelUseTLS bool, parseInitialDelayNanos int64, parseMaxDelayNanos int64, parseMultiplier float64, parseJitter float64, parseMinConnectTimeoutNanos int64, shouldUseReconnect bool) {
		defer func() {
			if parseRecovered := recover(); parseRecovered != nil {
				parseT.Fatalf("GetTunnelConfigError panicked: %v", parseRecovered)
			}
		}()

		var parseReconnectConfig *ReconnectConfig
		if shouldUseReconnect {
			parseReconnectConfig = &ReconnectConfig{
				InitialDelay:      time.Duration(parseInitialDelayNanos),
				MaxDelay:          time.Duration(parseMaxDelayNanos),
				Multiplier:        parseMultiplier,
				Jitter:            parseJitter,
				MinConnectTimeout: time.Duration(parseMinConnectTimeoutNanos),
			}
		}

		parseConfig := TunnelConfig{
			Target:           parseTarget,
			ShouldUseTLS:     shouldTunnelUseTLS,
			HandshakeTimeout: time.Duration(parseHandshakeTimeoutNanos),
			ReconnectConfig:  parseReconnectConfig,
		}

		parseErr := GetTunnelConfigError(parseConfig)
		if strings.TrimSpace(parseTarget) == "" && parseErr == nil {
			parseT.Fatal("GetTunnelConfigError accepted empty target")
		}
	})
}
