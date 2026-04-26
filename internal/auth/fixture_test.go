package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPostToken_realCNTokenShape 用 testdata/token.json 重放真实 Tesla CN
// /oauth2/v3/token 的成功响应结构,确保字段映射不漂移。
//
// fixture 来源:实测 2026-04-26 抓取的 client_credentials grant 响应,
// 已脱敏(access_token 替换为 "<REDACTED.JWT>")。
func TestPostToken_realCNTokenShape(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "token.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	tok, err := postToken(context.Background(), srv.URL, form)
	if err != nil {
		t.Fatalf("postToken on fixture: %v", err)
	}

	if tok.AccessToken != "<REDACTED.JWT>" {
		t.Errorf("access_token mismatch: %q", tok.AccessToken)
	}
	if tok.TokenType != "Bearer" {
		t.Errorf("token_type expected Bearer, got %q", tok.TokenType)
	}
	delta := tok.ExpiresAt.Sub(tok.ObtainedAt).Seconds()
	if delta < 28790 || delta > 28810 {
		t.Errorf("expires_in not honored: delta=%.0fs (want ~28800)", delta)
	}
	// scope 字段缺失 → Scopes 应为 nil(CN client_credentials 响应不带 scope)
	if tok.Scopes != nil {
		t.Errorf("CN cc grant has no scope; expected nil, got %v", tok.Scopes)
	}
}

// TestFixturesDoNotLeakSecrets 静态扫描 testdata,确保未来 fixture 不漏敏。
//
// 真值黑名单从仓库根 .forbidden_strings 读取(本地 only,.gitignore 排除)。
// 文件不存在时 t.Skip,以便 CI / fork / 公仓库不持有真值字符串。
func TestFixturesDoNotLeakSecrets(t *testing.T) {
	forbidden, err := loadForbiddenStrings()
	if err != nil {
		t.Skipf("no .forbidden_strings (%v); skipping leak-scan", err)
	}
	matches, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, p := range matches {
		data, _ := os.ReadFile(p)
		s := string(data)
		for _, bad := range forbidden {
			if strings.Contains(s, bad) {
				t.Errorf("%s: contains forbidden %q (sanitization regression!)", p, bad)
			}
		}
	}
}

// loadForbiddenStrings 从仓库根 .forbidden_strings 读取每行一个真值子串。
// 注释行(# 开头)与空行被忽略。
func loadForbiddenStrings() ([]string, error) {
	// 测试 cwd 是 internal/auth;仓库根上溯两级。
	raw, err := os.ReadFile(filepath.Join("..", "..", ".forbidden_strings"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil, errors.New(".forbidden_strings has no entries")
	}
	return out, nil
}
