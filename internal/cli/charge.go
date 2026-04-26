package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	"github.com/wmango/tesla-cli/internal/errs"
)

const chargeLong = `充电控制(签名命令)。

子命令
  start <vin>             启动充电
  stop <vin>              停止充电
  limit <vin> --percent N 设置充电上限(50..100)`

func newChargeCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "charge",
		Short:         "充电控制",
		Long:          chargeLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newChargeStartCommand(v),
		newChargeStopCommand(v),
		newChargeLimitCommand(v),
	)
	return cmd
}

func newChargeStartCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "start [vin]",
		Short:         "启动充电",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVehicleAction(cmd, v, args, "charge.start",
				func(veh *vehicle.Vehicle) error { return veh.ChargeStart(cmd.Context()) })
		},
	}
}

func newChargeStopCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "stop [vin]",
		Short:         "停止充电",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVehicleAction(cmd, v, args, "charge.stop",
				func(veh *vehicle.Vehicle) error { return veh.ChargeStop(cmd.Context()) })
		},
	}
}

const chargeLimitLong = `设置充电上限百分比(50..100)。

scopes
  vehicle_charging_cmds

退出码
  0   成功
  2   --percent 参数越界
  4   token 过期
  6   虚拟钥匙未配对 / 签名失败
  7   车辆离线

示例
  tesla charge limit 5YJ... --percent 80
  TESLA_VIN=5YJ... tesla charge limit --percent 90`

func newChargeLimitCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "limit [vin]",
		Short:         "设置充电上限百分比",
		Long:          chargeLimitLong,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			percent, _ := cmd.Flags().GetInt("percent")
			if percent < 50 || percent > 100 {
				return errs.New(errs.ExitUsage, "percent must be in [50, 100]").
					WithHint("e.g. --percent 80")
			}
			return runVehicleAction(cmd, v, args, "charge.limit",
				func(veh *vehicle.Vehicle) error {
					return veh.ChangeChargeLimit(cmd.Context(), int32(percent))
				})
		},
	}
	cmd.Flags().Int("percent", 80, "充电上限百分比 (50..100)")
	return cmd
}
