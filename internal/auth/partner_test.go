package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ------------- postToken (covers oauth.go core) -------------

func TestPostToken_successParsesAllFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type: %q", ct)
		}
		_, _ = w.Write([]byte(`{
            "access_token":"AT-X","refresh_token":"RT-Y","token_type":"Bearer",
            "expires_in":3600,"scope":"openid offline_access","id_token":"id"
        }`))
	}))
	defer srv.Close()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	tok, err := postToken(context.Background(), srv.URL, form)
	if err != nil {
		t.Fatalf("postToken: %v", err)
	}
	if tok.AccessToken != "AT-X" || tok.RefreshToken != "RT-Y" || tok.TokenType != "Bearer" {
		t.Errorf("fields: %+v", tok)
	}
	if got := tok.ExpiresAt.Sub(tok.ObtainedAt); got < time.Hour-2*time.Second || got > time.Hour+2*time.Second {
		t.Errorf("expires_in not honored: delta=%v", got)
	}
	if len(tok.Scopes) != 2 || tok.Scopes[0] != "openid" {
		t.Errorf("scopes: %v", tok.Scopes)
	}
}

func TestPostToken_emptyAccessTokenRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":""}`))
	}))
	defer srv.Close()

	_, err := postToken(context.Background(), srv.URL, url.Values{})
	if err == nil {
		t.Fatalf("empty access_token should error")
	}
}

func TestPostToken_4xxIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()
	_, err := postToken(context.Background(), srv.URL, url.Values{})
	if err == nil {
		t.Fatalf("4xx should error")
	}
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("error should include body: %v", err)
	}
}

func TestPostToken_invalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	_, err := postToken(context.Background(), srv.URL, url.Values{})
	if err == nil {
		t.Fatalf("invalid JSON should error")
	}
}

func TestPostToken_defaultsTokenTypeWhenMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":60}`))
	}))
	defer srv.Close()
	tok, err := postToken(context.Background(), srv.URL, url.Values{})
	if err != nil {
		t.Fatalf("postToken: %v", err)
	}
	if tok.TokenType != "Bearer" {
		t.Errorf("default TokenType should be Bearer, got %q", tok.TokenType)
	}
}

// ------------- callbackHandler (oauth.go callback path) -------------

func TestCallbackHandler_successDeliversCode(t *testing.T) {
	ch := make(chan callbackResult, 1)
	h := callbackHandler("/callback", "STATE-ABC", ch)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/callback?code=THE-CODE&state=STATE-ABC")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	res := <-ch
	if res.err != nil {
		t.Fatalf("unexpected err: %v", res.err)
	}
	if res.code != "THE-CODE" {
		t.Errorf("code: %q", res.code)
	}
}

func TestCallbackHandler_stateMismatch(t *testing.T) {
	ch := make(chan callbackResult, 1)
	h := callbackHandler("/callback", "EXPECTED", ch)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/callback?code=X&state=WRONG")
	resp.Body.Close()
	res := <-ch
	if res.err == nil {
		t.Fatalf("state mismatch should produce err")
	}
}

func TestCallbackHandler_providerError(t *testing.T) {
	ch := make(chan callbackResult, 1)
	h := callbackHandler("/callback", "S", ch)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/callback?error=access_denied&error_description=user")
	resp.Body.Close()
	res := <-ch
	if res.err == nil || !strings.Contains(res.err.Error(), "access_denied") {
		t.Fatalf("expected access_denied: %v", res.err)
	}
}

func TestCallbackHandler_missingCode(t *testing.T) {
	ch := make(chan callbackResult, 1)
	h := callbackHandler("/callback", "S", ch)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/callback?state=S")
	resp.Body.Close()
	res := <-ch
	if res.err == nil {
		t.Fatalf("missing code should error")
	}
}

func TestCallbackHandler_defaultPathWhenEmpty(t *testing.T) {
	ch := make(chan callbackResult, 1)
	h := callbackHandler("", "S", ch)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/callback?code=C&state=S")
	resp.Body.Close()
	res := <-ch
	if res.code != "C" {
		t.Errorf("default /callback path failed: %+v", res)
	}
}

// ------------- Refresh / Login input validation -------------

func TestRefresh_emptyRefreshTokenRejected(t *testing.T) {
	_, err := Refresh(context.Background(), RefreshOptions{Region: "na", RefreshToken: ""})
	if err == nil {
		t.Fatalf("empty refresh_token should error")
	}
}

func TestRefresh_unknownRegion(t *testing.T) {
	_, err := Refresh(context.Background(), RefreshOptions{
		Region: "zz", RefreshToken: "rt",
	})
	if err == nil {
		t.Fatalf("unknown region should error")
	}
}

func TestLogin_inputValidation(t *testing.T) {
	cases := []struct {
		name string
		opt  LoginOptions
	}{
		{"missing client_id", LoginOptions{RedirectURI: "x"}},
		{"missing redirect_uri", LoginOptions{ClientID: "x"}},
		{"unknown region", LoginOptions{ClientID: "x", RedirectURI: "http://x/cb", Region: "zz"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := Login(context.Background(), tc.opt)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestLogin_contextCancelExitsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Login(ctx, LoginOptions{
		Region:      "na",
		ClientID:    "cid",
		RedirectURI: "http://127.0.0.1:0/cb",
		Timeout:     1 * time.Second,
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("got err (any non-nil acceptable when ctx cancelled): %v", err)
	}
}

// ------------- writeBrowserResult -------------

func TestWriteBrowserResult_okAndError(t *testing.T) {
	rrOK := httptest.NewRecorder()
	writeBrowserResult(rrOK, true, "all good")
	if rrOK.Code != http.StatusOK {
		t.Errorf("ok path should be 200, got %d", rrOK.Code)
	}
	if !strings.Contains(rrOK.Body.String(), "all good") {
		t.Errorf("body missing message")
	}

	rrErr := httptest.NewRecorder()
	writeBrowserResult(rrErr, false, "boom")
	if rrErr.Code != http.StatusBadRequest {
		t.Errorf("err path should be 400, got %d", rrErr.Code)
	}
	if !strings.Contains(rrErr.Body.String(), "boom") {
		t.Errorf("err body missing message")
	}
}

// ------------- partner.go input validation -------------

func TestPartnerToken_inputValidation(t *testing.T) {
	if _, err := PartnerToken(context.Background(), PartnerOptions{Region: "na"}); err == nil {
		t.Fatalf("missing ClientID should error")
	}
	if _, err := PartnerToken(context.Background(), PartnerOptions{Region: "na", ClientID: "x"}); err == nil {
		t.Fatalf("missing ClientSecret should error")
	}
	if _, err := PartnerToken(context.Background(), PartnerOptions{
		Region: "zz", ClientID: "x", ClientSecret: "y",
	}); err == nil {
		t.Fatalf("unknown region should error")
	}
}

func TestRegisterPartner_inputValidation(t *testing.T) {
	if _, err := RegisterPartner(context.Background(), "", "na", "d"); err == nil {
		t.Fatalf("missing token should error")
	}
	if _, err := RegisterPartner(context.Background(), "tok", "na", ""); err == nil {
		t.Fatalf("missing domain should error")
	}
	if _, err := RegisterPartner(context.Background(), "tok", "zz", "d"); err == nil {
		t.Fatalf("unknown region should error")
	}
}

func TestVerifyPartnerPublicKey_inputValidation(t *testing.T) {
	if _, err := VerifyPartnerPublicKey(context.Background(), "tok", "zz", "d"); err == nil {
		t.Fatalf("unknown region should error")
	}
}
