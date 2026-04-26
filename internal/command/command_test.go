package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestOptions_validate_table(t *testing.T) {
	cases := []struct {
		name string
		opt  Options
		ok   bool
	}{
		{"missing token", Options{PrivateKeyPath: "p", VIN: "v"}, false},
		{"missing key path", Options{AccessToken: "t", VIN: "v"}, false},
		{"missing vin", Options{AccessToken: "t", PrivateKeyPath: "p"}, false},
		{"complete", Options{AccessToken: "t", PrivateKeyPath: "p", VIN: "v"}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opt.validate()
			if tc.ok && err != nil {
				t.Errorf("want ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("want error, got nil")
			}
		})
	}
}

func TestRun_nilActionRejected(t *testing.T) {
	err := Run(context.Background(), Options{
		AccessToken:    "t",
		PrivateKeyPath: "p",
		VIN:            "v",
	}, nil)
	if err == nil {
		t.Fatalf("nil action must error")
	}
	if !strings.Contains(err.Error(), "nil action") {
		t.Errorf("err should mention nil action: %v", err)
	}
}

func TestRun_missingFieldsRejected(t *testing.T) {
	err := Run(context.Background(), Options{},
		func(*vehicle.Vehicle) error { return nil })
	if err == nil {
		t.Fatalf("empty options must error")
	}
}

func TestRun_loadPrivateKeyError(t *testing.T) {
	// 写一个故意不合法的 PEM,让 protocol.LoadPrivateKey 解析失败,
	// Run 应返回带 "load private key" 前缀的包装错。
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(bad, []byte("not a pem"), 0o600); err != nil {
		t.Fatalf("seed bad pem: %v", err)
	}
	err := Run(context.Background(), Options{
		AccessToken:    "t",
		PrivateKeyPath: bad,
		VIN:            "v",
	}, func(*vehicle.Vehicle) error { return nil })
	if err == nil {
		t.Fatalf("invalid PEM must error")
	}
	if !strings.Contains(err.Error(), "load private key") {
		t.Errorf("err should mention load private key: %v", err)
	}
}

func TestUserAgent_isShort(t *testing.T) {
	// 与 internal/tesla 中同名常量保持一致,且不能包含 "(+http"
	// (这是触发 Akamai WAF 的 fingerprint;参见 docs cn-notes)。
	if strings.Contains(UserAgent, "(+http") {
		t.Errorf("UserAgent must not contain '(+http' substring (Akamai trip wire): %q", UserAgent)
	}
	if !strings.HasPrefix(UserAgent, "tesla-cli") {
		t.Errorf("UserAgent should start with 'tesla-cli', got %q", UserAgent)
	}
}
