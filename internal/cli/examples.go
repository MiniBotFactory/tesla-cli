package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

// commandExamples 把命令名映射到一组常用示例。
// 后续每实现一个真实命令,在这里追加示例。
var commandExamples = map[string][]string{
	"version": {
		"tesla version",
		"tesla version -o yaml",
		"tesla version -o json | jq -r .data.version",
	},
	"docs": {
		"tesla docs",
		"tesla docs auth-flow",
		"tesla docs virtual-key -o json",
	},
	"auth": {
		"tesla auth login --scopes 'openid offline_access vehicle_device_data vehicle_cmds'",
		"tesla auth status",
		"tesla auth partner register --domain my.app.com",
	},
	"key": {
		"tesla key generate --out ~/.config/tesla/keys",
		"tesla key publish --domain my.app.com",
		"tesla key pair-url --vin 5YJ...",
	},
	"vehicle": {
		"tesla vehicle list",
		"tesla vehicle data 5YJ... --jq '.charge_state.battery_level'",
		"tesla vehicle wake --vin 5YJ...",
	},
	"charge": {
		"tesla charge limit 5YJ... --percent 80",
		"tesla charge start --vin 5YJ...",
		"echo '{\"percent\":75}' | tesla charge limit 5YJ... --json-input",
	},
	"climate": {
		"tesla climate on --vin 5YJ...",
		"tesla climate set 5YJ... --driver 22 --passenger 22",
		"tesla climate seat 5YJ... --seat 0 --level 2",
	},
	"telemetry": {
		"tesla telemetry config set --vin 5YJ... --fields speed,location,soc --hz 1",
		"tesla telemetry stream --vin 5YJ... --ndjson | jq -c .",
	},
}

const examplesLong = `打印命令示例,按命令分组。

用法
  tesla examples              列出所有有示例的命令
  tesla examples <command>    打印某个命令的示例

示例
  tesla examples
  tesla examples charge
  tesla examples vehicle -o json | jq -r '.data.examples[]'`

func newExamplesCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "examples [command]",
		Short:         "打印命令示例",
		Long:          examplesLong,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			renderer := output.NewRenderer(format)

			if len(args) == 0 {
				return renderer.Render(cmd.OutOrStdout(),
					output.Success(map[string]any{"commands": sortedCommands()}, "", started))
			}
			name := args[0]
			items, ok := commandExamples[name]
			if !ok {
				return errs.New(errs.ExitUsage,
					fmt.Sprintf("no examples for %q (available: %s)",
						name, strings.Join(sortedCommands(), ", "))).
					WithHint("run `tesla examples` to list commands")
			}
			return renderer.Render(cmd.OutOrStdout(),
				output.Success(map[string]any{"command": name, "examples": items}, "", started))
		},
	}
}

func sortedCommands() []string {
	keys := make([]string, 0, len(commandExamples))
	for k := range commandExamples {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
