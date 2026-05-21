package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	utls "github.com/refraction-networking/utls"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// 双端点配置（429 时自动 fallback）
type kiroEndpoint struct {
	URL       string
	Origin    string
	AmzTarget string
	Name      string
}

var kiroEndpoints = []kiroEndpoint{
	{
		URL:       "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "AmazonCodeWhispererStreamingService.GenerateAssistantResponse",
		Name:      "CodeWhisperer",
	},
}

// makeUTLSDialer 创建 UTLS Chrome 指纹 TLS 拨号器
// 使用 Chrome 的 TLS 指纹而非 Go 标准库指纹，防止 AWS 识别为非标准客户端
func makeUTLSDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}

		var netConn net.Conn
		if proxyURL := os.Getenv("VPN_PROXY_URL"); proxyURL != "" {
			p, err := url.Parse(proxyURL)
			if err != nil {
				return nil, err
			}
			// 通过代理建立 TCP 连接
			d := &net.Dialer{Timeout: 30 * time.Second}
			netConn, err = d.DialContext(ctx, "tcp", net.JoinHostPort(p.Hostname(), p.Port()))
			if err != nil {
				return nil, err
			}
			// CONNECT 隧道
			connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr)
			_, err = netConn.Write([]byte(connectReq))
			if err != nil {
				netConn.Close()
				return nil, err
			}
			// 读取代理响应（简单方式）
			buf := make([]byte, 4096)
			n, err := netConn.Read(buf)
			if err != nil || !strings.Contains(string(buf[:n]), "200") {
				netConn.Close()
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("proxy CONNECT failed: %s", string(buf[:n]))
			}
		} else {
			d := &net.Dialer{Timeout: 30 * time.Second}
			netConn, err = d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
		}

		// UTLS Chrome 指纹 + 强制 HTTP/1.1
		spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
		if err != nil {
			netConn.Close()
			return nil, err
		}
		for _, ext := range spec.Extensions {
			if alpn, ok := ext.(*utls.ALPNExtension); ok {
				alpn.AlpnProtocols = []string{"http/1.1"}
				break
			}
		}
		tlsConn := utls.UClient(netConn, &utls.Config{ServerName: host}, utls.HelloCustom)
		if err := tlsConn.ApplyPreset(&spec); err != nil {
			netConn.Close()
			return nil, err
		}
		if err := tlsConn.Handshake(); err != nil {
			netConn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
}

// 全局 HTTP 客户端，UTLS Chrome 指纹 + 支持 VPN_PROXY_URL 代理
var kiroHttpClient = func() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   false,                                                  // UTLS 强制 HTTP/1.1
		TLSNextProto:        map[string]func(string, *tls.Conn) http.RoundTripper{}, // 禁用 Go 内置 HTTP/2
		DialTLSContext:      makeUTLSDialer(),
	}
	// 注意：代理已在 UTLS dialer 中处理，不再设置 transport.Proxy
	return &http.Client{Timeout: 0, Transport: transport} // Timeout=0: 流式响应不设超时
}()

// getSortedEndpoints 根据首选端点配置排序端点列表
func getSortedEndpoints(preferred string) []kiroEndpoint {
	if len(kiroEndpoints) <= 1 {
		return kiroEndpoints
	}
	if preferred == "amazonq" {
		return []kiroEndpoint{kiroEndpoints[1], kiroEndpoints[0]}
	}
	if preferred == "codewhisperer" {
		return []kiroEndpoint{kiroEndpoints[0], kiroEndpoints[1]}
	}
	// "auto" 或空值：默认顺序
	return []kiroEndpoint{kiroEndpoints[0], kiroEndpoints[1]}
}

// apiDebugLog 包级 debug 日志函数，记录到 data/debug.log
func apiDebugLog(format string, args ...interface{}) {
	if os.Getenv("DEBUG_REQUESTS") != "true" {
		return
	}
	f, _ := os.OpenFile("data/debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, format+"\n", args...)
		f.Close()
	}
}
