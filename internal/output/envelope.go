// Package output 提供 Agent 友好的统一输出契约。
//
// 所有命令的成功/失败响应都包装在 Envelope 中,便于 Agent 用 jq/yq 解析。
package output

import (
	"time"

	"github.com/wmango/tesla-cli/internal/errs"
)

// Envelope 是所有 CLI 输出的统一信封。
//
// 成功:Ok=true,Data 非空。
// 失败:Ok=false,Code/Message/Hint 填充,Data 为 nil。
type Envelope struct {
	Ok         bool        `json:"ok"                      yaml:"ok"`
	Data       interface{} `json:"data,omitempty"          yaml:"data,omitempty"`
	Code       string      `json:"code,omitempty"          yaml:"code,omitempty"`
	Message    string      `json:"message,omitempty"       yaml:"message,omitempty"`
	Hint       string      `json:"hint,omitempty"          yaml:"hint,omitempty"`
	Retryable  bool        `json:"retryable,omitempty"     yaml:"retryable,omitempty"`
	RequestID  string      `json:"request_id,omitempty"    yaml:"request_id,omitempty"`
	DurationMS int64       `json:"duration_ms,omitempty"   yaml:"duration_ms,omitempty"`
}

// Success 构造一个成功信封。data 可以为 nil(纯动作命令如 lock)。
func Success(data interface{}, requestID string, started time.Time) Envelope {
	return Envelope{
		Ok:         true,
		Data:       data,
		RequestID:  requestID,
		DurationMS: time.Since(started).Milliseconds(),
	}
}

// Failure 从 *errs.Error 构造失败信封。
func Failure(e *errs.Error, requestID string, started time.Time) Envelope {
	if e == nil {
		return Envelope{Ok: false, Code: codeName(errs.ExitGeneric), Message: "unknown error"}
	}
	return Envelope{
		Ok:         false,
		Code:       codeName(e.Code),
		Message:    e.Error(),
		Hint:       e.Hint,
		Retryable:  e.Retryable,
		RequestID:  requestID,
		DurationMS: time.Since(started).Milliseconds(),
	}
}

// codeName 把退出码映射成稳定的字符串,Agent 用字符串匹配比数字更易读。
func codeName(c errs.ExitCode) string {
	switch c {
	case errs.ExitOK:
		return "OK"
	case errs.ExitUsage:
		return "USAGE"
	case errs.ExitConfig:
		return "CONFIG"
	case errs.ExitAuth:
		return "AUTH"
	case errs.ExitScope:
		return "SCOPE"
	case errs.ExitVirtualKey:
		return "VIRTUAL_KEY"
	case errs.ExitVehicleState:
		return "VEHICLE_STATE"
	case errs.ExitUpstream5xx:
		return "UPSTREAM_5XX"
	case errs.ExitTimeout:
		return "TIMEOUT"
	case errs.ExitRateLimit:
		return "RATE_LIMIT"
	default:
		return "GENERIC"
	}
}
