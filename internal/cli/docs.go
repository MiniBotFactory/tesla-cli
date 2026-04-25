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

// docTopics 是离线文档主题表。后续 M2+ 用 //go:embed 嵌入完整 markdown。
// 此处先以短摘要占位,保证 M1 即可使用 `tesla docs <topic>`。
var docTopics = map[string]string{
	"auth-flow": `Tesla Fleet API 认证流程:
  1. config init 配置 client_id / client_secret / domain
  2. auth partner register 注册域名(上传公钥)
  3. auth login 走 OAuth authorization_code 流程获取车主 token
  4. key pair-url 让车主把虚拟钥匙加到车上
  5. 之后 vehicle/charge/climate 子命令均可使用`,

	"virtual-key": `虚拟钥匙(Virtual Key)说明:
  • EC P-256 (secp256r1/prime256v1) 公私钥对
  • 公钥托管于 https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem
  • 车主在 Tesla App 中点击深链,把公钥作为虚拟钥匙加到车上
  • 之后所有写命令必须经 Vehicle Command Proxy 用私钥签名
  • Model S/X 2021 前老车不支持,走旧 REST 路径`,

	"telemetry": `Fleet Telemetry(实时遥测):
  • 走 WebSocket / Protobuf,需先用 telemetry config set 配置字段与频率
  • 字段示例:speed / location / soc / charge_state / drive_state
  • 频率上限通常 10 Hz,Tesla 服务端会节流
  • CLI 用 tesla telemetry stream --ndjson 输出 NDJSON 给 Agent`,

	"agent-recipes": `Agent 调用范式:
  • 所有命令默认 --output json,失败信封含 code/hint/retryable
  • 退出码语义见 tesla docs errors
  • 长流式数据用 NDJSON,逐行 jq -c 解析
  • 写命令幂等:--idempotency-key,本地 24h 去重
  • 高频自省:tesla <cmd> --help -o json 拿结构化 flag schema`,

	"errors": `退出码契约:
  0  成功
  1  通用错误
  2  参数 / 用法错误
  3  配置错误
  4  鉴权失败
  5  scope 不足
  6  虚拟钥匙未配对
  7  车辆离线 / 唤醒失败
  8  Tesla 5xx
  9  超时
  10 限流`,
}

const docsLong = `离线查看长文档主题。

可用主题
  auth-flow      OAuth + Partner 注册全流程
  virtual-key    虚拟钥匙生成 / 部署 / 配对
  telemetry      实时遥测配置与流式订阅
  agent-recipes  Agent 调用范式与契约
  errors         退出码语义表

示例
  tesla docs auth-flow
  tesla docs virtual-key -o json`

func newDocsCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "docs <topic>",
		Short:         "离线长文档查询",
		Long:          docsLong,
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
					output.Success(map[string]any{"topics": sortedTopics()}, "", started))
			}
			topic := args[0]
			body, ok := docTopics[topic]
			if !ok {
				return errs.New(errs.ExitUsage,
					fmt.Sprintf("unknown topic %q (available: %s)",
						topic, strings.Join(sortedTopics(), ", "))).
					WithHint("run `tesla docs` without args to list topics")
			}
			return renderer.Render(cmd.OutOrStdout(),
				output.Success(map[string]any{"topic": topic, "body": body}, "", started))
		},
	}
}

func sortedTopics() []string {
	keys := make([]string, 0, len(docTopics))
	for k := range docTopics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
