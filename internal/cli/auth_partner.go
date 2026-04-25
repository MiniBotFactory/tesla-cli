package cli

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/auth"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

const partnerProfileSuffix = "-partner"

const authPartnerLong = `合作伙伴账号(client_credentials)子命令。

用途
  • partner token       用 client_credentials grant 拿合作伙伴令牌
  • partner register    POST /api/1/partner_accounts 注册域名
  • partner verify      GET  /api/1/partner_accounts/public_key 验证公钥`

func newAuthPartnerCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "partner",
		Short:         "合作伙伴账号 token / 域名注册",
		Long:          authPartnerLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newPartnerTokenCommand(v),
		newPartnerRegisterCommand(v),
		newPartnerVerifyCommand(v),
	)
	return cmd
}

func newPartnerTokenCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "token",
		Short:         "用 client_credentials 拿 partner token",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			values, err := loadConfigTOML(cfg.ConfigFilePath())
			if err != nil {
				return errs.New(errs.ExitConfig, "no config file").
					WithHint("run `tesla config init` first")
			}
			scopesArg, _ := cmd.Flags().GetString("scopes")
			audienceArg, _ := cmd.Flags().GetString("audience")
			scopes := strings.Fields(firstNonEmpty(scopesArg, "openid vehicle_device_data"))
			region := firstNonEmpty(cfg.Region, values["region"], "na")

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			tok, err := auth.PartnerToken(ctx, auth.PartnerOptions{
				Region:       region,
				ClientID:     values["client_id"],
				ClientSecret: values["client_secret"],
				Scopes:       scopes,
				Audience:     audienceArg,
			})
			if err != nil {
				return errs.Wrap(errs.ExitAuth, "partner token", err)
			}
			store := auth.NewFileStore(cfg.BaseDir)
			profile := cfg.Profile + partnerProfileSuffix
			if err := store.Save(profile, tok); err != nil {
				return errs.Wrap(errs.ExitConfig, "save partner token", err)
			}
			env := output.Success(map[string]any{
				"profile": profile,
				"path":    store.Path(profile),
				"token":   tok.SafeView(),
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("scopes", "", "覆盖默认 scopes(空格分隔)")
	cmd.Flags().String("audience", "", "Fleet API base(可选)")
	return cmd
}

func newPartnerRegisterCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "register",
		Short:         "POST /api/1/partner_accounts 注册域名",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			values, err := loadConfigTOML(cfg.ConfigFilePath())
			if err != nil {
				return errs.New(errs.ExitConfig, "no config file").
					WithHint("run `tesla config init` first")
			}
			domainArg, _ := cmd.Flags().GetString("domain")
			domain := firstNonEmpty(domainArg, values["domain"])
			if domain == "" {
				return errs.New(errs.ExitUsage, "domain required").
					WithHint("set --domain or `tesla config set domain ...`")
			}
			tok, err := loadPartnerToken(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			region := firstNonEmpty(cfg.Region, values["region"], "na")
			res, err := auth.RegisterPartner(ctx, tok.AccessToken, region, domain)
			if err != nil {
				return errs.Wrap(errs.ExitGeneric, "partner register", err)
			}
			env := output.Success(map[string]any{
				"domain":   domain,
				"response": res,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("domain", "", "要注册的根域名(覆盖配置)")
	return cmd
}

func newPartnerVerifyCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "verify",
		Short:         "GET /api/1/partner_accounts/public_key 验证已注册公钥",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			values, err := loadConfigTOML(cfg.ConfigFilePath())
			if err != nil {
				return errs.New(errs.ExitConfig, "no config file").
					WithHint("run `tesla config init` first")
			}
			domainArg, _ := cmd.Flags().GetString("domain")
			domain := firstNonEmpty(domainArg, values["domain"])
			if domain == "" {
				return errs.New(errs.ExitUsage, "domain required")
			}
			tok, err := loadPartnerToken(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			region := firstNonEmpty(cfg.Region, values["region"], "na")
			res, err := auth.VerifyPartnerPublicKey(ctx, tok.AccessToken, region, domain)
			if err != nil {
				return errs.Wrap(errs.ExitGeneric, "partner verify", err)
			}
			env := output.Success(map[string]any{
				"domain":   domain,
				"response": res,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("domain", "", "要验证的根域名(覆盖配置)")
	return cmd
}

// loadPartnerToken 读取 <profile>-partner 的 token,过期则提示。
func loadPartnerToken(cfg config.Config) (*auth.Token, error) {
	store := auth.NewFileStore(cfg.BaseDir)
	profile := cfg.Profile + partnerProfileSuffix
	tok, err := store.Load(profile)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil, errs.New(errs.ExitAuth, "no partner token").
				WithHint("run `tesla auth partner token` first")
		}
		return nil, errs.Wrap(errs.ExitGeneric, "load partner token", err)
	}
	if tok.Expired() {
		return nil, errs.New(errs.ExitAuth, "partner token expired").
			WithHint("re-run `tesla auth partner token`")
	}
	return tok, nil
}
