# tesla-cli-cn

Tesla Fleet API 命令行工具 — **为 Agent 调用而生**。

- 默认 JSON 输出,stderr 仅日志,退出码语义化
- 标准 OAuth 2.0 + PKCE,虚拟钥匙 EC P-256 签名
- 支持北美 / 欧洲 / **中国大陆**三个区域(CN 端点已实测打通)
- 单二进制分发,冷启动 < 20ms

```
tesla
├── auth          login / logout / status / token / refresh / partner
├── key           generate / publish / pubkey / pair-url
├── config        init / set / get / path / show
├── vehicle       list / info / data / wake / lock / unlock / honk / flash
├── charge        start / stop / limit
├── energy        list / info / live
├── docs          离线长文档 (auth-flow / virtual-key / cn-notes / agent-recipes / errors)
├── examples      命令示例
├── completion    bash / zsh / fish / powershell
└── man           生成 man pages
```

完整命令树见 `tesla --help`。每条命令都有 `--help`,自带 scopes、退出码、3+ 示例。

---

## 安装

### 通过 npm(推荐 — 单条命令)

```bash
npm install -g tesla-cli-cn
tesla --help
```

postinstall 自动从 GitHub Releases 下你机器对应平台的二进制。
**国内用户**网络差时设镜像(任选其一):

```bash
TESLA_CLI_MIRROR=https://ghfast.top  npm install -g tesla-cli-cn
TESLA_CLI_MIRROR=https://gh-proxy.com npm install -g tesla-cli-cn
```

下载失败会自动重试 3 次(2s / 4s / 8s 退避)。

### 从源码编译

```bash
git clone git@github.com:MiniBotFactory/tesla-cli.git
cd tesla-cli
make build                       # 产物 bin/tesla
make install                     # 安装到 ~/.local/bin/tesla
```

依赖:Go 1.21+,标准库 + `cobra` + `viper` + `yaml.v3`,无 cgo。

---

## 快速上手(5 步,从 0 到调通真车)

