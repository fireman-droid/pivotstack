// Package auth 提供认证相关功能的 HTTP 客户端
package auth

import (
	"net/http"
	"time"
)

// 全局 HTTP 客户端，复用连接池
// auth 请求直连 AWS（不走 VPN 代理），避免代理阻塞 OIDC 端点
var httpClient = func() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   false,
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		// 防止 POST 在重定向时被改为 GET（导致 405 Method Not Allowed）
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				req.Method = via[0].Method
				req.Body = via[0].Body
				req.GetBody = via[0].GetBody
				req.ContentLength = via[0].ContentLength
				for key, val := range via[0].Header {
					req.Header[key] = val
				}
			}
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}()
