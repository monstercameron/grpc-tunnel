package bridge

import (
	"net/url"
	"testing"
	"time"
)

// FuzzParseBridgeTargetURL fuzzes bridge backend target parsing for safety invariants.
func FuzzParseBridgeTargetURL(parseF *testing.F) {
	parseF.Add("")
	parseF.Add("%")
	parseF.Add("/grpc")
	parseF.Add("localhost:50051")
	parseF.Add("127.0.0.1:50051")
	parseF.Add("[::1]:50051")
	parseF.Add("api.example.com:443")
	parseF.Add("user:pass@localhost:50051")

	parseF.Fuzz(func(parseT *testing.T, parseTargetAddress string) {
		defer func() {
			if parseRecovered := recover(); parseRecovered != nil {
				parseT.Fatalf("parseBridgeTargetURL panicked: %v", parseRecovered)
			}
		}()

		parseTargetURL, parseErr := parseBridgeTargetURL(parseTargetAddress)
		if parseErr != nil {
			return
		}

		if parseTargetURL == nil {
			parseT.Fatal("parseBridgeTargetURL returned nil URL without error")
		}
		if parseTargetURL.Scheme != "http" {
			parseT.Fatalf("parseBridgeTargetURL scheme = %q, want http", parseTargetURL.Scheme)
		}
		if parseTargetURL.Host == "" {
			parseT.Fatal("parseBridgeTargetURL returned empty host without error")
		}

		parseNormalizedURL, parseNormalizeErr := url.Parse(parseTargetURL.String())
		if parseNormalizeErr != nil {
			parseT.Fatalf("normalized target URL parse error: %v", parseNormalizeErr)
		}
		if parseNormalizedURL.Host == "" {
			parseT.Fatal("normalized target URL host unexpectedly empty")
		}
	})
}

// FuzzGetHandlerConfigError fuzzes websocket and timeout config validation for panic safety.
func FuzzGetHandlerConfigError(parseF *testing.F) {
	parseF.Add(0, 0, int64(0), false, int64(0), int64(0), int64(0))
	parseF.Add(4096, 4096, int64(16<<20), false, int64(time.Second), int64(5*time.Second), int64(10*time.Second))
	parseF.Add(4096, 4096, int64(0), true, int64(0), int64(0), int64(10*time.Second))
	parseF.Add(-1, 4096, int64(0), false, int64(0), int64(0), int64(0))

	parseF.Fuzz(func(parseT *testing.T, parseReadBufferSize int, parseWriteBufferSize int, parseReadLimitBytes int64, shouldDisableReadLimit bool, parsePingIntervalNanos int64, parseIdleTimeoutNanos int64, parseBackendDialTimeoutNanos int64) {
		defer func() {
			if parseRecovered := recover(); parseRecovered != nil {
				parseT.Fatalf("getHandlerConfigError panicked: %v", parseRecovered)
			}
		}()

		parseConfig := Config{
			ReadBufferSize:         parseReadBufferSize,
			WriteBufferSize:        parseWriteBufferSize,
			ReadLimitBytes:         parseReadLimitBytes,
			ShouldDisableReadLimit: shouldDisableReadLimit,
			PingInterval:           time.Duration(parsePingIntervalNanos),
			IdleTimeout:            time.Duration(parseIdleTimeoutNanos),
			BackendDialTimeout:     time.Duration(parseBackendDialTimeoutNanos),
		}

		parseErr := getHandlerConfigError(parseConfig)
		if shouldDisableReadLimit && parseReadLimitBytes > 0 && parseErr == nil {
			parseT.Fatal("getHandlerConfigError allowed ReadLimitBytes with ShouldDisableReadLimit")
		}
	})
}
