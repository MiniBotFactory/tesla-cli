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

	"cn-notes": `Tesla 中国大陆账号联调差异(实测 2026-04 通过):

1. 开发者门户分开
   • 全球:https://developer.tesla.com
   • 中国:https://developer.tesla.cn(单独账号,需 +86 手机号)
   两边的 client_id 互不通用。在 .com 注册的 client 用 cn 端点会
   "client_not_found"。

2. OAuth 端点不在 fleet-auth 子域,直接在 auth.tesla.cn 主域:
     authorize: https://auth.tesla.cn/oauth2/v3/authorize
     token:     https://auth.tesla.cn/oauth2/v3/token
     api base:  https://fleet-api.prd.cn.vn.cloud.tesla.cn
     OIDC:      https://auth.tesla.cn/oauth2/v3/.well-known/openid-configuration
   猜测过的 fleet-auth.prd.vn.cloud.tesla.cn 不存在(SSL 直拒)。

3. Akamai WAF 反爬
   auth.tesla.cn 前面是 Akamai。它对 User-Agent 含 "(+https://...)" 形态
   的请求直接 403 Access Denied(返回 errors.edgesuite.net)。本 CLI 已把
   UA 改为简短 "tesla-cli/0.0"。如要换 UA,避开类爬虫指纹。

4. client_credentials grant 的 scope
   CN client 用 client_credentials 拿 partner token 时,响应里 scope 字段
   缺失,CLI 的 Token.Scopes 会是 nil(预期行为)。openid 单 scope 即可。

5. authorization_code grant
   redirect_uri 通常是生产域名(例如 https://your-domain.cn/code),不是
   localhost。CLI 自动切到 manual paste 模式:把浏览器跳到的完整 URL
   或 "code state" 字符串粘到 stdin。

6. partner_accounts/public_key 验证(verify)报 "missing scopes"
   CN region 这个端点要求额外 scope,目前不影响主流程。register 成功
   后 response 已含 public_key_hash,等同确认入账。

7. 命令调整
   • --region cn(也可写在 config.toml,推荐)
   • domain 字段填 host(不带 https://),如 your.example.cn
   • 公钥仍部署到 https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem`,
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
