// Package cli 组装 cobra 根命令与全部子命令。
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

const longDescription = `tesla — Tesla Fleet API 命令行工具(Agent 友好)

设计要点
  • 默认 JSON 输出,stderr 仅日志,退出码语义化
  • OAuth 第三方令牌 + 虚拟钥匙签名(Vehicle Command)
  • 支持 --profile 多账号、--dry-run、--explain、--json-input

常用流程
  1) tesla config init                 配置 client_id / domain
  2) tesla key generate                生成虚拟钥匙密钥对
  3) tesla auth partner register       注册合作伙伴域名
  4) tesla auth login                  车主 OAuth 授权
  5) tesla key pair-url --vin <vin>    把虚拟钥匙加到车上
  6) tesla vehicle list / charge limit / climate set ...

更多
  tesla docs <topic>      离线文档(auth-flow / virtual-key / agent-recipes)
  tesla examples <cmd>    打印命令示例
  tesla <cmd> --help-full 完整 help(含 scopes / 退出码 / 多个示例)`

// NewRootCommand 构造根命令。所有子命令由本函数注册。
//
// 不在此处启动副作用(网络/读写文件);副作用应在子命令的 RunE 中进行。
func NewRootCommand() *cobra.Command {
	v := viper.New()
	v.SetEnvPrefix(config.EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	root := &cobra.Command{
		Use:           "tesla",
		Short:         "Tesla Fleet API 命令行工具",
		Long:          longDescription,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	registerGlobalFlags(root, v)
	root.AddCommand(
		newVersionCommand(v),
		newDocsCommand(v),
		newExamplesCommand(v),
		newCompletionCommand(),
		newManCommand(),
	)
	return root
}

// registerGlobalFlags 注册所有命令共享的持久 flags。
func registerGlobalFlags(root *cobra.Command, v *viper.Viper) {
	pf := root.PersistentFlags()

	pf.String("profile", "default", "凭据 profile 名(支持多账号切换)")
	pf.String("region", "na", "Tesla API 区域:na | eu | cn")
	pf.StringP("output", "o", "json", "输出格式:json | yaml | table | text")
	pf.String("jq", "", "对输出再走一遍 jq 表达式(嵌入,免外部依赖)")
	pf.Bool("raw", false, "不做后处理,原样回显 Tesla API 响应")
	pf.BoolP("quiet", "q", false, "静默模式,仅打印关键值")
	pf.CountP("verbose", "v", "详细日志,可叠加(-v / -vv)")
	pf.Bool("no-color", false, "禁用彩色输出")
	pf.String("timeout", "30s", "请求超时时长(Go duration 格式)")
	pf.Int("retry", 3, "5xx/429 自动重试次数")
	pf.Int("rate-limit", 0, "每秒最大请求数,0 表示不限")
	pf.Bool("dry-run", false, "仅打印将要发送的 HTTP 请求,不实际发送")
	pf.String("config", "", "覆盖配置文件路径")
	pf.String("token-file", "", "从文件加载 token(CI / 无 keyring 环境)")
	pf.String("vin", "", "全局默认 VIN(免每条命令重复指定)")
	pf.Bool("json-input", false, "从 stdin 读 JSON 作为入参")
	pf.Bool("explain", false, "先以 JSON 输出'我将做什么 / scopes / 风险',再询问执行")
	pf.Bool("help-full", false, "显示完整 help(含 scopes / 退出码 / 多个示例)")

	// 将所有持久 flag 同步到 viper,便于 env 覆盖。
	if err := v.BindPFlags(pf); err != nil {
		// BindPFlags 在正常情况下不会失败;一旦失败说明编译时 flag 名重复。
		fmt.Fprintf(os.Stderr, "tesla: bind flags: %v\n", err)
		os.Exit(int(errs.ExitGeneric))
	}
}

// Execute 是程序入口的统一封装:运行根命令、按错误码退出。
func Execute() {
	root := NewRootCommand()
	started := time.Now()

	if err := root.Execute(); err != nil {
		writeFailure(root, err, started)
		os.Exit(int(errs.CodeOf(err)))
	}
}

// writeFailure 在错误路径上,把失败信封写到 stdout(保持 Agent 契约)。
func writeFailure(root *cobra.Command, err error, started time.Time) {
	v := viper.GetViper()
	cfg := config.DefaultConfig().BindViper(v)
	format, perr := output.ParseFormat(cfg.Output)
	if perr != nil {
		format = output.FormatJSON
	}
	var ee *errs.Error
	if e, ok := err.(*errs.Error); ok {
		ee = e
	} else {
		ee = errs.Wrap(errs.ExitGeneric, "command failed", err)
	}
	env := output.Failure(ee, "", started)
	if rerr := output.NewRenderer(format).Render(root.OutOrStdout(), env); rerr != nil {
		fmt.Fprintf(os.Stderr, "tesla: render failure: %v\n", rerr)
	}
}
