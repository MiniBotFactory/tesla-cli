package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wmango/tesla-cli/internal/tesla"
)

// ----------------- isLoopbackHost -----------------

func TestIsLoopbackHost_table(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"", true}, // 空 = 同主机
		{"example.com", false},
		{"t.example.cn", false},
		{"127.0.0.2", false}, // 严格匹配,不放过子段
		{"localhost.evil.com", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.host, func(t *testing.T) {
			if got := isLoopbackHost(tc.host); got != tc.want {
				t.Errorf("isLoopbackHost(%q): got %v want %v", tc.host, got, tc.want)
			}
		})
	}
}

// ----------------- parsePastedCallback -----------------

func TestParsePastedCallback_fullURL(t *testing.T) {
	code, state, err := parsePastedCallback(
		"https://example.com/code?code=ABC123&state=XYZ789&extra=ignored")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "ABC123" {
		t.Errorf("code: %q", code)
	}
	if state != "XYZ789" {
		t.Errorf("state: %q", state)
	}
}

func TestParsePastedCallback_queryFragment(t *testing.T) {
	code, state, err := parsePastedCallback("?code=Q1&state=Q2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "Q1" || state != "Q2" {
		t.Errorf("got code=%q state=%q", code, state)
	}
}

func TestParsePastedCallback_spaceSeparated(t *testing.T) {
	code, state, err := parsePastedCallback("THE-CODE THE-STATE")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "THE-CODE" || state != "THE-STATE" {
		t.Errorf("got code=%q state=%q", code, state)
	}
}

func TestParsePastedCallback_codeOnly(t *testing.T) {
	code, state, err := parsePastedCallback("JUST-CODE")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "JUST-CODE" {
		t.Errorf("code: %q", code)
	}
	if state != "" {
		t.Errorf("state should be empty, got %q", state)
	}
}

func TestParsePastedCallback_providerError(t *testing.T) {
	_, _, err := parsePastedCallback(
		"https://example.com/code?error=access_denied&error_description=user+canceled")
	if err == nil {
		t.Fatalf("expected provider error")
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("err should mention access_denied: %v", err)
	}
}

func TestParsePastedCallback_emptyInput(t *testing.T) {
	_, _, err := parsePastedCallback("")
	if err == nil {
		t.Fatalf("expected error for empty input")
	}
}

func TestParsePastedCallback_invalidURL(t *testing.T) {
	_, _, err := parsePastedCallback("https://%zz/code?code=x")
	if err == nil {
		t.Fatalf("expected error for malformed URL")
	}
}

// ----------------- loginManual -----------------

