package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/output"
)

// configKeys 是 ~/.config/tesla/config.toml 支持的所有键(白名单)。
var configKeys = []string{
	"client_id",
	"client_secret",
	"domain",
	"region",
	"redirect_uri",
	"scopes",
}

const configLong = `管理 ~/.config/tesla/config.toml。

子命令
  init      非交互式写入完整配置(从 flag / env)
  set       设置单个字段
  get       读取单个字段
  path      打印配置文件绝对路径
  show      打印当前配置(脱敏)`

func newConfigCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "config",
		Short:         "配置管理",
		Long:          configLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newConfigInitCommand(v),
		newConfigSetCommand(v),
		newConfigGetCommand(v),
		newConfigPathCommand(v),
		newConfigShowCommand(v),
	)
	return cmd
}

const configInitLong = `非交互式写入完整配置。

为 Agent 友好,本命令不弹 prompt。所有字段从 flag / 环境变量读取。

环境变量映射(可与 flag 互换):
  TESLA_CLIENT_ID         --client-id
  TESLA_CLIENT_SECRET     --client-secret
  TESLA_DOMAIN            --domain
  TESLA_REDIRECT_URI      --redirect-uri
  TESLA_SCOPES            --scopes

示例
  tesla config init \
    --client-id $TESLA_CLIENT_ID \
    --client-secret $TESLA_CLIENT_SECRET \
    --domain my.example.com \
    --region-init na \
    --redirect-uri http://localhost:8765/callback \
    --scopes "openid offline_access vehicle_device_data vehicle_cmds"`

func newConfigInitCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "非交互式写入完整配置",
		Long:          configInitLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			values, err := readInitFlags(cmd, v)
			if err != nil {
				return err
			}
			force, _ := cmd.Flags().GetBool("force")
			if err := writeConfigTOML(cfg.ConfigFilePath(), values, force); err != nil {
				return errs.Wrap(errs.ExitConfig, "init: write config", err)
			}
			env := output.Success(map[string]any{
				"path":   cfg.ConfigFilePath(),
				"keys":   sortedKeys(values),
				"region": values["region"],
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("client-id", "", "Tesla 应用 client_id (env: TESLA_CLIENT_ID)")
	cmd.Flags().String("client-secret", "", "Tesla 应用 client_secret (env: TESLA_CLIENT_SECRET)")
	cmd.Flags().String("domain", "", "你的应用根域名(用于公钥托管 / partner register)")
	cmd.Flags().String("region-init", "na", "Tesla API 区域:na | eu | cn(避免与全局 --region 冲突)")
	cmd.Flags().String("redirect-uri", "http://localhost:8765/callback", "OAuth 回调 URL")
	cmd.Flags().String("scopes", "openid offline_access vehicle_device_data vehicle_cmds", "空格分隔的 scope 列表")
	cmd.Flags().Bool("force", false, "覆盖已存在配置")
	return cmd
}

func readInitFlags(cmd *cobra.Command, v *viper.Viper) (map[string]string, error) {
	get := func(flag, envKey string) string {
		s, _ := cmd.Flags().GetString(flag)
		if s != "" {
			return s
		}
		return v.GetString(envKey)
	}
	values := map[string]string{
		"client_id":     get("client-id", "client_id"),
		"client_secret": get("client-secret", "client_secret"),
		"domain":        get("domain", "domain"),
		"region":        firstNonEmpty(getFlag(cmd, "region-init"), v.GetString("region")),
		"redirect_uri":  get("redirect-uri", "redirect_uri"),
		"scopes":        get("scopes", "scopes"),
	}
	if values["client_id"] == "" {
		return nil, errs.New(errs.ExitConfig, "config init: --client-id required").
			WithHint("set --client-id or TESLA_CLIENT_ID")
	}
	if values["region"] == "" {
		values["region"] = "na"
	}
	return values, nil
}

func getFlag(cmd *cobra.Command, name string) string {
	s, _ := cmd.Flags().GetString(name)
	return s
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

func newConfigSetCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "set <key> <value>",
		Short:         "设置单个字段",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			key, value := args[0], args[1]
			if !validKey(key) {
				return errs.New(errs.ExitUsage,
					fmt.Sprintf("unknown key %q (allowed: %s)", key, strings.Join(configKeys, ", ")))
			}
			values, _ := loadConfigTOML(cfg.ConfigFilePath())
			if values == nil {
				values = map[string]string{}
			}
			values[key] = value
			if err := writeConfigTOML(cfg.ConfigFilePath(), values, true); err != nil {
				return errs.Wrap(errs.ExitConfig, "set", err)
			}
			env := output.Success(map[string]any{"key": key, "path": cfg.ConfigFilePath()}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newConfigGetCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "get <key>",
		Short:         "读取单个字段",
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
			key := args[0]
			if !validKey(key) {
				return errs.New(errs.ExitUsage,
					fmt.Sprintf("unknown key %q (allowed: %s)", key, strings.Join(configKeys, ", ")))
			}
			values, err := loadConfigTOML(cfg.ConfigFilePath())
			if err != nil {
				return errs.Wrap(errs.ExitConfig, "get", err)
			}
			val, ok := values[key]
			env := output.Success(map[string]any{"key": key, "value": val, "exists": ok}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newConfigPathCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "path",
		Short:         "打印配置文件绝对路径",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			env := output.Success(map[string]any{
				"config_path":  cfg.ConfigFilePath(),
				"profile_path": cfg.ProfileFilePath(),
				"base_dir":     cfg.BaseDir,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func newConfigShowCommand(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:           "show",
		Short:         "打印当前配置(脱敏)",
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
				if errors.Is(err, os.ErrNotExist) {
					return errs.New(errs.ExitConfig, "no config file").
						WithHint("run `tesla config init` first")
				}
				return errs.Wrap(errs.ExitConfig, "show", err)
			}
			env := output.Success(map[string]any{
				"path":   cfg.ConfigFilePath(),
				"values": redactSensitive(values),
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
}

func validKey(k string) bool {
	for _, x := range configKeys {
		if x == k {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func redactSensitive(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k == "client_secret" && v != "" {
			out[k] = "***"
			continue
		}
		out[k] = v
	}
	return out
}

// 简易 TOML 读写:仅支持单层 key="value"。

func writeConfigTOML(path string, values map[string]string, force bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force)", path)
		}
	}
	var b strings.Builder
	b.WriteString("# tesla-cli configuration\n")
	for _, k := range configKeys {
		v := values[k]
		v = strings.ReplaceAll(v, `"`, `\"`)
		fmt.Fprintf(&b, "%s = \"%s\"\n", k, v)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadConfigTOML(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.TrimPrefix(val, `"`)
		val = strings.TrimSuffix(val, `"`)
		val = strings.ReplaceAll(val, `\"`, `"`)
		out[key] = val
	}
	return out, nil
}
