package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultConfig_sensibleDefaults(t *testing.T) {
	c := DefaultConfig()
	if c.Profile != "default" {
		t.Errorf("Profile default mismatch: %q", c.Profile)
	}
	if c.Region != "na" {
		t.Errorf("Region default mismatch: %q", c.Region)
	}
	if c.Output != "json" {
		t.Errorf("Output default mismatch: %q", c.Output)
	}
	if c.Timeout != "30s" {
		t.Errorf("Timeout default mismatch: %q", c.Timeout)
	}
	if c.Retry != 3 {
		t.Errorf("Retry default mismatch: %d", c.Retry)
	}
	if !c.Color {
		t.Errorf("Color should default true")
	}
	if c.BaseDir == "" {
		t.Errorf("BaseDir should not be empty")
	}
}

func TestBindViper_overridesAllRecognizedKeys(t *testing.T) {
	v := viper.New()
	v.Set("profile", "work")
	v.Set("region", "eu")
	v.Set("output", "yaml")
	v.Set("quiet", true)
	v.Set("verbose", 2)
	v.Set("no-color", true)
	v.Set("timeout", "10s")
	v.Set("retry", 5)
	v.Set("dry-run", true)
	v.Set("vin", "5YJ12345678901234")
	v.Set("config", "/tmp/x.toml")

	got := DefaultConfig().BindViper(v)

	checks := []struct {
		field string
		ok    bool
	}{
		{"Profile", got.Profile == "work"},
		{"Region", got.Region == "eu"},
		{"Output", got.Output == "yaml"},
		{"Quiet", got.Quiet == true},
		{"Verbose", got.Verbose == 2},
		{"Color", got.Color == false},
		{"Timeout", got.Timeout == "10s"},
		{"Retry", got.Retry == 5},
		{"DryRun", got.DryRun == true},
		{"VIN", got.VIN == "5YJ12345678901234"},
		{"CfgPath", got.CfgPath == "/tmp/x.toml"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("BindViper did not override %s correctly: %+v", c.field, got)
		}
	}
}

func TestBindViper_returnsNewCopy(t *testing.T) {
	v := viper.New()
	v.Set("profile", "x")

	orig := DefaultConfig()
	derived := orig.BindViper(v)

	if orig.Profile == "x" {
		t.Fatalf("BindViper must not mutate receiver")
	}
	if derived.Profile != "x" {
		t.Fatalf("derived should have new profile")
	}
}

func TestConfigFilePath_default(t *testing.T) {
	c := DefaultConfig()
	got := c.ConfigFilePath()
	if !strings.HasSuffix(got, "config.toml") {
		t.Fatalf("ConfigFilePath should end in config.toml, got %q", got)
	}
}

func TestConfigFilePath_overrideViaCfgPath(t *testing.T) {
	c := DefaultConfig()
	c.CfgPath = "/etc/custom.toml"
	if got := c.ConfigFilePath(); got != "/etc/custom.toml" {
		t.Fatalf("CfgPath override ignored: %q", got)
	}
}

func TestProfileFilePath_includesProfileName(t *testing.T) {
	c := DefaultConfig()
	c.Profile = "work"
	got := c.ProfileFilePath()
	want := filepath.Join(c.BaseDir, "profiles", "work.json")
	if got != want {
		t.Fatalf("ProfileFilePath: want %q, got %q", want, got)
	}
}

func TestDefaultBaseDir_xdgEnvWins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	got := defaultBaseDir()
	want := filepath.Join("/tmp/xdg-test", "tesla")
	if got != want {
		t.Fatalf("XDG override ignored: want %q, got %q", want, got)
	}
}
