package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/wmango/tesla-cli/internal/tesla"
)

// LoginOptions 是车主 OAuth 登录所需输入。
//
// ClientSecret 仅当应用是 confidential client 时需要;
// Tesla 当前发的应用通常是 confidential。
type LoginOptions struct {
	Region       string        // na | eu | cn
	ClientID     string        // 必需
	ClientSecret string        // 可选
	RedirectURI  string        // 必需,需与 developer.tesla.com 注册一致
	Scopes       []string      // 例:openid offline_access vehicle_device_data
	Audience     string        // 通常是 Fleet API base(按 region)
	OpenBrowser  bool          // 是否自动开浏览器
	BindAddr     string        // 本地回调监听地址,默认 127.0.0.1
	Timeout      time.Duration // 用户授权超时
	Notify       io.Writer     // 通知输出器(一般是 stderr)

	// Manual=true 时跳过本地回调 server,改为从 Stdin 读"被重定向后的完整 URL"
	// (或 "code state" 字符串)。当 RedirectURI 不指向 localhost/127.0.0.1
	// 时由 Login() 自动开启。
	Manual bool
	Stdin  io.Reader // manual 模式下的输入源,默认 os.Stdin
}

// RefreshOptions 是刷新 token 所需输入。
type RefreshOptions struct {
	Region       string
	ClientID     string
	ClientSecret string
	RefreshToken string
	Scopes       []string // 可选
}

// Login 完成 authorization_code + PKCE 流,返回新令牌。
func Login(ctx context.Context, opts LoginOptions) (*Token, error) {
	if opts.ClientID == "" {
		return nil, errors.New("oauth: ClientID required")
	}
	if opts.RedirectURI == "" {
		return nil, errors.New("oauth: RedirectURI required")
	}
	ep, err := tesla.EndpointsFor(opts.Region)
	if err != nil {
		return nil, fmt.Errorf("oauth: %w", err)
	}
	pk, err := NewPKCE()
	if err != nil {
		return nil, err
	}
	state, err := randomState(24)
	if err != nil {
		return nil, fmt.Errorf("oauth: state: %w", err)
	}

	parsed, err := url.Parse(opts.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("oauth: parse redirect_uri: %w", err)
	}

	// 自动检测:redirect_uri 不指向 localhost/127.0.0.1 时,本地 callback server
	// 永远收不到回调 — 切到 manual paste 模式。
	if !opts.Manual && !isLoopbackHost(parsed.Hostname()) {
		opts.Manual = true
	}
	if opts.Manual {
		return loginManual(ctx, ep, opts, pk, state)
	}
	bind := opts.BindAddr
	if bind == "" {
		bind = "127.0.0.1"
	}
	listenAddr := bind + ":" + parsed.Port()
	if parsed.Port() == "" {
		listenAddr = bind + ":0"
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("oauth: listen %s: %w", listenAddr, err)
	}
	defer listener.Close()

	authURL := buildAuthorizeURL(ep.AuthorizeURL, opts, pk, state)

	cbCh := make(chan callbackResult, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           callbackHandler(parsed.Path, state, cbCh),
	}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	notify := opts.Notify
	if notify == nil {
		notify = io.Discard
	}
	fmt.Fprintf(notify, "Open this URL to authorize:\n  %s\n", authURL)
	if opts.OpenBrowser {
		_ = openBrowser(authURL)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, errors.New("oauth: timeout waiting for browser callback")
	case res := <-cbCh:
		if res.err != nil {
			return nil, res.err
		}
		return exchangeCode(ctx, ep.TokenURL, opts, pk.Verifier, res.code)
	}
}

// Refresh 用 refresh_token 拿新 access_token。
func Refresh(ctx context.Context, opts RefreshOptions) (*Token, error) {
	if opts.RefreshToken == "" {
		return nil, errors.New("oauth: RefreshToken required")
	}
	ep, err := tesla.EndpointsFor(opts.Region)
	if err != nil {
		return nil, fmt.Errorf("oauth: %w", err)
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", opts.ClientID)
	if opts.ClientSecret != "" {
		form.Set("client_secret", opts.ClientSecret)
	}
	form.Set("refresh_token", opts.RefreshToken)
	if len(opts.Scopes) > 0 {
		form.Set("scope", strings.Join(opts.Scopes, " "))
	}
	return postToken(ctx, ep.TokenURL, form)
}

type callbackResult struct {
	code string
	err  error
}

func callbackHandler(expectedPath, expectedState string, ch chan<- callbackResult) http.Handler {
	mux := http.NewServeMux()
	path := expectedPath
	if path == "" {
		path = "/callback"
	}
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			writeBrowserResult(w, false, "Authorization error: "+e)
			ch <- callbackResult{err: fmt.Errorf("oauth: provider error: %s (%s)", e, q.Get("error_description"))}
			return
		}
		if got := q.Get("state"); got != expectedState {
			writeBrowserResult(w, false, "State mismatch")
			ch <- callbackResult{err: errors.New("oauth: state mismatch (possible CSRF)")}
			return
		}
		code := q.Get("code")
		if code == "" {
			writeBrowserResult(w, false, "Missing code")
			ch <- callbackResult{err: errors.New("oauth: callback missing code")}
			return
		}
		writeBrowserResult(w, true, "Authorization complete. You can close this tab.")
		ch <- callbackResult{code: code}
	})
	return mux
}

