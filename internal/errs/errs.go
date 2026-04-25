// Package errs 定义 CLI 的退出码契约和错误类型。
//
// Agent 调用约定:
//   - stdout 仅业务数据
//   - stderr 仅日志/警告
//   - 退出码语义化(见 ExitCode 常量)
package errs

import "fmt"

// ExitCode 是 CLI 退出码语义。0 = 成功;非 0 = 失败的语义化分类。
type ExitCode int

const (
	ExitOK           ExitCode = 0  // 成功
	ExitGeneric      ExitCode = 1  // 通用错误(未分类)
	ExitUsage        ExitCode = 2  // 参数 / 用法错误
	ExitConfig       ExitCode = 3  // 配置错误(缺 client_id 等)
	ExitAuth         ExitCode = 4  // 鉴权失败 / token 过期
	ExitScope        ExitCode = 5  // OAuth scope 不足
	ExitVirtualKey   ExitCode = 6  // 虚拟钥匙未配对 / 签名失败
	ExitVehicleState ExitCode = 7  // 车辆离线 / 唤醒失败
	ExitUpstream5xx  ExitCode = 8  // Tesla 服务端 5xx
	ExitTimeout      ExitCode = 9  // 请求超时
	ExitRateLimit    ExitCode = 10 // 触发限流(已重试耗尽)
)

// Error 是携带退出码的 CLI 错误。所有内部错误应包装为 Error,
// 以便顶层入口能正确决定退出码。
type Error struct {
	Code      ExitCode
	Message   string
	Hint      string // 给用户/Agent 的恢复建议
	Retryable bool
	Cause     error
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap 支持 errors.Is / errors.As。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New 创建一个新的 *Error。保持不可变:返回新对象,不修改入参。
func New(code ExitCode, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Wrap 用退出码语义包装底层错误。
func Wrap(code ExitCode, msg string, cause error) *Error {
	return &Error{Code: code, Message: msg, Cause: cause}
}

// WithHint 返回带恢复建议的新副本(不可变模式)。
func (e *Error) WithHint(hint string) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Hint = hint
	return &cp
}

// WithRetryable 返回标记为可重试的新副本。
func (e *Error) WithRetryable(retryable bool) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Retryable = retryable
	return &cp
}

// CodeOf 从任意 error 中抽取退出码;非 *Error 返回 ExitGeneric。
// nil 返回 ExitOK。
func CodeOf(err error) ExitCode {
	if err == nil {
		return ExitOK
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ExitGeneric
}
