//go:build !js && !wasm

package grpctunnel

import (
	"errors"
	"io"
	"testing"
)

// TestWebSocketConn_ReadAfterClose verifies closed connections stop reads immediately.
func TestWebSocketConn_ReadAfterClose(parseT *testing.T) {
	parseConnection := &webSocketConn{}
	parseConnection.isClosed.Store(true)

	parseBytesRead, parseErr := parseConnection.Read(make([]byte, 8))
	if parseBytesRead != 0 {
		parseT.Fatalf("Read() bytes = %d, want 0", parseBytesRead)
	}
	if !errors.Is(parseErr, io.EOF) {
		parseT.Fatalf("Read() error = %v, want %v", parseErr, io.EOF)
	}
}

// TestWebSocketConn_WriteAfterClose verifies closed connections reject writes.
func TestWebSocketConn_WriteAfterClose(parseT *testing.T) {
	parseConnection := &webSocketConn{}
	parseConnection.isClosed.Store(true)

	parseBytesWritten, parseErr := parseConnection.Write([]byte("payload"))
	if parseBytesWritten != 0 {
		parseT.Fatalf("Write() bytes = %d, want 0", parseBytesWritten)
	}
	if !errors.Is(parseErr, io.ErrClosedPipe) {
		parseT.Fatalf("Write() error = %v, want %v", parseErr, io.ErrClosedPipe)
	}
}
