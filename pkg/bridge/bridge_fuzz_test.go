package bridge

import (
	"testing"
)

// FuzzWebSocketConnWrite tests Write method with fuzz inputs
func FuzzWebSocketConnWrite(parseF *testing.F) {
	// Seed corpus
	parseF.Add([]byte("hello"))
	parseF.Add([]byte(""))
	parseF.Add([]byte{0x00})
	parseF.Add([]byte{0xFF, 0xFE, 0xFD})

	parseF.Fuzz(func(parseT *testing.T, parseData []byte) {
		if len(parseData) > 1024*1024 { // Cap at 1MB for performance
			return
		}

		parseMock := &mockWebSocket{}

		// Should not panic
		parseErr := parseMock.WriteMessage(1, parseData)

		if parseErr == nil && len(parseMock.writeMessages) != 1 {
			parseT.Errorf("Write should have 1 message")
		}
	})
}

// FuzzWebSocketConnRead tests Read method with fuzz inputs
func FuzzWebSocketConnRead(parseF *testing.F) {
	// Seed corpus
	parseF.Add([]byte("test data"))
	parseF.Add([]byte(""))
	parseF.Add([]byte{0x00, 0x01, 0x02})

	parseF.Fuzz(func(parseT *testing.T, parseData []byte) {
		if len(parseData) > 1024*1024 { // Cap at 1MB for performance
			return
		}

		parseMock := &mockWebSocket{
			readMessages: [][]byte{parseData},
		}

		// Should not panic
		defer func() {
			if parseR := recover(); parseR != nil {
				parseT.Errorf("ReadMessage panicked: %v", parseR)
			}
		}()

		parseMsgType, parseMsgData, parseErr := parseMock.ReadMessage()

		if parseErr == nil {
			if len(parseMsgData) != len(parseData) {
				parseT.Errorf("Read returned %d bytes, expected %d", len(parseMsgData), len(parseData))
			}
			_ = parseMsgType
		}
	})
}

// FuzzBinaryMessage tests handling of arbitrary binary data
func FuzzBinaryMessage(parseF *testing.F) {
	// Seed with various binary patterns
	parseF.Add([]byte{0x00})
	parseF.Add([]byte{0xFF})
	parseF.Add([]byte{0x00, 0xFF, 0x00, 0xFF})
	parseF.Add([]byte("\x00\x01\x02\x03\x04\x05"))

	parseF.Fuzz(func(parseT *testing.T, parseData []byte) {
		if len(parseData) > 100000 { // Cap for performance
			return
		}

		parseMock := &mockWebSocket{
			readMessages: [][]byte{parseData},
		}

		// Read should handle any binary data without panicking
		defer func() {
			if parseR := recover(); parseR != nil {
				parseT.Errorf("ReadMessage panicked on binary data: %v", parseR)
			}
		}()

		parseMock.ReadMessage()
	})
}

// FuzzMessageSizes tests various message sizes
func FuzzMessageSizes(parseF *testing.F) {
	// Seed with boundary sizes
	parseF.Add(0)
	parseF.Add(1)
	parseF.Add(64)
	parseF.Add(1024)
	parseF.Add(65535)

	parseF.Fuzz(func(parseT *testing.T, parseSize int) {
		if parseSize < 0 || parseSize > 100000 { // Cap for performance
			return
		}

		parseData := make([]byte, parseSize)
		for parseI := range parseData {
			parseData[parseI] = byte(parseI % 256)
		}

		parseMock := &mockWebSocket{
			readMessages: [][]byte{parseData},
		}

		// Should handle any size without panicking
		defer func() {
			if parseR := recover(); parseR != nil {
				parseT.Errorf("Panicked on message size %d: %v", parseSize, parseR)
			}
		}()

		parseMock.ReadMessage()
	})
}
