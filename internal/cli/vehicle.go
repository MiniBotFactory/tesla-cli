package cli

import (
	"context"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/client"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

const vehicleLong = `车辆只读 / 唤醒命令。

子命令
  list        列出账号下所有车辆
  info <vin>  车辆静态信息(VIN / 型号 / 状态)
  data <vin>  实时数据(电量 / 位置 / 温度 / 软件版本)
  wake <vin>  唤醒车辆(休眠态 → online)

提示
  --vin 全局 flag 或 $TESLA_VIN 可省略 <vin> 参数。
  data 输出量大,常配 --jq 过滤,例:
    tesla vehicle data 5YJ... --jq '.charge_state.battery_level'`

func newVehicleCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "vehicle",
		Short:         "车辆查询 / 唤醒",
		Long:          vehicleLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newVehicleListCommand(v),
		newVehicleInfoCommand(v),
		newVehicleDataCommand(v),
		newVehicleWakeCommand(v),
	)
	return cmd
}

func newVehicleListCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "列出账号下所有车辆",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			c, err := buildAPIClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			vs, err := client.ListVehicles(ctx, c)
			if err != nil {
				return err
			}
			env := output.Success(map[string]any{
				"count":    len(vs),
				"vehicles": vs,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newVehicleInfoCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "info [vin]",
		Short:         "车辆静态信息",
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
			vin, err := resolveVINArg(args, cfg)
			if err != nil {
				return err
			}
			c, err := buildAPIClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			vh, err := client.ResolveVehicle(ctx, c, vin)
			if err != nil {
				return err
			}
			env := output.Success(vh, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newVehicleDataCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "data [vin]",
		Short:         "车辆实时数据",
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
			vin, err := resolveVINArg(args, cfg)
			if err != nil {
				return err
			}
			c, err := buildAPIClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			data, err := client.VehicleData(ctx, c, vin)
			if err != nil {
				return err
			}
			env := output.Success(data, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newVehicleWakeCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "wake [vin]",
		Short:         "唤醒车辆",
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
			vin, err := resolveVINArg(args, cfg)
			if err != nil {
				return err
			}
			c, err := buildAPIClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			vh, err := client.ResolveVehicle(ctx, c, vin)
			if err != nil {
				return err
			}
			res, err := client.WakeUp(ctx, c, strconv.FormatInt(vh.ID, 10))
			if err != nil {
				return err
			}
			env := output.Success(res, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

// resolveVINArg 从 args[0] 或 --vin / $TESLA_VIN 拿 VIN。
func resolveVINArg(args []string, cfg config.Config) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	if cfg.VIN != "" {
		return cfg.VIN, nil
	}
	return "", errs.New(errs.ExitUsage, "vin required").
		WithHint("pass <vin> as positional or use --vin / TESLA_VIN")
}

// buildAPIClient 把 ensureValidToken + client.New 组合起来。
// vehicle/energy 命令公用同一处错误处理。
func buildAPIClient(ctx context.Context, cfg config.Config) (*client.Client, error) {
	tok, err := ensureValidToken(ctx, cfg)
	if err != nil {
		return nil, err
	}
	values, _ := loadConfigTOML(cfg.ConfigFilePath())
	region := firstNonEmpty(cfg.Region, values["region"], "na")
	timeout, _ := time.ParseDuration(firstNonEmpty(cfg.Timeout, "30s"))
	c, err := client.New(client.Options{
		Region:      region,
		AccessToken: tok.AccessToken,
		Timeout:     timeout,
		Retry:       cfg.Retry,
	})
	if err != nil {
		return nil, errs.Wrap(errs.ExitConfig, "build client", err)
	}
	return c, nil
}
