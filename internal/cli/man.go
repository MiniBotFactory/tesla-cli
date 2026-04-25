package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/wmango/tesla-cli/internal/errs"
)

const manLong = `生成 man pages 到指定目录。

用法
  tesla man --out ./manpages

之后可:
  sudo cp ./manpages/tesla.1 /usr/local/share/man/man1/
  man tesla`

func newManCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "man",
		Short:         "生成 man pages",
		Long:          manLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, _ := cmd.Flags().GetString("out")
			if out == "" {
				return errs.New(errs.ExitUsage, "missing --out").
					WithHint("example: tesla man --out ./manpages")
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return errs.Wrap(errs.ExitGeneric, "create out dir", err)
			}
			header := &doc.GenManHeader{Title: "TESLA", Section: "1"}
			absOut, err := filepath.Abs(out)
			if err != nil {
				return errs.Wrap(errs.ExitGeneric, "abs path", err)
			}
			return doc.GenManTree(cmd.Root(), header, absOut)
		},
	}
	cmd.Flags().String("out", "", "输出目录")
	return cmd
}
