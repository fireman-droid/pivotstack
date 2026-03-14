package proxy

import (
	"crypto/rand"
	"encoding/hex"
)

// generateRequestID 生成唯一请求 ID
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
