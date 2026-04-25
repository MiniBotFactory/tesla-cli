package output

import (
	"encoding/json"
	"io"
)

type jsonRenderer struct{}

// Render 把 Envelope 编码为缩进的 JSON。
// Agent 通常用 jq 解析,加换行符让 NDJSON 拼接更安全。
func (jsonRenderer) Render(w io.Writer, env Envelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}
