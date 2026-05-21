package proxy

import (
	"net/http"
	"strings"
)

// operatorFromRequest 提取操作人标识（用于 audit log）
func operatorFromRequest(r *http.Request) string {
	if u := r.Header.Get("X-Admin-User"); u != "" {
		return u
	}
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}
	return "admin@" + ip
}
