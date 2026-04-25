package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/client"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

const energyLong = `Powerwall / Solar 能源站点查询。

子命令
  list              列出账号下所有产品(车辆 + 能源站点混合)
  info <site_id>    站点静态信息
  live <site_id>    实时状态(功率 / SOC / grid)

提示
  list 输出包含车辆;只看能源站点用 --jq:
    tesla energy list --jq '[.data.products[] | select(.resource_type=="battery")]'`

func newEnergyCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "energy",
		Short:         "Powerwall / Solar 能源站点",
		Long:          energyLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newEnergyListCommand(v),
		newEnergyInfoCommand(v),
		newEnergyLiveCommand(v),
	)
	return cmd
}

func newEnergyListCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "列出账号下所有产品",
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
			items, err := client.ListProducts(ctx, c)
			if err != nil {
				return err
			}
			env := output.Success(map[string]any{
				"count":    len(items),
				"products": items,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newEnergyInfoCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "info <site_id>",
		Short:         "能源站点静态信息",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			siteID := args[0]
			if siteID == "" {
				return errs.New(errs.ExitUsage, "site_id required")
			}
			c, err := buildAPIClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			data, err := client.EnergySiteInfo(ctx, c, siteID)
			if err != nil {
				return err
			}
			env := output.Success(data, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newEnergyLiveCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "live <site_id>",
		Short:         "能源站点实时状态",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			siteID := args[0]
			if siteID == "" {
				return errs.New(errs.ExitUsage, "site_id required")
			}
			c, err := buildAPIClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			data, err := client.EnergyLiveStatus(ctx, c, siteID)
			if err != nil {
				return err
			}
			env := output.Success(data, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}
