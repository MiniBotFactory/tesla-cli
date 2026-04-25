package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wmango/tesla-cli/internal/errs"
)

// ListProducts 拉取 /api/1/products 混合列表(车辆 + Powerwall + Solar)。
// 不强类型化:不同产品字段差异大;CLI 层用 --jq 过滤。
func ListProducts(ctx context.Context, c *Client) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/api/1/products")
	if err != nil {
		return nil, err
	}
	var env struct {
		Response []map[string]any `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("client: parse products: %w", err)
	}
	return env.Response, nil
}

// EnergySiteInfo 拉取能源站点静态信息。
func EnergySiteInfo(ctx context.Context, c *Client, siteID string) (map[string]any, error) {
	if siteID == "" {
		return nil, errs.New(errs.ExitUsage, "site_id required")
	}
	raw, err := c.Get(ctx, "/api/1/energy_sites/"+siteID+"/site_info")
	if err != nil {
		return nil, err
	}
	var env struct {
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("client: parse site_info: %w", err)
	}
	return env.Response, nil
}

// EnergyLiveStatus 拉取能源站点实时状态(功率、SOC 等)。
func EnergyLiveStatus(ctx context.Context, c *Client, siteID string) (map[string]any, error) {
	if siteID == "" {
		return nil, errs.New(errs.ExitUsage, "site_id required")
	}
	raw, err := c.Get(ctx, "/api/1/energy_sites/"+siteID+"/live_status")
	if err != nil {
		return nil, err
	}
	var env struct {
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("client: parse live_status: %w", err)
	}
	return env.Response, nil
}
