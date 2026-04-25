package cli

import (
	"github.com/spf13/cobra"

	"github.com/wmango/tesla-cli/internal/errs"
)

const completionLong = `生成 shell 自动补全脚本。

支持的 shell:bash | zsh | fish | powershell

安装示例(zsh)
  tesla completion zsh > "${fpath[1]}/_tesla"

安装示例(bash, macOS + brew)
  tesla completion bash > $(brew --prefix)/etc/bash_completion.d/tesla

安装示例(fish)
  tesla completion fish > ~/.config/fish/completions/tesla.fish`

func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:                   "completion {bash|zsh|fish|powershell}",
		Short:                 "生成 shell 补全脚本",
		Long:                  completionLong,
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		SilenceErrors:         true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			default:
				return errs.New(errs.ExitUsage, "unsupported shell: "+args[0])
			}
		},
	}
}
