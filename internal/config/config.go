// Package config 提供配置加载与路径解析。
//
// 优先级(高→低):flag > 环境变量 TESLA_* > 配置文件 > 内置默认。
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// EnvPrefix 是所有环境变量的统一前缀,例如 TESLA_REGION。
const EnvPrefix = "TESLA"

// Config 是不可变的运行时配置快照。
//
// 不要在运行期间修改字段;改用 With* 方法返回新副本。
type Config struct {
	Profile string
	Region  string
	Output  string
	Quiet   bool
	Verbose int
	Color   bool
	Timeout string
	Retry   int
	DryRun  bool
	VIN     string
	CfgPath string
	BaseDir string
}

// DefaultConfig 返回内置默认值。任何字段都可被 flag/env/file 覆盖。
//
// Region 故意留空:cobra `--region` flag 也默认空,这样 cli 层
// `firstNonEmpty(cfg.Region, values["region"], "na")` 才能让
// config.toml 里的 region 接手;否则 builtin "na" 会盖住 toml。
func DefaultConfig() Config {
	return Config{
		Profile: "default",
		Region:  "",
		Output:  "json",
		Quiet:   false,
		Verbose: 0,
		Color:   true,
		Timeout: "30s",
		Retry:   3,
		DryRun:  false,
		BaseDir: defaultBaseDir(),
	}
}

// defaultBaseDir 返回 ~/.config/tesla(XDG)。
func defaultBaseDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "tesla")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".tesla"
	}
	return filepath.Join(home, ".config", "tesla")
}

// BindViper 把 viper 的当前值写入新副本并返回。
// 不修改 v 的状态(只读消费)。
func (c Config) BindViper(v *viper.Viper) Config {
	cp := c
	if s := v.GetString("profile"); s != "" {
		cp.Profile = s
	}
	if s := v.GetString("region"); s != "" {
		cp.Region = s
	}
	if s := v.GetString("output"); s != "" {
		cp.Output = s
	}
	if v.IsSet("quiet") {
		cp.Quiet = v.GetBool("quiet")
	}
	if v.IsSet("verbose") {
		cp.Verbose = v.GetInt("verbose")
	}
	if v.IsSet("no-color") {
		cp.Color = !v.GetBool("no-color")
	}
	if s := v.GetString("timeout"); s != "" {
		cp.Timeout = s
	}
	if v.IsSet("retry") {
		cp.Retry = v.GetInt("retry")
	}
	if v.IsSet("dry-run") {
		cp.DryRun = v.GetBool("dry-run")
	}
	if s := v.GetString("vin"); s != "" {
		cp.VIN = s
	}
	if s := v.GetString("config"); s != "" {
		cp.CfgPath = s
	}
	return cp
}

// ConfigFilePath 返回当前 profile 的配置文件路径。
// 文件格式:TOML;字段 client_id / domain / region / default_profile。
func (c Config) ConfigFilePath() string {
	if c.CfgPath != "" {
		return c.CfgPath
	}
	return filepath.Join(c.BaseDir, "config.toml")
}

// ProfileFilePath 返回当前 profile 的 token 文件路径。
// 文件格式:JSON;字段 access_token / refresh_token / expires_at(RFC3339)/ scopes。
func (c Config) ProfileFilePath() string {
	return filepath.Join(c.BaseDir, "profiles", c.Profile+".json")
}
