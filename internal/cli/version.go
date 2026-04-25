package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/meta"
	"github.com/wmango/tesla-cli/internal/output"
)

const versionLong = `打印 tesla CLI 的版本元数据。

输出包含:
  version     语义化版本(由 ldflags 注入)
  commit      git commit short SHA
  build_date  ISO-8601 构建时间

退出码
  0  始终成功

示例
  tesla version
  tesla version -o yaml
  tesla version -o json | jq -r .data.version`

func newVersionCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "打印 CLI 版本信息",
		Long:          versionLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			env := output.Success(meta.Snapshot(), "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}
