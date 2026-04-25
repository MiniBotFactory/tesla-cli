package output

import (
	"io"

	"gopkg.in/yaml.v3"
)

type yamlRenderer struct{}

// Render 把 Envelope 编码为 YAML(2 空格缩进)。
func (yamlRenderer) Render(w io.Writer, env Envelope) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(env)
}