func writeBrowserResult(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	status := "OK"
	if !ok {
		status = "ERROR"
		w.WriteHeader(http.StatusBadRequest)
	}
	_, _ = fmt.Fprintf(w,
		"<html><body style='font-family:sans-serif;padding:2rem'><h1>tesla-cli — %s</h1><p>%s</p></body></html>",
		status, msg)
}

func buildAuthorizeURL(base string, o LoginOptions, pk PKCE, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", o.ClientID)
	q.Set("redirect_uri", o.RedirectURI)
	q.Set("scope", strings.Join(o.Scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", pk.Challenge)
	q.Set("code_challenge_method", pk.Method)
	q.Set("prompt", "login")
	if o.Audience != "" {
		q.Set("audience", o.Audience)
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + q.Encode()
}

func exchangeCode(ctx context.Context, tokenURL string, o LoginOptions, verifier, code string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", o.ClientID)
	if o.ClientSecret != "" {
		form.Set("client_secret", o.ClientSecret)
	}
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", o.RedirectURI)
	if o.Audience != "" {
		form.Set("audience", o.Audience)
	}
	return postToken(ctx, tokenURL, form)
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token"`
}

func postToken(ctx context.Context, tokenURL string, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth: build req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	tesla.SetUA(req)

	client := tesla.NewHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: post token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("oauth: token endpoint %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("oauth: parse token: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, errors.New("oauth: empty access_token in response")
	}
	now := time.Now().UTC()
	t := &Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Scopes:       splitScopes(tr.Scope),
		ObtainedAt:   now,
		ExpiresAt:    now.Add(time.Duration(tr.ExpiresIn) * time.Second),
	}
	if t.TokenType == "" {
		t.TokenType = "Bearer"
	}
	return t, nil
}

func splitScopes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// randomState 用 PKCE verifier 当随机源,截前 n 字符。
func randomState(n int) (string, error) {
	pk, err := NewPKCE()
	if err != nil {
		return "", err
	}
	if len(pk.Verifier) < n {
		return pk.Verifier, nil
	}
	return pk.Verifier[:n], nil
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

// isLoopbackHost 判断主机名是否是回环地址(本地 callback 可达)。
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "":
		return true
	}
	return false
}

// loginManual 走"用户复制重定向 URL 粘贴回来"流程。
// redirect_uri 是生产域名时(本地接不到 callback),由 Login 自动切到这里。
func loginManual(ctx context.Context, ep tesla.Endpoints, opts LoginOptions, pk PKCE, state string) (*Token, error) {
	authURL := buildAuthorizeURL(ep.AuthorizeURL, opts, pk, state)

	notify := opts.Notify
	if notify == nil {
		notify = io.Discard
	}
	fmt.Fprintf(notify, "\n=== Manual paste mode ===\n")
	fmt.Fprintf(notify, "redirect_uri (%s) is not loopback; CLI cannot receive the callback locally.\n\n", opts.RedirectURI)
	fmt.Fprintf(notify, "1) Open this URL in your browser and complete authorization:\n\n  %s\n\n", authURL)
	fmt.Fprintf(notify, "2) Your browser will be redirected to:\n     %s?code=...&state=...\n\n", opts.RedirectURI)
	fmt.Fprintf(notify, "3) Paste the FULL redirected URL (or just \"<code> <state>\" separated by space):\n> ")

	if opts.OpenBrowser {
		_ = openBrowser(authURL)
	}

	in := opts.Stdin
	if in == nil {
		in = os.Stdin
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return nil, fmt.Errorf("oauth: read paste: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, errors.New("oauth: empty paste input")
	}

	code, gotState, err := parsePastedCallback(line)
	if err != nil {
		return nil, err
	}
	if gotState != "" && gotState != state {
		return nil, fmt.Errorf("oauth: state mismatch (CSRF?): expected %q got %q", state, gotState)
	}
	if code == "" {
		return nil, errors.New("oauth: no code in pasted input")
	}

	// 让上层调用方在异常 ctx 时能抢先返回。
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	return exchangeCode(ctx, ep.TokenURL, opts, pk.Verifier, code)
}

// parsePastedCallback 接受三种形态:
//
//	a) 完整 URL:           https://example.com/code?code=ABC&state=XYZ
//	b) query 片段:         ?code=ABC&state=XYZ
//	c) 空格分隔:           ABC XYZ
func parsePastedCallback(line string) (code, state string, err error) {
	switch {
	case strings.Contains(line, "://") || strings.HasPrefix(line, "?"):
		raw := line
		if strings.HasPrefix(raw, "?") {
			raw = "https://x" + raw // 让 url.Parse 能消费
		}
		u, perr := url.Parse(raw)
		if perr != nil {
			return "", "", fmt.Errorf("oauth: parse pasted URL: %w", perr)
		}
		q := u.Query()
		if e := q.Get("error"); e != "" {
			return "", "", fmt.Errorf("oauth: provider error: %s (%s)", e, q.Get("error_description"))
		}
		return q.Get("code"), q.Get("state"), nil
	default:
		parts := strings.Fields(line)
		if len(parts) == 0 {
			return "", "", errors.New("oauth: empty input")
		}
		code = parts[0]
		if len(parts) >= 2 {
			state = parts[1]
		}
		return code, state, nil
	}
}
