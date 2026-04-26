// Package command 封装对车辆下发"写"指令(走签名 + Vehicle Command Proxy)的
// 公共流程。基于官方 github.com/teslamotors/vehicle-command SDK。
//
// 用法:
//
//	err := command.Run(ctx, command.Options{
//	    AccessToken:    tok.AccessToken,
//	    PrivateKeyPath: filepath.Join(baseDir, "keys", "private-key.pem"),
//	    VIN:            "5YJ...",
//	}, func(v *vehicle.Vehicle) error {
//	    return v.Lock(ctx)
//	})
package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

// UserAgent 是发送给 Tesla 的客户端标识。
// 与 internal/tesla.UserAgent 对齐;独立常量避免 internal/command 依赖
// internal/tesla(后者面向 Fleet API REST,本包面向签名命令通道)。
const UserAgent = "tesla-cli/0.0"

// Options 描述发送一条命令所需的全部输入。
type Options struct {
	AccessToken    string // 必需:车主 OAuth access_token(已校验未过期)
	PrivateKeyPath string // 必需:虚拟钥匙私钥 PEM 文件绝对路径
	VIN            string // 必需:17 位 VIN
}

// Action 是调用方传入的"在 vehicle 已建立加密会话后要做的事"。
// SDK 提供的高层方法都可以放进去,如 v.Lock(ctx) / v.ChangeChargeLimit(ctx, 80)。
type Action func(*vehicle.Vehicle) error

// Run 完成 加载私钥 → 构造 account → 拿 vehicle handle → Connect →
// StartSession → fn(v) → Disconnect 的完整链路。
//
// 任何一步失败都会立即返回包装错误;Disconnect 用 defer 保证。
func Run(ctx context.Context, opts Options, fn Action) error {
	if err := opts.validate(); err != nil {
		return err
	}
	if fn == nil {
		return errors.New("command: nil action")
	}

	key, err := protocol.LoadPrivateKey(opts.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("command: load private key %s: %w", opts.PrivateKeyPath, err)
	}

	acct, err := account.New(opts.AccessToken, UserAgent)
	if err != nil {
		return fmt.Errorf("command: account: %w", err)
	}

	veh, err := acct.GetVehicle(ctx, opts.VIN, key, nil)
	if err != nil {
		return fmt.Errorf("command: get vehicle %s: %w", opts.VIN, err)
	}
	defer veh.Disconnect()

	if err := veh.Connect(ctx); err != nil {
		return fmt.Errorf("command: connect: %w", err)
	}
	if err := veh.StartSession(ctx, nil); err != nil {
		return fmt.Errorf("command: start session: %w", err)
	}

	return fn(veh)
}

// validate 校验 Options 必需字段。
func (o Options) validate() error {
	if o.AccessToken == "" {
		return errors.New("command: AccessToken required")
	}
	if o.PrivateKeyPath == "" {
		return errors.New("command: PrivateKeyPath required")
	}
	if o.VIN == "" {
		return errors.New("command: VIN required")
	}
	return nil
}
