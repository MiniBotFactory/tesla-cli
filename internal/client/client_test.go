package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wmango/tesla-cli/internal/errs"
)

// newTestClient 返回一个把 baseURL 指向 httptest server 的 *Client。
// 同包测试可直接访问未导出字段,免去对 endpoints.go 的硬编码依赖。
func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	return &Client{
		http:      &http.Client{Timeout: 5 * time.Second},
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     "test-token",
		retry:     3,
		userAgent: "tesla-cli-test/0",
	}
}

// ----------------- client.go -----------------

func TestNew_rejectsEmptyAccessToken(t *testing.T) {
	_, err := New(Options{Region: "na", AccessToken: ""})
	if err == nil {
		t.Fatalf("empty token should error")
	}
}

func TestNew_unknownRegionFails(t *testing.T) {
	_, err := New(Options{Region: "zz", AccessToken: "x"})
	if err == nil {
		t.Fatalf("unknown region should error")
	}
}

func TestNew_validInputs(t *testing.T) {
	c, err := New(Options{Region: "na", AccessToken: "tok", Timeout: 10 * time.Second, Retry: 2})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.token != "tok" {
		t.Errorf("token field not stored")
	}
	if c.retry != 2 {
		t.Errorf("retry not honored")
	}
}

func TestGet_returnsBodyOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing Bearer header: %q", got)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	body, err := c.Get(context.Background(), "/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body mismatch: %q", body)
	}
}

func TestDo_4xxClassification(t *testing.T) {
	cases := []struct {
		status int
		want   errs.ExitCode
	}{
		{http.StatusUnauthorized, errs.ExitAuth},
		{http.StatusForbidden, errs.ExitScope},
		{http.StatusNotFound, errs.ExitUsage},
		{http.StatusBadRequest, errs.ExitUsage},
		{http.StatusUnprocessableEntity, errs.ExitUsage},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"err":"x"}`))
			}))
			defer srv.Close()

			c := newTestClient(t, srv.URL)
			_, err := c.Get(context.Background(), "/x")
			if err == nil {
				t.Fatalf("expected error for %d", tc.status)
			}
			if got := errs.CodeOf(err); got != tc.want {
				t.Errorf("status %d: want code %d, got %d (%v)", tc.status, tc.want, got, err)
			}
		})
	}
}

func TestDo_5xxRetriesThenFails(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.retry = 2 // 总共最多 3 次 (initial + 2 retries)

	_, err := c.Get(context.Background(), "/x")
	if err == nil {
		t.Fatalf("should fail after retries")
	}
	if errs.CodeOf(err) != errs.ExitUpstream5xx {
		t.Errorf("want ExitUpstream5xx, got %d (%v)", errs.CodeOf(err), err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 calls (initial + 2 retries), got %d", got)
	}
}

func TestDo_429RetryThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	body, err := c.Get(context.Background(), "/x")
	if err != nil {
		t.Fatalf("should succeed after retry: %v", err)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body wrong: %q", body)
	}
}

func TestPost_bodySerializedAsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type: %q", ct)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.Post(context.Background(), "/x", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("Post: %v", err)
	}
}

func TestEnsureLeadingSlash(t *testing.T) {
	if got := ensureLeadingSlash("/a"); got != "/a" {
		t.Errorf("kept slash: %q", got)
	}
	if got := ensureLeadingSlash("a"); got != "/a" {
		t.Errorf("added slash: %q", got)
	}
}

// ----------------- vehicles.go -----------------

func TestListVehicles_parsesEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/vehicles" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
            "response":[
              {"id":12345,"vehicle_id":987,"vin":"5YJ12345678901234","display_name":"M3","state":"online","api_version":68}
            ],
            "count":1
        }`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	vs, err := ListVehicles(context.Background(), c)
	if err != nil {
		t.Fatalf("ListVehicles: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("expected 1 vehicle, got %d", len(vs))
	}
	if vs[0].VIN != "5YJ12345678901234" || vs[0].State != "online" {
		t.Errorf("vehicle fields: %+v", vs[0])
	}
}

func TestVehicleData_emptyVinRejected(t *testing.T) {
	c := newTestClient(t, "http://unused")
	_, err := VehicleData(context.Background(), c, "")
	if err == nil {
		t.Fatalf("empty VIN should error")
	}
}

func TestResolveVehicle_byVIN(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":[{"id":1,"vin":"AAA"}],"count":1}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	vh, err := ResolveVehicle(context.Background(), c, "AAA")
	if err != nil {
		t.Fatalf("ResolveVehicle: %v", err)
	}
	if vh.VIN != "AAA" {
		t.Errorf("got %+v", vh)
	}
}

func TestResolveVehicle_byID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":[{"id":42,"vin":"AAA"}],"count":1}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	vh, err := ResolveVehicle(context.Background(), c, "42")
	if err != nil {
		t.Fatalf("ResolveVehicle by id: %v", err)
	}
	if vh.ID != 42 {
		t.Errorf("expected id 42, got %d", vh.ID)
	}
}

func TestResolveVehicle_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":[],"count":0}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := ResolveVehicle(context.Background(), c, "ZZZ")
	if err == nil {
		t.Fatalf("expected error when not found")
	}
}

// ----------------- energy.go -----------------

func TestListProducts_parsesEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/products" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"response":[{"resource_type":"battery"},{"resource_type":"vehicle"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	items, err := ListProducts(context.Background(), c)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 products, got %d", len(items))
	}
}

func TestEnergySiteInfo_emptyIDRejected(t *testing.T) {
	c := newTestClient(t, "http://unused")
	_, err := EnergySiteInfo(context.Background(), c, "")
	if err == nil {
		t.Fatalf("empty site_id should error")
	}
}

func TestEnergyLiveStatus_returnsParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"solar_power":1234,"battery_power":-500}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	got, err := EnergyLiveStatus(context.Background(), c, "site-1")
	if err != nil {
		t.Fatalf("EnergyLiveStatus: %v", err)
	}
	if got["solar_power"].(float64) != 1234 {
		t.Errorf("solar_power not parsed: %+v", got)
	}
}
