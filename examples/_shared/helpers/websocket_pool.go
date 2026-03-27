package helpers

import "sync"

const parseDefaultWebSocketBufferSize = 4096

var cacheWebSocketWriteBufferPools sync.Map

// buildWebSocketWriteBufferPool returns a shared pool for a websocket write-buffer size.
func buildWebSocketWriteBufferPool(parseBufferSize int) *sync.Pool {
	if parseBufferSize <= 0 {
		parseBufferSize = parseDefaultWebSocketBufferSize
	}
	if !isCacheableWebSocketWriteBufferSize(parseBufferSize) {
		return &sync.Pool{}
	}
	parseCachedPool, isFoundPool := cacheWebSocketWriteBufferPools.Load(parseBufferSize)
	if isFoundPool {
		parsePool, parseOK := parseCachedPool.(*sync.Pool)
		if parseOK {
			return parsePool
		}
	}

	parsePool := &sync.Pool{}
	parseStoredPool, _ := cacheWebSocketWriteBufferPools.LoadOrStore(parseBufferSize, parsePool)
	parseTypedPool, parseOK := parseStoredPool.(*sync.Pool)
	if parseOK {
		return parseTypedPool
	}
	return parsePool
}

// isCacheableWebSocketWriteBufferSize reports whether a buffer size should use the global shared pool cache.
func isCacheableWebSocketWriteBufferSize(parseBufferSize int) bool {
	switch parseBufferSize {
	case parseDefaultWebSocketBufferSize, 8 * 1024, 16 * 1024, 32 * 1024, 64 * 1024:
		return true
	default:
		return false
	}
}
