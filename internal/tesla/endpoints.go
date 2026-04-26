// Package tesla 提供 Tesla Fleet API 的通用基础设施:
//   - 区域端点表
//   - HTTP 客户端(超时 / 重试 / UA)
//
// 该包不感知具体业务,被 auth、command、telemetry 等子包共用。
package tesla

import (
	"fmt"
	"strings"
)

// Endpoints 是某个 region 下的 Tesla 端点集合。
type Endpoints struct {
	Region       string
	AuthorizeURL string // 浏览器跳转的 OAuth authorize URL
	TokenURL     string // 服务端换 token / 刷新 token
	APIBase      string // Fleet API 根
	OIDCMetadata string // OIDC 配置(自检用)
}

// 已知端点表。来源:Tesla developer docs, fleet-auth metadata。
//
// auth.tesla.com 是车主登录页(全球统一),
// fleet-auth.prd.vn.cloud.tesla.com 是第三方 OAuth 服务器(全球统一)。
// fleet-api 才按区域分。
var endpointsByRegion = map[string]Endpoints{
	"na": {
		Region:       "na",
		AuthorizeURL: "https://auth.tesla.com/oauth2/v3/authorize",
		TokenURL:     "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token",
		APIBase:      "https://fleet-api.prd.na.vn.cloud.tesla.com",
		OIDCMetadata: "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/thirdparty/.well-known/openid-configuration",
	},
	"eu": {
		Region:       "eu",
		AuthorizeURL: "https://auth.tesla.com/oauth2/v3/authorize",
		TokenURL:     "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token",
		APIBase:      "https://fleet-api.prd.eu.vn.cloud.tesla.com",
		OIDCMetadata: "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/thirdparty/.well-known/openid-configuration",
	},
	// CN 区域:authorize / token 都在 auth.tesla.cn 主域(不走 fleet-auth 子域),
	// OIDC discovery 在标准 .well-known 路径下。
	// 实测 2026-04-26:client_credentials grant 已工作。
	"cn": {
		Region:       "cn",
		AuthorizeURL: "https://auth.tesla.cn/oauth2/v3/authorize",
		TokenURL:     "https://auth.tesla.cn/oauth2/v3/token",
		APIBase:      "https://fleet-api.prd.cn.vn.cloud.tesla.cn",
		OIDCMetadata: "https://auth.tesla.cn/oauth2/v3/.well-known/openid-configuration",
	},
}

// EndpointsFor 按 region 返回端点集合。region 不区分大小写。
// 未知 region 返回 error。
func EndpointsFor(region string) (Endpoints, error) {
	r := strings.ToLower(strings.TrimSpace(region))
	if r == "" {
		r = "na"
	}
	ep, ok := endpointsByRegion[r]
	if !ok {
		return Endpoints{}, fmt.Errorf("unknown region %q (allowed: na | eu | cn)", region)
	}
	return ep, nil
}

// AllRegions 返回支持的 region 列表(供 help 文本与校验使用)。
func AllRegions() []string {
	return []string{"na", "eu", "cn"}
}

// PairDeepLink 构造把虚拟钥匙加到车上的 Tesla App 深链。
// 由 keys 子包消费;放在 tesla 包是因为格式与官方约定耦合,
// 未来可能按 region 切换域名。
func PairDeepLink(domain string) string {
	return "https://tesla.com/_ak/" + domain
}
