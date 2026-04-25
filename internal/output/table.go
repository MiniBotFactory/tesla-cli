package output

import (
	"fmt"
	"io"
)

// tableRenderer M1 简化实现:直接退化为 text,后续接入 lipgloss/table 渲染。
type tableRenderer struct{}

// Render 当前与 textRenderer 行为一致;保持接口不变。
func (tableRenderer) Render(w io.Writer, env Envelope) error {
	return textRenderer{}.Render(w, env)
}

// textRenderer 把 Envelope 渲染为人类可读的紧凑文本。
type textRenderer struct{}

// Render 输出形如:
//
//	ok
//	data: {...}
//
// 失败时输出 message + hint。
func (textRenderer) Render(w io.Writer, env Envelope) error {
	if env.Ok {
		if _, err := fmt.Fprintf(w, "ok\n"); err != nil {
			return err
		}
		if env.Data != nil {
			if _, err := fmt.Fprintf(w, "data: %v\n", env.Data); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintf(w, "error [%s]: %s\n", env.Code, env.Message); err != nil {
		return err
	}
	if env.Hint != "" {
		if _, err := fmt.Fprintf(w, "hint: %s\n", env.Hint); err != nil {
			return err
		}
	}
	return nil
}
