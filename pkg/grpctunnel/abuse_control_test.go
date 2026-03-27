//go:build !js && !wasm

package grpctunnel

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestGetBridgeConfigError_AbuseControlValidation verifies abuse-control limits reject negative values.
func TestGetBridgeConfigError_AbuseControlValidation(parseT *testing.T) {
	parseTests := []BridgeConfig{
		{MaxActiveConnections: -1},
		{MaxConnectionsPerClient: -1},
		{MaxUpgradesPerClientPerMinute: -1},
	}

	for _, parseConfig := range parseTests {
		if parseErr := GetBridgeConfigError(parseConfig); parseErr == nil {
			parseT.Fatalf("GetBridgeConfigError() expected validation error for %#v, got nil", parseConfig)
		}
	}
}

// TestBridgeAbuseGuard_ConnectionCaps verifies global and per-client connection caps are enforced.
func TestBridgeAbuseGuard_ConnectionCaps(parseT *testing.T) {
	parseGuard := buildBridgeAbuseGuard(BridgeConfig{
		MaxActiveConnections:    2,
		MaxConnectionsPerClient: 1,
	})
	parseNow := time.Now()

	parseClientOneReq := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseClientOneReq.RemoteAddr = "203.0.113.10:50000"
	parseClientTwoReq := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseClientTwoReq.RemoteAddr = "203.0.113.11:50001"

	if parseErr := parseGuard.reserveBridgeConnection(parseClientOneReq, parseNow); parseErr != nil {
		parseT.Fatalf("reserveBridgeConnection(client1) error: %v", parseErr)
	}
	if parseErr := parseGuard.reserveBridgeConnection(parseClientOneReq, parseNow); parseErr == nil {
		parseT.Fatal("reserveBridgeConnection(client1 second) expected per-client cap error, got nil")
	}
	if parseErr := parseGuard.reserveBridgeConnection(parseClientTwoReq, parseNow); parseErr != nil {
		parseT.Fatalf("reserveBridgeConnection(client2) error: %v", parseErr)
	}

	parseClientThreeReq := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseClientThreeReq.RemoteAddr = "203.0.113.12:50002"
	if parseErr := parseGuard.reserveBridgeConnection(parseClientThreeReq, parseNow); parseErr == nil {
		parseT.Fatal("reserveBridgeConnection(client3) expected global cap error, got nil")
	}

	parseGuard.clearBridgeConnection(parseClientOneReq)
	if parseErr := parseGuard.reserveBridgeConnection(parseClientThreeReq, parseNow); parseErr != nil {
		parseT.Fatalf("reserveBridgeConnection(client3 after clear) error: %v", parseErr)
	}
}

// TestBuildBridgeHandler_RejectsUpgradeWhenRateLimitExceeded verifies bridge handler returns 429 when per-client upgrade rate is exceeded.
func TestBuildBridgeHandler_RejectsUpgradeWhenRateLimitExceeded(parseT *testing.T) {
	parseGrpcServer := grpc.NewServer()
	defer parseGrpcServer.Stop()

	parseHandler, parseErr := BuildBridgeHandler(parseGrpcServer, BridgeConfig{
		MaxUpgradesPerClientPerMinute: 1,
	})
	if parseErr != nil {
		parseT.Fatalf("BuildBridgeHandler() error: %v", parseErr)
	}

	parseReqOne := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseReqOne.RemoteAddr = "203.0.113.42:50100"
	parseWOne := httptest.NewRecorder()
	parseHandler.ServeHTTP(parseWOne, parseReqOne)

	parseReqTwo := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	parseReqTwo.RemoteAddr = "203.0.113.42:50101"
	parseWTwo := httptest.NewRecorder()
	parseHandler.ServeHTTP(parseWTwo, parseReqTwo)

	if parseWTwo.Code != http.StatusTooManyRequests {
		parseT.Fatalf("second ServeHTTP() status = %d, want %d", parseWTwo.Code, http.StatusTooManyRequests)
	}
}
