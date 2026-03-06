// Package auth 提供认证相关功能的 HTTP 客户端
package auth

import (
	"net/http"
	"net/url"
	"os"
	"time"
)

// 全局 HTTP 客户端，复用连接池，支持 VPN_PROXY_URL 代理
// 用于所有 auth 模块的 HTTP 请求
var httpClient = func() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}
	if proxyURL := os.Getenv("VPN_PROXY_URL"); proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}()
