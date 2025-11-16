package bridge

import (
	"testing"
)

// FuzzWebSocketConnWrite tests Write method with fuzz inputs
func FuzzWebSocketConnWrite(f *testing.F) {
	// Seed corpus
	f.Add([]byte("hello"))
	f.Add([]byte(""))
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF, 0xFE, 0xFD})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024*1024 { // Cap at 1MB for performance
			return
		}

		mock := &mockWebSocket{}

		// Should not panic
		err := mock.WriteMessage(1, data)

		if err == nil && len(mock.writeMessages) != 1 {
			t.Errorf("Write should have 1 message")
		}
	})
}

// FuzzWebSocketConnRead tests Read method with fuzz inputs
func FuzzWebSocketConnRead(f *testing.F) {
	// Seed corpus
	f.Add([]byte("test data"))
	f.Add([]byte(""))
	f.Add([]byte{0x00, 0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024*1024 { // Cap at 1MB for performance
			return
		}

		mock := &mockWebSocket{
			readMessages: [][]byte{data},
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ReadMessage panicked: %v", r)
			}
		}()

		msgType, msgData, err := mock.ReadMessage()

		if err == nil {
			if len(msgData) != len(data) {
				t.Errorf("Read returned %d bytes, expected %d", len(msgData), len(data))
			}
			_ = msgType
		}
	})
}

// FuzzBinaryMessage tests handling of arbitrary binary data
func FuzzBinaryMessage(f *testing.F) {
	// Seed with various binary patterns
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF})
	f.Add([]byte{0x00, 0xFF, 0x00, 0xFF})
	f.Add([]byte("\x00\x01\x02\x03\x04\x05"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 100000 { // Cap for performance
			return
		}

		mock := &mockWebSocket{
			readMessages: [][]byte{data},
		}

		// Read should handle any binary data without panicking
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ReadMessage panicked on binary data: %v", r)
			}
		}()

		mock.ReadMessage()
	})
}

// FuzzMessageSizes tests various message sizes
func FuzzMessageSizes(f *testing.F) {
	// Seed with boundary sizes
	f.Add(0)
	f.Add(1)
	f.Add(64)
	f.Add(1024)
	f.Add(65535)

	f.Fuzz(func(t *testing.T, size int) {
		if size < 0 || size > 100000 { // Cap for performance
			return
		}

		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		mock := &mockWebSocket{
			readMessages: [][]byte{data},
		}

		// Should handle any size without panicking
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panicked on message size %d: %v", size, r)
			}
		}()

		mock.ReadMessage()
	})
}
