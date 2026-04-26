package cli

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	"github.com/wmango/tesla-cli/internal/command"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

// classifyCommandError 根据 SDK 错误字符串特征,把签名命令的失败精确归类到
// 退出码语义 + 给出可执行 hint。SDK 没暴露稳定错误码,只能 substring 匹配。
func classifyCommandError(description string, err error) *errs.Error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "offline") || strings.Contains(msg, "asleep") ||
		strings.Contains(msg, "unavailable"):
		return errs.Wrap(errs.ExitVehicleState, description, err).
			WithHint("vehicle is sleeping; run `tesla vehicle wake` and retry after ~15s").
			WithRetryable(true)
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized"):
		return errs.Wrap(errs.ExitAuth, description, err).
			WithHint("token expired or invalid; run `tesla auth refresh`").
			WithRetryable(true)
	case strings.Contains(msg, "scope"):
		return errs.Wrap(errs.ExitScope, description, err).
			WithHint("missing OAuth scope; re-run `tesla auth login` with `vehicle_cmds` / `vehicle_charging_cmds`")
	default:
		return errs.Wrap(errs.ExitVirtualKey, description, err).
			WithHint("ensure key pair exists (`tesla key generate`) and is paired with the vehicle (`tesla key pair-url`)")
	}
}

// runVehicleAction 是所有"需要虚拟钥匙签名"的车辆指令共享的执行骨架:
//
//  1. 解析 VIN
//  2. 加载未过期的 token(自动刷新逻辑由 ensureValidToken 内部决定)
//  3. command.Run(ctx, opts, action) — 完成 Connect+StartSession+fn+Disconnect
//  4. 写成功 Envelope
//
// description 进 Envelope 的 data.action 字段,便于 Agent 识别"做了什么"。
func runVehicleAction(
	cmd *cobra.Command,
	v *viper.Viper,
	args []string,
	description string,
	action command.Action,
) error {
	started := time.Now()
	cfg := config.DefaultConfig().BindViper(v)
	format, err := output.ParseFormat(cfg.Output)
	if err != nil {
		return err
	}
	vin, err := resolveVINArg(args, cfg)
	if err != nil {
		return err
	}
	tok, err := ensureValidToken(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	keyPath := filepath.Join(cfg.BaseDir, "keys", "private-key.pem")
	if err := command.Run(ctx, command.Options{
		AccessToken:    tok.AccessToken,
		PrivateKeyPath: keyPath,
		VIN:            vin,
	}, action); err != nil {
		return classifyCommandError(description, err)
	}
	env := output.Success(map[string]any{
		"action":  description,
		"vin":     vin,
		"applied": true,
	}, "", started)
	return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
}

const lockLong = `锁车(签名命令)。

前置
  • tesla auth login 已完成
  • tesla key generate + 公钥部署 + tesla key pair-url 已让车主把钥匙加到车上

scopes
  vehicle_cmds

退出码
  0   成功
  4   token 过期 / 鉴权失败
  6   虚拟钥匙未配对 / 签名失败 / Connect 失败
  7   车辆离线(请先 tesla vehicle wake)

示例
  tesla vehicle lock 5YJ...
  tesla vehicle lock --vin 5YJ...`

func newVehicleLockCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "lock [vin]",
		Short:         "锁车",
		Long:          lockLong,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVehicleAction(cmd, v, args, "lock",
				func(veh *vehicle.Vehicle) error { return veh.Lock(cmd.Context()) })
		},
	}
}

func newVehicleUnlockCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "unlock [vin]",
		Short:         "解锁车辆",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVehicleAction(cmd, v, args, "unlock",
				func(veh *vehicle.Vehicle) error { return veh.Unlock(cmd.Context()) })
		},
	}
}

func newVehicleHonkCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "honk [vin]",
		Short:         "鸣笛(短促一声)",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVehicleAction(cmd, v, args, "honk",
				func(veh *vehicle.Vehicle) error { return veh.HonkHorn(cmd.Context()) })
		},
	}
}

func newVehicleFlashCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "flash [vin]",
		Short:         "闪灯",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVehicleAction(cmd, v, args, "flash",
				func(veh *vehicle.Vehicle) error { return veh.FlashLights(cmd.Context()) })
		},
	}
}
