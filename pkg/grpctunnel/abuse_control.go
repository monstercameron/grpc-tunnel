//go:build !js && !wasm

package grpctunnel

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const parseBridgeAbuseWindowDuration = time.Minute

// storeBridgeAbuseRateWindow tracks one client's fixed-window upgrade attempt counts.
type storeBridgeAbuseRateWindow struct {
	getWindowStartedAt time.Time
	getWindowCount     int
}

// bridgeAbuseGuard stores in-memory counters used to enforce websocket abuse controls.
type bridgeAbuseGuard struct {
	setGuardLock sync.Mutex
	setConfig    BridgeConfig

	getActiveConnections       int
	storeClientConnections     map[string]int
	storeClientUpgradeAttempts map[string]storeBridgeAbuseRateWindow
}

// buildBridgeAbuseGuard creates an abuse guard for bridge runtime controls.
func buildBridgeAbuseGuard(parseConfig BridgeConfig) *bridgeAbuseGuard {
	return &bridgeAbuseGuard{
		setConfig:                  parseConfig,
		storeClientConnections:     map[string]int{},
		storeClientUpgradeAttempts: map[string]storeBridgeAbuseRateWindow{},
	}
}

// reserveBridgeConnection validates abuse controls and reserves one connection slot.
func (parseGuard *bridgeAbuseGuard) reserveBridgeConnection(parseRequest *http.Request, parseNow time.Time) error {
	if parseGuard == nil {
		return nil
	}

	parseClientKey := buildBridgeClientKey(parseRequest)
	if parseClientKey == "" {
		parseClientKey = "unknown"
	}

	parseGuard.setGuardLock.Lock()
	defer parseGuard.setGuardLock.Unlock()

	if parseGuard.setConfig.MaxUpgradesPerClientPerMinute > 0 {
		parseWindow := parseGuard.storeClientUpgradeAttempts[parseClientKey]
		if parseWindow.getWindowStartedAt.IsZero() || parseNow.Sub(parseWindow.getWindowStartedAt) >= parseBridgeAbuseWindowDuration {
			parseWindow = storeBridgeAbuseRateWindow{
				getWindowStartedAt: parseNow,
				getWindowCount:     0,
			}
		}
		if parseWindow.getWindowCount >= parseGuard.setConfig.MaxUpgradesPerClientPerMinute {
			return fmt.Errorf("upgrade rate exceeded for client %q", parseClientKey)
		}
		parseWindow.getWindowCount++
		parseGuard.storeClientUpgradeAttempts[parseClientKey] = parseWindow
	}

	if parseGuard.setConfig.MaxActiveConnections > 0 && parseGuard.getActiveConnections >= parseGuard.setConfig.MaxActiveConnections {
		return fmt.Errorf("active connection cap exceeded")
	}

	parseClientConnections := parseGuard.storeClientConnections[parseClientKey]
	if parseGuard.setConfig.MaxConnectionsPerClient > 0 && parseClientConnections >= parseGuard.setConfig.MaxConnectionsPerClient {
		return fmt.Errorf("per-client connection cap exceeded for client %q", parseClientKey)
	}

	parseGuard.getActiveConnections++
	parseGuard.storeClientConnections[parseClientKey] = parseClientConnections + 1
	return nil
}

// clearBridgeConnection releases one reserved connection slot for abuse controls.
func (parseGuard *bridgeAbuseGuard) clearBridgeConnection(parseRequest *http.Request) {
	if parseGuard == nil {
		return
	}

	parseClientKey := buildBridgeClientKey(parseRequest)
	if parseClientKey == "" {
		parseClientKey = "unknown"
	}

	parseGuard.setGuardLock.Lock()
	defer parseGuard.setGuardLock.Unlock()

	if parseGuard.getActiveConnections > 0 {
		parseGuard.getActiveConnections--
	}

	parseClientConnections := parseGuard.storeClientConnections[parseClientKey]
	if parseClientConnections <= 1 {
		delete(parseGuard.storeClientConnections, parseClientKey)
		return
	}
	parseGuard.storeClientConnections[parseClientKey] = parseClientConnections - 1
}

// buildBridgeClientKey derives a stable client key for abuse controls from request remote address.
func buildBridgeClientKey(parseRequest *http.Request) string {
	if parseRequest == nil {
		return ""
	}

	parseRemoteAddress := strings.TrimSpace(parseRequest.RemoteAddr)
	if parseRemoteAddress == "" {
		return ""
	}

	parseHost, _, parseErr := net.SplitHostPort(parseRemoteAddress)
	if parseErr == nil {
		return strings.TrimSpace(parseHost)
	}
	return parseRemoteAddress
}
