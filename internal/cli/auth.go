package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/auth"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

const authLong = `OAuth 认证管理。

子命令
  login      打开浏览器走 authorization_code + PKCE,拿车主令牌
  refresh    用 refresh_token 换新 access_token
  logout     删除本地保存的 token
  status     查看当前 profile 的 token 元数据(脱敏)
  token      打印 access_token(脚本 / Agent 用)
  partner    合作伙伴账号 token / 域名注册`

func newAuthCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "auth",
		Short:         "OAuth 认证管理",
		Long:          authLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newAuthLoginCommand(v),
		newAuthRefreshCommand(v),
		newAuthLogoutCommand(v),
		newAuthStatusCommand(v),
		newAuthTokenCommand(v),
		newAuthPartnerCommand(v),
	)
	return cmd
}

const authLoginLong = `走 OAuth authorization_code + PKCE 流程,获取车主令牌。

前置
  • tesla config init 已写入 client_id / client_secret / redirect_uri / region

行为
  1. 启动本地 HTTP 监听 redirect_uri 端口
  2. 打开默认浏览器(--no-browser 关闭)
  3. 等待 callback 拿 code,POST 换 token
  4. 把 token 存到 ~/.config/tesla/profiles/<profile>.json (mode 0600)

示例
  tesla auth login
  tesla auth login --scopes "openid offline_access vehicle_device_data vehicle_cmds"
  tesla auth login --no-browser   # 仅打印 URL,适合远程 / CI`

func newAuthLoginCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "login",
		Short:         "OAuth 授权登录",
		Long:          authLoginLong,
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
			noBrowser, _ := cmd.Flags().GetBool("no-browser")
			manualFlag, _ := cmd.Flags().GetBool("manual")
			timeoutArg, _ := cmd.Flags().GetDuration("login-timeout")

			scopes := strings.Fields(firstNonEmpty(scopesArg, values["scopes"]))
			region := firstNonEmpty(cfg.Region, values["region"], "na")

			ctx, cancel := context.WithTimeout(cmd.Context(), timeoutArg)
			defer cancel()

			tok, err := auth.Login(ctx, auth.LoginOptions{
				Region:       region,
				ClientID:     values["client_id"],
				ClientSecret: values["client_secret"],
				RedirectURI:  values["redirect_uri"],
				Scopes:       scopes,
				OpenBrowser:  !noBrowser,
				Timeout:      timeoutArg,
				Notify:       cmd.ErrOrStderr(),
				Manual:       manualFlag,
				Stdin:        cmd.InOrStdin(),
			})
			if err != nil {
				return errs.Wrap(errs.ExitAuth, "login", err)
			}
			store := auth.NewFileStore(cfg.BaseDir)
			if err := store.Save(cfg.Profile, tok); err != nil {
				return errs.Wrap(errs.ExitConfig, "save token", err)
			}
			env := output.Success(map[string]any{
				"profile": cfg.Profile,
				"path":    store.Path(cfg.Profile),
				"token":   tok.SafeView(),
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("scopes", "", "覆盖配置里的 scopes(空格分隔)")
	cmd.Flags().Bool("no-browser", false, "不自动打开浏览器,仅打印 URL")
	cmd.Flags().Bool("manual", false, "强制 manual paste 模式(redirect_uri 非 localhost 时自动开启)")
	cmd.Flags().Duration("login-timeout", 5*time.Minute, "等待用户授权的超时")
	return cmd
}

func newAuthRefreshCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "refresh",
		Short:         "用 refresh_token 换新 access_token",
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
			store := auth.NewFileStore(cfg.BaseDir)
			cur, err := store.Load(cfg.Profile)
			if err != nil {
				return errs.Wrap(errs.ExitAuth, "load token", err).
					WithHint("run `tesla auth login` first")
			}
			if cur.RefreshToken == "" {
				return errs.New(errs.ExitAuth, "no refresh_token (login again with offline_access scope)")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			tok, err := auth.Refresh(ctx, auth.RefreshOptions{
				Region:       firstNonEmpty(cfg.Region, values["region"], "na"),
				ClientID:     values["client_id"],
				ClientSecret: values["client_secret"],
				RefreshToken: cur.RefreshToken,
			})
			if err != nil {
				return errs.Wrap(errs.ExitAuth, "refresh", err)
			}
			if err := store.Save(cfg.Profile, tok); err != nil {
				return errs.Wrap(errs.ExitConfig, "save token", err)
			}
			env := output.Success(map[string]any{
				"profile": cfg.Profile,
				"token":   tok.SafeView(),
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newAuthLogoutCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "logout",
		Short:         "删除本地 token",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			store := auth.NewFileStore(cfg.BaseDir)
			path := store.Path(cfg.Profile)
			if err := store.Delete(cfg.Profile); err != nil {
				return errs.Wrap(errs.ExitGeneric, "logout", err)
			}
			env := output.Success(map[string]any{
				"profile": cfg.Profile,
				"deleted": path,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newAuthStatusCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "status",
		Short:         "查看当前 profile 的 token 元数据(脱敏)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			store := auth.NewFileStore(cfg.BaseDir)
			tok, err := store.Load(cfg.Profile)
			if err != nil {
				if errors.Is(err, auth.ErrNotFound) {
					return errs.New(errs.ExitAuth, "no token for profile "+cfg.Profile).
						WithHint("run `tesla auth login`")
				}
				return errs.Wrap(errs.ExitGeneric, "load token", err)
			}
			env := output.Success(map[string]any{
				"profile": cfg.Profile,
				"path":    store.Path(cfg.Profile),
				"token":   tok.SafeView(),
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newAuthTokenCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "token",
		Short:         "打印 access_token(脚本 / Agent 用)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			store := auth.NewFileStore(cfg.BaseDir)
			tok, err := store.Load(cfg.Profile)
			if err != nil {
				if errors.Is(err, auth.ErrNotFound) {
					return errs.New(errs.ExitAuth, "no token for profile "+cfg.Profile).
						WithHint("run `tesla auth login`")
				}
				return errs.Wrap(errs.ExitGeneric, "load token", err)
			}
			if tok.Expired() {
				return errs.New(errs.ExitAuth, "token expired").
					WithHint("run `tesla auth refresh`").
					WithRetryable(true)
			}
			if cfg.Quiet {
				fmt.Fprintln(cmd.OutOrStdout(), tok.AccessToken)
				return nil
			}
			env := output.Success(map[string]any{
				"access_token": tok.AccessToken,
				"token_type":   tok.TokenType,
				"expires_at":   tok.ExpiresAt,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}
