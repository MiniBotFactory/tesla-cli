package auth

import (
	"context"
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
func TestFixturesDoNotLeakSecrets(t *testing.T) {
	forbidden := []string{
		"LRW0000000000000",  // 真 VIN
		"f2ec4168-78cf-495b", // 真 client_id 前缀
		"f8cc0757-4fc6-4f59", // 真 account_id 前缀
		"ta-secret.",         // 真 client_secret 前缀
		"Test Car",              // 真 display_name
		"STE20240907",        // 真 wall_connector serial
		"9876543210987654",   // 真 vehicle_id
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
