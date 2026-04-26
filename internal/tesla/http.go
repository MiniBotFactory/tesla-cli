package tesla

import (
	"net/http"
	"time"
)

// UserAgent 是所有出站请求的统一 UA。
//
// 故意保持简短:Akamai WAF(Tesla CN auth.tesla.cn 用)对含
// "(+https://...)" 形式的 UA 直接 403 Access Denied。已实测:
//
//	"tesla-cli/0.0"                       → 200 ✓
//	"tesla-cli/0.0 (+https://github...)"  → 403 Akamai Access Denied
const UserAgent = "tesla-cli/0.0"

// NewHTTPClient 返回带超时的 *http.Client。
// 重试逻辑下沉到调用方(因不同端点的可重试错误集合不同)。
func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// SetUA 在请求上设置统一 UA;调用方在发送前调用。
func SetUA(req *http.Request) {
	if req == nil {
		return
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
}
