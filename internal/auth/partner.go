package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wmango/tesla-cli/internal/tesla"
)

// PartnerOptions 是 client_credentials grant 所需输入。
type PartnerOptions struct {
	Region       string
	ClientID     string
	ClientSecret string   // client_credentials 必需
	Scopes       []string // 至少 openid + 业务 scope
	Audience     string   // 通常是 Fleet API base
}

// PartnerToken 用 client_credentials grant 拿合作伙伴令牌。
//
// 用途:注册域名 / 查询自家配置 / 上传公钥校验,
// 不能用于车主车辆相关操作。
func PartnerToken(ctx context.Context, opts PartnerOptions) (*Token, error) {
	if opts.ClientID == "" {
		return nil, errors.New("partner: ClientID required")
	}
	if opts.ClientSecret == "" {
		return nil, errors.New("partner: ClientSecret required")
	}
	ep, err := tesla.EndpointsFor(opts.Region)
	if err != nil {
		return nil, fmt.Errorf("partner: %w", err)
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", opts.ClientID)
	form.Set("client_secret", opts.ClientSecret)
	if len(opts.Scopes) > 0 {
		form.Set("scope", strings.Join(opts.Scopes, " "))
	}
	if opts.Audience != "" {
		form.Set("audience", opts.Audience)
	}
	return postToken(ctx, ep.TokenURL, form)
}

// RegisterPartner 把一个域名注册到 Tesla,使其公钥被认可。
//
// 端点:POST {APIBase}/api/1/partner_accounts
// Body: { "domain": "<root domain>" }
func RegisterPartner(ctx context.Context, partnerAccessToken, region, domain string) (map[string]any, error) {
	if partnerAccessToken == "" {
		return nil, errors.New("partner: access_token required")
	}
	if domain == "" {
		return nil, errors.New("partner: domain required")
	}
	ep, err := tesla.EndpointsFor(region)
	if err != nil {
		return nil, fmt.Errorf("partner: %w", err)
	}
	body, _ := json.Marshal(map[string]string{"domain": domain})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ep.APIBase+"/api/1/partner_accounts", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("partner: build req: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+partnerAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	tesla.SetUA(req)

	client := tesla.NewHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("partner: post: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("partner: register %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"raw": string(raw)}, nil
	}
	return out, nil
}

// VerifyPartnerPublicKey 调 GET /api/1/partner_accounts/public_key?domain=...
// 返回服务端记录的公钥,供 CLI doctor 比对本地。
func VerifyPartnerPublicKey(ctx context.Context, partnerAccessToken, region, domain string) (map[string]any, error) {
	ep, err := tesla.EndpointsFor(region)
	if err != nil {
		return nil, fmt.Errorf("partner: %w", err)
	}
	q := url.Values{}
	q.Set("domain", domain)
	full := ep.APIBase + "/api/1/partner_accounts/public_key?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, fmt.Errorf("partner: build req: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+partnerAccessToken)
	req.Header.Set("Accept", "application/json")
	tesla.SetUA(req)

	client := tesla.NewHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("partner: get: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("partner: public_key %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"raw": string(raw)}, nil
	}
	return out, nil
}
