package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ----------------- pkce.go -----------------

func TestNewPKCE_lengthAndCharset(t *testing.T) {
	pk, err := NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE: %v", err)
	}
	if pk.Method != "S256" {
		t.Errorf("Method should be S256, got %q", pk.Method)
	}
	if got := len(pk.Verifier); got != 43 {
		t.Errorf("Verifier should be 43 chars, got %d", got)
	}
	if len(pk.Challenge) != 43 {
		t.Errorf("Challenge should be 43 chars, got %d", len(pk.Challenge))
	}
	for _, ch := range pk.Verifier + pk.Challenge {
		ok := (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_'
		if !ok {
			t.Errorf("non-base64url char in PKCE: %q", ch)
		}
	}
}

func TestNewPKCE_uniqueAcrossCalls(t *testing.T) {
	a, _ := NewPKCE()
	b, _ := NewPKCE()
	if a.Verifier == b.Verifier {
		t.Fatalf("PKCE verifiers should be unique across calls")
	}
}

// ----------------- tokens.go -----------------

func TestToken_Expired(t *testing.T) {
	cases := []struct {
		name string
		tok  *Token
		want bool
	}{
		{"nil → expired", nil, true},
		{"expired", &Token{ExpiresAt: time.Now().Add(-1 * time.Minute)}, true},
		{"about-to-expire (within 30s window)", &Token{ExpiresAt: time.Now().Add(10 * time.Second)}, true},
		{"valid", &Token{ExpiresAt: time.Now().Add(1 * time.Hour)}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tok.Expired(); got != tc.want {
				t.Errorf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestToken_SafeView_masksAndIncludesMeta(t *testing.T) {
	tok := &Token{
		AccessToken:  "abcdefgh1234567890XYZ",
		RefreshToken: "RT-12345678901234567890",
		TokenType:    "Bearer",
		Scopes:       []string{"openid", "offline_access"},
		ExpiresAt:    time.Date(2026, 4, 26, 15, 0, 0, 0, time.UTC),
		ObtainedAt:   time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC),
	}
	v := tok.SafeView()
	at, _ := v["access_token_masked"].(string)
	if !strings.Contains(at, "***") {
		t.Errorf("access_token should be masked: %q", at)
	}
	if strings.Contains(at, tok.AccessToken) {
		t.Errorf("masked access_token must not contain full secret: %q", at)
	}
	if v["token_type"] != "Bearer" {
		t.Errorf("token_type lost: %v", v["token_type"])
	}
	if exp, _ := v["expires_at"].(string); exp != "2026-04-26T15:00:00Z" {
		t.Errorf("expires_at not RFC3339: %q", exp)
	}
}

func TestToken_SafeView_nilSafe(t *testing.T) {
	var tok *Token
	if got := tok.SafeView(); got != nil {
		t.Errorf("nil token should yield nil view, got %+v", got)
	}
}

func TestMask_shortStringsFullyMasked(t *testing.T) {
	if got := mask("abc"); got != "***" {
		t.Errorf("short string should be fully masked, got %q", got)
	}
	if got := mask(""); got != "***" {
		t.Errorf("empty string should be fully masked, got %q", got)
	}
}

func TestMask_longStringsKeepEdges(t *testing.T) {
	got := mask("0123456789ABCDEF")
	if !strings.HasPrefix(got, "0123") || !strings.HasSuffix(got, "CDEF") {
		t.Errorf("mask should keep first/last 4: %q", got)
	}
}

// ----------------- store.go -----------------

func newSampleToken() *Token {
	return &Token{
		AccessToken:  "AT-1234567890",
		RefreshToken: "RT-0987654321",
		TokenType:    "Bearer",
		Scopes:       []string{"openid", "offline_access"},
		ExpiresAt:    time.Date(2026, 4, 26, 15, 0, 0, 0, time.UTC),
		ObtainedAt:   time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC),
	}
}

func TestFileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)

	tok := newSampleToken()
	if err := s.Save("default", tok); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.Load("default")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AccessToken != tok.AccessToken {
		t.Errorf("access_token mismatch")
	}
	if !got.ExpiresAt.Equal(tok.ExpiresAt) {
		t.Errorf("expires_at lost roundtrip: %v vs %v", got.ExpiresAt, tok.ExpiresAt)
	}
}

func TestFileStore_FilePermissions0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions not enforced on Windows")
	}
	dir := t.TempDir()
	s := NewFileStore(dir)
	if err := s.Save("default", newSampleToken()); err != nil {
		t.Fatalf("save: %v", err)
	}
	st, err := os.Stat(filepath.Join(dir, "profiles", "default.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("token file should be 0600, got %o", mode)
	}
}

func TestFileStore_LoadMissingReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)
	_, err := s.Load("ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_DeleteIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore(dir)
	if err := s.Delete("ghost"); err != nil {
		t.Fatalf("delete on missing should be idempotent: %v", err)
	}
	if err := s.Save("real", newSampleToken()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.Delete("real"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Load("real"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete, load should be ErrNotFound, got %v", err)
	}
}

func TestFileStore_SaveNilRejected(t *testing.T) {
	s := NewFileStore(t.TempDir())
	if err := s.Save("p", nil); err == nil {
		t.Fatalf("save nil should error")
	}
}

// ----------------- oauth.go (纯函数) -----------------

func TestSplitScopes_table(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"openid", []string{"openid"}},
		{"openid offline_access", []string{"openid", "offline_access"}},
		{"  openid   offline_access  ", []string{"openid", "offline_access"}},
	}
	for _, tc := range cases {
		got := splitScopes(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("scopes %q: got %d items, want %d", tc.in, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("scopes %q: idx %d got %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestBuildAuthorizeURL_includesAllParams(t *testing.T) {
	pk := PKCE{Verifier: "v", Challenge: "c", Method: "S256"}
	opts := LoginOptions{
		ClientID:    "cid",
		RedirectURI: "http://localhost:8765/callback",
		Scopes:      []string{"openid", "offline_access"},
		Audience:    "https://api.example.com",
	}
	got := buildAuthorizeURL("https://auth.example.com/authorize", opts, pk, "STATE")

	required := []string{
		"client_id=cid",
		"response_type=code",
		"code_challenge=c",
		"code_challenge_method=S256",
		"state=STATE",
		"audience=",
		"scope=",
		"redirect_uri=",
	}
	for _, r := range required {
		if !strings.Contains(got, r) {
			t.Errorf("authorize URL missing %q: %s", r, got)
		}
	}
}

func TestBuildAuthorizeURL_handlesPreexistingQuery(t *testing.T) {
	pk := PKCE{Method: "S256"}
	url := buildAuthorizeURL("https://auth.example.com/authorize?x=1",
		LoginOptions{ClientID: "c", RedirectURI: "r"}, pk, "s")
	if !strings.Contains(url, "x=1&") {
		t.Errorf("should keep existing ?x=1 with & separator: %s", url)
	}
}

func TestRandomState_lengthClamped(t *testing.T) {
	got, err := randomState(24)
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	if len(got) != 24 {
		t.Errorf("expected 24 chars, got %d (%q)", len(got), got)
	}
}
