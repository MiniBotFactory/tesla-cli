package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wmango/tesla-cli/internal/errs"
)

// Vehicle 是 GET /api/1/vehicles 列表元素的精简视图。
//
// Tesla 实际响应字段更多;这里挑稳定的几个,其他通过 --raw 看原始数据。
type Vehicle struct {
	ID          int64  `json:"id"           yaml:"id"`
	VehicleID   int64  `json:"vehicle_id"   yaml:"vehicle_id"`
	VIN         string `json:"vin"          yaml:"vin"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	State       string `json:"state"        yaml:"state"` // online / asleep / offline
	APIVersion  int    `json:"api_version"  yaml:"api_version"`
}

// listVehiclesEnvelope 是 Tesla 列表响应的标准信封。
type listVehiclesEnvelope struct {
	Response   []Vehicle      `json:"response"`
	Count      int            `json:"count"`
	Pagination map[string]any `json:"pagination,omitempty"`
}

// ListVehicles 拉取所有车辆。
func ListVehicles(ctx context.Context, c *Client) ([]Vehicle, error) {
	raw, err := c.Get(ctx, "/api/1/vehicles")
	if err != nil {
		return nil, err
	}
	var env listVehiclesEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("client: parse vehicles: %w", err)
	}
	return env.Response, nil
}

// VehicleData 拉取车辆实时数据(电量 / 位置 / 温度等)。
// vinOrID 接受 17 字符 VIN 或数字 id。
func VehicleData(ctx context.Context, c *Client, vinOrID string) (map[string]any, error) {
	if vinOrID == "" {
		return nil, errs.New(errs.ExitUsage, "vin or id required")
	}
	raw, err := c.Get(ctx, "/api/1/vehicles/"+vinOrID+"/vehicle_data")
	if err != nil {
		return nil, err
	}
	var env struct {
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("client: parse vehicle_data: %w", err)
	}
	return env.Response, nil
}

// WakeUp 唤醒车辆。Tesla 通常 5-30s 内进入 online。
func WakeUp(ctx context.Context, c *Client, idOrVin string) (map[string]any, error) {
	if idOrVin == "" {
		return nil, errs.New(errs.ExitUsage, "vehicle id or vin required")
	}
	raw, err := c.Post(ctx, "/api/1/vehicles/"+idOrVin+"/wake_up", nil)
	if err != nil {
		return nil, err
	}
	var env struct {
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("client: parse wake_up: %w", err)
	}
	return env.Response, nil
}

// ResolveVehicle 接受 VIN 或数字 id,返回该车辆的完整 Vehicle 元数据。
// 用于需要数字 id 时把 VIN 映射成 id。
func ResolveVehicle(ctx context.Context, c *Client, vinOrID string) (*Vehicle, error) {
	vinOrID = strings.TrimSpace(vinOrID)
	if vinOrID == "" {
		return nil, errs.New(errs.ExitUsage, "vin or id required")
	}
	all, err := ListVehicles(ctx, c)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].VIN == vinOrID || idMatch(all[i].ID, vinOrID) {
			return &all[i], nil
		}
	}
	return nil, errs.New(errs.ExitUsage,
		fmt.Sprintf("no vehicle matches %q", vinOrID)).
		WithHint("run `tesla vehicle list` to see available VINs")
}

func idMatch(id int64, s string) bool {
	return fmt.Sprintf("%d", id) == s
}