> 前置条件:已在 [developer.tesla.com](https://developer.tesla.com)(全球)或 [developer.tesla.cn](https://developer.tesla.cn)(中国)创建 App,拿到 `client_id` / `client_secret`。

### 1. 生成虚拟钥匙

```bash
tesla key generate
# → ~/.config/tesla/keys/{private,public}-key.pem (mode 0600 / 0644)
```

### 2. 把公钥部署到你的域名

```bash
tesla key pubkey --quiet > public-key.pem
# 部署到:https://<your-domain>/.well-known/appspecific/com.tesla.3p.public-key.pem
# 必须 HTTPS 有效证书,自签名不被 Tesla 接受
```

### 3. 写配置

```bash
tesla config init \
  --client-id     '<YOUR-CLIENT-ID>' \
  --client-secret '<YOUR-CLIENT-SECRET>' \
  --domain        'your.example.com' \
  --region-init   'na' \
  --redirect-uri  'http://localhost:8765/callback' \
  --scopes        'openid offline_access vehicle_device_data vehicle_cmds vehicle_charging_cmds'
```

文件落到 `~/.config/tesla/config.toml`,**mode 0600**。

### 4. 注册域名 + 车主授权

```bash
tesla auth partner token                                # client_credentials,拿 partner JWT
tesla auth partner register --domain your.example.com   # Tesla 入账你的公钥

tesla auth login                                        # 浏览器跳转 → token 落到 profiles/default.json
tesla auth status                                       # 看 token 元数据(脱敏)
```

### 5. 调用 Fleet API

```bash
tesla vehicle list
# {"ok":true,"data":{"count":1,"vehicles":[{"vin":"LRW0000000000000","display_name":"My Car","state":"online",...}]}}

tesla vehicle data --vin LRW0000000000000 --jq '.charge_state.battery_level'
tesla energy list
```

---

## Agent 调用契约

| 项 | 规则 |
|---|---|
| stdout | 仅业务数据(JSON / 一行 ID / NDJSON 流) |
| stderr | 日志、提示、警告 |
| 输出格式 | `--output json` 默认,可选 `yaml / table / text` |
| 退出码 | 语义化(见下) |
| 失败信封 | `{ok:false, code:"AUTH", message, hint, retryable}` |
| 输入优先级 | flag > `TESLA_*` env > `~/.config/tesla/config.toml` > 内置默认 |
| stdin | `--json-input` 从 stdin 读 JSON 当入参 |

### 退出码语义

| 码 | 含义 | 字符串 code |
|----|---|---|
| 0  | 成功 | `OK` |
| 1  | 通用错误 | `GENERIC` |
| 2  | 参数 / 用法错误 | `USAGE` |
| 3  | 配置错误 | `CONFIG` |
| 4  | 鉴权失败 / token 过期 | `AUTH` |
| 5  | OAuth scope 不足 | `SCOPE` |
| 6  | 虚拟钥匙未配对 / 签名失败 | `VIRTUAL_KEY` |
| 7  | 车辆离线 / 唤醒失败 | `VEHICLE_STATE` |
| 8  | Tesla 5xx | `UPSTREAM_5XX` |
| 9  | 超时 | `TIMEOUT` |
| 10 | 触发限流(已重试耗尽) | `RATE_LIMIT` |

### 三个 Agent 调用范式

```bash
# A. 链式 + jq 提取
tesla charge limit --percent 80 \
  && tesla charge start \
  && tesla vehicle data --jq '.charge_state.battery_level'

# B. stdin 注入 JSON 参数
echo '{"address":"1 Hacker Way, Menlo Park"}' \
  | tesla drive nav --json-input

# C. 流式 NDJSON(M5)
timeout 30 tesla telemetry stream --vin <VIN> \
  --fields location,speed --hz 1 --ndjson \
  | jq -c '{t:.timestamp, lat:.location.lat, lon:.location.lon}'
```

---

## 文件布局

```
~/.config/tesla/
├── config.toml                 客户端配置(0600)
├── profiles/
│   ├── default.json            车主 token(0600,原子写入)
│   └── default-partner.json    Partner Token
├── keys/
│   ├── private-key.pem         EC P-256(0600)
│   └── public-key.pem          (0644)
└── cache/                      M3+ 只读 API 缓存
```

`XDG_CONFIG_HOME` 受尊重;CI 用 `--token-file <path>` 跳过 keyring。

---

## Tesla 中国大陆账号特别说明

跑 `tesla docs cn-notes` 看完整差异(简版):

- 注册在 [developer.tesla.cn](https://developer.tesla.cn)(单独账号,需 +86 手机号),与全球账号不互通
- 端点:`auth.tesla.cn` + `fleet-api.prd.cn.vn.cloud.tesla.cn`(本 CLI 已配好,`--region cn` 即可)
- `auth.tesla.cn` 由 Akamai WAF 守护,UA 含 `(+https://...)` 会 403 — 本 CLI 已规避
- redirect_uri 通常是你的生产域名(非 localhost),CLI 自动切 **manual paste 模式**

---

## 测试与覆盖率

```bash
go test -race -cover ./...
```

| 包 | 覆盖率 |
|---|---|
| `internal/meta` | 100% |
| `internal/tesla` | 100% |
| `internal/config` | 97% |
| `internal/errs` | 96% |
| `internal/output` | 87% |
| `internal/keys` | 81% |
| `internal/client` | 65% |
| `internal/auth` | 70% |

业务核心 6 包平均 ~94%。`auth` 包剩余未覆盖行集中在 `Login` 的 `net.Listen + http.Server` loopback 路径与 `RegisterPartner/Refresh` 的真实远端调用 — 需要集成测试覆盖。manual-paste 路径与 `parsePastedCallback` 已通过 `manual_test.go` 全覆盖。

Fixture 来自真实 Tesla CN 响应(脱敏),回放在 `internal/{auth,client}/testdata/`。`TestFixturesDoNotLeakSecrets` 静态扫描守住 fixture 永不退化回真车。

---

## 项目结构

```
.
├── cmd/tesla/main.go                CLI 入口(11 行)
├── internal/
│   ├── meta/                        版本元数据(ldflags 注入)
│   ├── errs/                        退出码契约 + Error 类型
│   ├── output/                      Envelope + JSON/YAML/Table/Text
│   ├── config/                      viper 包装 + XDG 路径
│   ├── tesla/                       区域端点表 + HTTP client
│   ├── auth/                        OAuth + PKCE + Token + Partner
│   ├── keys/                        EC P-256 PEM + Tesla App 配对深链
│   ├── client/                      Fleet API REST 封装
│   └── cli/                         cobra 根 + 全部子命令
├── go.mod / go.sum
├── Makefile
└── .gitignore
```

`go vet ./...` 干净,`gofmt` 自动应用。

---

## 开发流程

```bash
make build         # 编译
make test          # go test ./...
make lint          # go vet + gofmt 校验
make completions   # 生成 bash/zsh/fish/powershell 补全
make manpages      # 生成 man pages
```

提交规范:`type: short description`(`feat`/`fix`/`refactor`/`test`/`docs`/`chore`/`perf`/`ci`)。

---

## License

MIT — 详见 `LICENSE`。