// fakeTokenServer 接收 form-encoded POST,断言 grant_type=authorization_code
// 与 code/code_verifier/redirect_uri 都被传上来,然后回一个 token JSON。
func fakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.PostFormValue("grant_type"); got != "authorization_code" {
			t.Errorf("grant_type: %q", got)
		}
		if got := r.PostFormValue("code"); got != "THE-CODE" {
			t.Errorf("code passed-through: %q", got)
		}
		if got := r.PostFormValue("code_verifier"); got == "" {
			t.Errorf("code_verifier missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "access_token":"AT-OK","refresh_token":"RT-OK","token_type":"Bearer",
            "expires_in":3600,"scope":"openid offline_access"
        }`))
	}))
}

func fakeEndpoints(tokenURL string) tesla.Endpoints {
	return tesla.Endpoints{
		Region:       "test",
		AuthorizeURL: "https://auth.example/authorize",
		TokenURL:     tokenURL,
		APIBase:      "https://api.example",
	}
}

func TestLoginManual_happyPath_fullURLPaste(t *testing.T) {
	tokenSrv := fakeTokenServer(t)
	defer tokenSrv.Close()

	pk, _ := NewPKCE()
	state := "FIXED-STATE"

	pasted := "https://prod.example.com/cb?code=THE-CODE&state=" + state + "\n"
	var notify bytes.Buffer
	tok, err := loginManual(context.Background(),
		fakeEndpoints(tokenSrv.URL),
		LoginOptions{
			ClientID:    "cid",
			RedirectURI: "https://prod.example.com/cb",
			Scopes:      []string{"openid"},
			Notify:      &notify,
			Stdin:       strings.NewReader(pasted),
		},
		pk, state)
	if err != nil {
		t.Fatalf("loginManual: %v", err)
	}
	if tok.AccessToken != "AT-OK" {
		t.Errorf("access_token mismatch: %q", tok.AccessToken)
	}
	if tok.RefreshToken != "RT-OK" {
		t.Errorf("refresh_token mismatch: %q", tok.RefreshToken)
	}
	if !strings.Contains(notify.String(), "Manual paste mode") {
		t.Errorf("notify should announce manual paste mode, got %q", notify.String())
	}
	if !strings.Contains(notify.String(), "client_id=cid") {
		t.Errorf("notify should print authorize URL with client_id; got %q", notify.String())
	}
}

func TestLoginManual_stateMismatchRejected(t *testing.T) {
	tokenSrv := fakeTokenServer(t)
	defer tokenSrv.Close()

	pk, _ := NewPKCE()
	pasted := "https://prod.example.com/cb?code=THE-CODE&state=WRONG\n"
	_, err := loginManual(context.Background(),
		fakeEndpoints(tokenSrv.URL),
		LoginOptions{
			ClientID:    "cid",
			RedirectURI: "https://prod.example.com/cb",
			Stdin:       strings.NewReader(pasted),
		},
		pk, "EXPECTED-STATE")
	if err == nil {
		t.Fatalf("state mismatch must error")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("err should mention state mismatch: %v", err)
	}
}

func TestLoginManual_emptyPasteRejected(t *testing.T) {
	pk, _ := NewPKCE()
	_, err := loginManual(context.Background(),
		fakeEndpoints("https://unused"),
		LoginOptions{
			ClientID:    "cid",
			RedirectURI: "https://prod.example.com/cb",
			Stdin:       strings.NewReader("\n"),
		},
		pk, "S")
	if err == nil {
		t.Fatalf("empty input must error")
	}
	if !strings.Contains(err.Error(), "empty paste") {
		t.Errorf("err should mention empty paste: %v", err)
	}
}

func TestLoginManual_codeStateSpaceForm(t *testing.T) {
	tokenSrv := fakeTokenServer(t)
	defer tokenSrv.Close()

	pk, _ := NewPKCE()
	pasted := "THE-CODE FIXED\n"
	tok, err := loginManual(context.Background(),
		fakeEndpoints(tokenSrv.URL),
		LoginOptions{
			ClientID:    "cid",
			RedirectURI: "https://prod.example.com/cb",
			Stdin:       strings.NewReader(pasted),
		},
		pk, "FIXED")
	if err != nil {
		t.Fatalf("loginManual: %v", err)
	}
	if tok.AccessToken != "AT-OK" {
		t.Errorf("access_token mismatch: %q", tok.AccessToken)
	}
}

func TestLoginManual_ctxAlreadyCancelled(t *testing.T) {
	pk, _ := NewPKCE()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pasted := "THE-CODE S\n"
	_, err := loginManual(ctx,
		fakeEndpoints("https://unused"),
		LoginOptions{
			ClientID:    "cid",
			RedirectURI: "https://prod.example.com/cb",
			Stdin:       strings.NewReader(pasted),
		},
		pk, "S")
	if err == nil {
		t.Fatalf("cancelled ctx should be propagated")
	}
}

// ----------------- Login() 自动切 manual 检测 -----------------

func TestLogin_autoSwitchesToManualForNonLoopbackRedirect(t *testing.T) {
	// Login() 内部 EndpointsFor("na") 拿到的真实 token URL 我们够不到,
	// 但因为 redirect_uri 是 https://prod.example.com/cb(非 loopback),
	// Login 会自动切到 loginManual。我们故意粘错 state 让 manual 路径
	// 在 state 校验阶段就拒绝 — 这同时证明:
	//   1. 流程已进入 manual 路径(否则会 net.Listen 阻塞)
	//   2. state CSRF 校验生效
	pasted := "https://prod.example.com/cb?code=X&state=WRONG\n"
	_, err := Login(context.Background(), LoginOptions{
		Region:      "na",
		ClientID:    "cid",
		RedirectURI: "https://prod.example.com/cb",
		Scopes:      []string{"openid"},
		Stdin:       strings.NewReader(pasted),
	})
	if err == nil {
		t.Fatalf("should error (state mismatch from auto-manual path)")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("err should be a manual-path state mismatch (proves auto-switch): %v", err)
	}
}
