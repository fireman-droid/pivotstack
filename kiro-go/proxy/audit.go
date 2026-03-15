package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var auditMu sync.Mutex

// AuditLog appends an audit entry to data/audit.log.
func AuditLog(action, operator, detail string) {
	auditMu.Lock()
	defer auditMu.Unlock()

	dir := "data"
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
