package tesla

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestEndpointsFor_allKnownRegions(t *testing.T) {
	for _, r := range AllRegions() {
		ep, err := EndpointsFor(r)
		if err != nil {
			t.Fatalf("region %q: unexpected err %v", r, err)
		}
		if ep.Region != r {
			t.Errorf("region %q: returned %q", r, ep.Region)
		}
		if ep.AuthorizeURL == "" || ep.TokenURL == "" || ep.APIBase == "" || ep.OIDCMetadata == "" {
			t.Errorf("region %q: empty URL field: %+v", r, ep)
		}
		if !strings.HasPrefix(ep.APIBase, "https://") {
			t.Errorf("region %q: APIBase must be https, got %q", r, ep.APIBase)
		}
	}
}

func TestEndpointsFor_emptyDefaultsToNA(t *testing.T) {
	ep, err := EndpointsFor("")
	if err != nil {
		t.Fatalf("empty region should default, got err %v", err)
	}
	if ep.Region != "na" {
		t.Errorf("empty region default should be na, got %q", ep.Region)
	}
}

func TestEndpointsFor_caseInsensitive(t *testing.T) {
	ep, err := EndpointsFor("EU")
	if err != nil {
		t.Fatalf("EU should be accepted: %v", err)
	}
	if ep.Region != "eu" {
		t.Errorf("EU should map to eu, got %q", ep.Region)
	}
}

func TestEndpointsFor_unknownReturnsError(t *testing.T) {
	_, err := EndpointsFor("zz")
	if err == nil {
		t.Fatalf("unknown region should error")
	}
}

func TestPairDeepLink_format(t *testing.T) {
	got := PairDeepLink("my.example.com")
	want := "https://tesla.com/_ak/my.example.com"
	if got != want {
		t.Errorf("PairDeepLink mismatch: want %q, got %q", want, got)
	}
}

func TestNewHTTPClient_defaultsAndOverride(t *testing.T) {
	c1 := NewHTTPClient(0)
	if c1.Timeout != 30*time.Second {
		t.Errorf("zero timeout should default to 30s, got %v", c1.Timeout)
	}
	c2 := NewHTTPClient(5 * time.Second)
	if c2.Timeout != 5*time.Second {
		t.Errorf("override ignored, got %v", c2.Timeout)
	}
}

func TestSetUA_setsHeaderWhenEmpty(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	SetUA(req)
	if got := req.Header.Get("User-Agent"); got != UserAgent {
		t.Errorf("UA should be set to %q, got %q", UserAgent, got)
	}
}

func TestSetUA_doesNotOverrideExisting(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("User-Agent", "custom/1.0")
	SetUA(req)
	if got := req.Header.Get("User-Agent"); got != "custom/1.0" {
		t.Errorf("existing UA must not be overwritten, got %q", got)
	}
}

func TestSetUA_nilSafe(t *testing.T) {
	// 不应 panic
	SetUA(nil)
}
