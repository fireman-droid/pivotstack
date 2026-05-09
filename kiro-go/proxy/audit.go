package proxy

import (
	"fmt"
	"kiro-api-proxy/config"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var auditMu sync.Mutex

// AuditLog appends an audit entry to <dataDir>/audit.log.
// 用 config.GetDataDir() 跟 call_logs / recharge_records 同口径，
// 本地实例用独立 data_local/ 不会污染生产 data/。
func AuditLog(action, operator, detail string) {
	auditMu.Lock()
	defer auditMu.Unlock()

	dir := config.GetDataDir()
	if dir == "" {
		dir = "data"
	}
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[AuditLog] ERROR: %v\n", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] action=%s operator=%s %s\n", ts, action, operator, detail)
	f.WriteString(line)
}
