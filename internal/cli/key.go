package cli

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/keys"
	"github.com/wmango/tesla-cli/internal/output"
)

const keyLong = `虚拟钥匙密钥对管理。

子命令
  generate    生成 EC P-256 密钥对(private-key.pem + public-key.pem)
  publish     输出把公钥部署到 .well-known 的指引
  pubkey      打印本地公钥 PEM(便于复制粘贴到部署平台)
  pair-url    生成把虚拟钥匙添加到车辆的 Tesla App 深链`

func newKeyCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "key",
		Short:         "虚拟钥匙密钥对管理",
		Long:          keyLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newKeyGenerateCommand(v),
		newKeyPublishCommand(v),
		newKeyPubkeyCommand(v),
		newKeyPairURLCommand(v),
	)
	return cmd
}

func newKeyGenerateCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "generate",
		Short:         "生成 EC P-256 密钥对到指定目录",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			outArg, _ := cmd.Flags().GetString("out")
			force, _ := cmd.Flags().GetBool("force")
			outDir := outArg
			if outDir == "" {
				outDir = filepath.Join(cfg.BaseDir, "keys")
			}
			res, err := keys.Generate(keys.GenerateOptions{OutDir: outDir, Force: force})
			if err != nil {
				return errs.Wrap(errs.ExitGeneric, "key generate", err)
			}
			env := output.Success(res, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("out", "", "输出目录(默认 <baseDir>/keys)")
	cmd.Flags().Bool("force", false, "覆盖已存在文件")
	return cmd
}

func newKeyPublishCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "publish",
		Short:         "输出公钥部署到 .well-known 的指引",
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
			fromArg, _ := cmd.Flags().GetString("from")
			domain := firstNonEmpty(domainArg, values["domain"])
			if domain == "" {
				return errs.New(errs.ExitUsage, "domain required").
					WithHint("set --domain or `tesla config set domain ...`")
			}
			pubPath := fromArg
			if pubPath == "" {
				pubPath = keys.PublicKeyPath(filepath.Join(cfg.BaseDir, "keys"))
			}
			steps := keys.PublishInstructions(domain, pubPath)
			env := output.Success(map[string]any{
				"domain":          domain,
				"public_key_path": pubPath,
				"well_known_path": keys.WellKnownPath,
				"deploy_target":   "https://" + domain + "/" + keys.WellKnownPath,
				"instructions":    steps,
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("domain", "", "目标域名(覆盖配置)")
	cmd.Flags().String("from", "", "本地 public-key.pem 路径(默认 <baseDir>/keys/public-key.pem)")
	return cmd
}

func newKeyPubkeyCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pubkey",
		Short:         "打印本地公钥 PEM",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			fromArg, _ := cmd.Flags().GetString("from")
			pubPath := fromArg
			if pubPath == "" {
				pubPath = keys.PublicKeyPath(filepath.Join(cfg.BaseDir, "keys"))
			}
			data, err := os.ReadFile(pubPath)
			if err != nil {
				return errs.Wrap(errs.ExitGeneric, "read pubkey", err).
					WithHint("run `tesla key generate` first")
			}
			if cfg.Quiet {
				_, _ = cmd.OutOrStdout().Write(data)
				return nil
			}
			env := output.Success(map[string]any{
				"path": pubPath,
				"pem":  string(data),
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("from", "", "本地 public-key.pem 路径")
	return cmd
}

func newKeyPairURLCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pair-url",
		Short:         "生成 Tesla App 配对深链",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			started := time.Now()
			cfg := config.DefaultConfig().BindViper(v)
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			values, _ := loadConfigTOML(cfg.ConfigFilePath())
			domainArg, _ := cmd.Flags().GetString("domain")
			domain := firstNonEmpty(domainArg, values["domain"])
			if domain == "" {
				return errs.New(errs.ExitUsage, "domain required").
					WithHint("set --domain or `tesla config set domain ...`")
			}
			url, err := keys.PairURL(domain)
			if err != nil {
				return errs.Wrap(errs.ExitGeneric, "pair-url", err)
			}
			if cfg.Quiet {
				_, _ = cmd.OutOrStdout().Write([]byte(url + "\n"))
				return nil
			}
			env := output.Success(map[string]any{
				"domain": domain,
				"url":    url,
				"howto":  "Send this URL to the vehicle owner; tapping it on a phone with Tesla App pairs the virtual key.",
			}, "", started)
			return output.NewRenderer(format).Render(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().String("domain", "", "目标域名(覆盖配置)")
	return cmd
}
