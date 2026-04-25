package output

import (
	"fmt"
	"io"

	"github.com/wmango/tesla-cli/internal/errs"
)

// Format 是支持的输出格式。
type Format string

const (
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatTable Format = "table"
	FormatText  Format = "text"
)

// AllFormats 列出全部支持的格式;help 文本与校验都引用它。
func AllFormats() []Format {
	return []Format{FormatJSON, FormatYAML, FormatTable, FormatText}
}

// ParseFormat 解析字符串为 Format,无效输入返回 USAGE 错误。
func ParseFormat(s string) (Format, error) {
	for _, f := range AllFormats() {
		if string(f) == s {
			return f, nil
		}
	}
	return "", errs.New(errs.ExitUsage,
		fmt.Sprintf("invalid output format %q (allowed: json|yaml|table|text)", s))
}

// Renderer 抽象出"把 Envelope 写到 Writer"。
type Renderer interface {
	Render(w io.Writer, env Envelope) error
}

// NewRenderer 按 Format 派发到具体实现。
func NewRenderer(f Format) Renderer {
	switch f {
	case FormatYAML:
		return yamlRenderer{}
	case FormatTable:
		return tableRenderer{}
	case FormatText:
		return textRenderer{}
	default:
		return jsonRenderer{} // 默认 JSON,Agent 友好
	}
}
