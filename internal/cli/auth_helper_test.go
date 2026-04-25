package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wmango/tesla-cli/internal/auth"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
)

// writeTestToken 直接落盘一个 token 到 store,绕开真实 OAuth flow。
func writeTestToken(t *testing.T, baseDir, profile string, expiresAt time.Time, refresh string) {
	t.Helper()
	store := auth.NewFileStore(baseDir)
	tok := &auth.Token{
		AccessToken:  "AT-1234567890",
		RefreshToken: refresh,
		TokenType:    "Bearer",
		Scopes:       []string{"openid", "offline_access"},
		ExpiresAt:    expiresAt,
		ObtainedAt:   time.Now().UTC(),
	}
	if err := store.Save(profile, tok); err != nil {
		t.Fatalf("save token: %v", err)
	}
}

func makeCfg(baseDir string) config.Config {
	c := config.DefaultConfig()
	c.BaseDir = baseDir
	c.Profile = "default"
	return c
}

func TestEnsureValidToken_returnsTokenWhenFresh(t *testing.T) {
	dir := t.TempDir()
	writeTestToken(t, dir, "default", time.Now().Add(1*time.Hour), "RT-X")

	tok, err := ensureValidToken(context.Background(), makeCfg(dir))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if tok.AccessToken != "AT-1234567890" {
		t.Errorf("token mismatch: %+v", tok)
	}
}

func TestEnsureValidToken_noTokenReturnsAuthCode(t *testing.T) {
	dir := t.TempDir()
	_, err := ensureValidToken(context.Background(), makeCfg(dir))
	if err == nil {
		t.Fatalf("expected error when no token")
	}
	if errs.CodeOf(err) != errs.ExitAuth {
		t.Errorf("want ExitAuth, got %d", errs.CodeOf(err))
	}
}

func TestEnsureValidToken_expiredWithoutRefreshReturnsAuth(t *testing.T) {
	dir := t.TempDir()
	writeTestToken(t, dir, "default", time.Now().Add(-1*time.Hour), "")

	_, err := ensureValidToken(context.Background(), makeCfg(dir))
	if err == nil {
		t.Fatalf("expected error for expired+no-refresh")
	}
	if errs.CodeOf(err) != errs.ExitAuth {
		t.Errorf("want ExitAuth, got %d", errs.CodeOf(err))
	}
}

func TestEnsureValidToken_expiredButHasRefreshNeedsConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestToken(t, dir, "default", time.Now().Add(-1*time.Hour), "RT-X")

	// 没写 config.toml → 应落到 ExitConfig 分支
	_, err := ensureValidToken(context.Background(), makeCfg(dir))
	if err == nil {
		t.Fatalf("expected config error")
	}
	if errs.CodeOf(err) != errs.ExitConfig {
		t.Errorf("want ExitConfig, got %d (%v)", errs.CodeOf(err), err)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "b", "c"); got != "b" {
		t.Errorf("first non-empty should be b, got %q", got)
	}
	if got := firstNonEmpty("", "", ""); got != "" {
		t.Errorf("all empty should yield empty, got %q", got)
	}
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("a should win, got %q", got)
	}
}

func TestConfigTOML_roundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	in := map[string]string{
		"client_id":     "cid",
		"client_secret": "sec",
		"domain":        "my.example.com",
		"region":        "na",
		"redirect_uri":  "http://localhost:8765/cb",
		"scopes":        "openid offline_access",
	}
	if err := writeConfigTOML(path, in, true); err != nil {
		t.Fatalf("write: %v", err)
	}
	if st, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	} else if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("config.toml should be 0600, got %o", mode)
	}
	out, err := loadConfigTOML(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for k, v := range in {
		if out[k] != v {
			t.Errorf("key %q lost roundtrip: want %q, got %q", k, v, out[k])
		}
	}
}

func TestConfigTOML_writeRejectsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := writeConfigTOML(path, map[string]string{"client_id": "x"}, true); err != nil {
		t.Fatalf("first write: %v", err)
	}
	err := writeConfigTOML(path, map[string]string{"client_id": "y"}, false)
	if err == nil {
		t.Fatalf("second write without force should fail")
	}
}

func TestValidKey(t *testing.T) {
	if !validKey("client_id") {
		t.Errorf("client_id should be valid")
	}
	if validKey("nonsense") {
		t.Errorf("nonsense should be invalid")
	}
}

func TestRedactSensitive(t *testing.T) {
	got := redactSensitive(map[string]string{
		"client_id":     "cid",
		"client_secret": "real-secret",
		"domain":        "x",
	})
	if got["client_id"] != "cid" {
		t.Errorf("client_id should pass through, got %q", got["client_id"])
	}
	if got["client_secret"] != "***" {
		t.Errorf("client_secret should be masked, got %q", got["client_secret"])
	}
}

func TestRedactSensitive_emptySecretNotMasked(t *testing.T) {
	got := redactSensitive(map[string]string{"client_secret": ""})
	if got["client_secret"] == "***" {
		t.Errorf("empty secret should not be shown as ***")
	}
}

func TestEnsureValidToken_neverReturnsBothNonNil(t *testing.T) {
	dir := t.TempDir()
	tok, err := ensureValidToken(context.Background(), makeCfg(dir))
	if err == nil && tok == nil {
		t.Fatalf("invariant violation: both nil")
	}
	if err != nil && tok != nil {
		t.Fatalf("invariant violation: both non-nil")
	}
}

func TestResolveVINArg_priority(t *testing.T) {
	cfg := config.DefaultConfig()

	// arg wins
	if vin, err := resolveVINArg([]string{"VIN-A"}, cfg); err != nil || vin != "VIN-A" {
		t.Errorf("arg should win: vin=%q err=%v", vin, err)
	}

	// fallback to cfg.VIN
	cfg.VIN = "VIN-FROM-CFG"
	if vin, err := resolveVINArg(nil, cfg); err != nil || vin != "VIN-FROM-CFG" {
		t.Errorf("cfg fallback failed: vin=%q err=%v", vin, err)
	}

	// neither -> usage error
	cfg.VIN = ""
	if _, err := resolveVINArg(nil, cfg); err == nil {
		t.Errorf("expected usage error when neither arg nor cfg")
	}
}
