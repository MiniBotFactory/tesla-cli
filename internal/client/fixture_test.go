package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListVehicles_realFixture 重放从 Tesla CN /api/1/vehicles 抓的脱敏响应,
// 验证 Vehicle 结构字段映射(VIN/state/api_version/display_name)不漂移。
func TestListVehicles_realFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "vehicles_list.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/1/vehicles") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	vs, err := ListVehicles(context.Background(), c)
	if err != nil {
		t.Fatalf("ListVehicles: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("expected 1 vehicle in fixture, got %d", len(vs))
	}
	v := vs[0]
	if v.VIN != "LRW0000000000000" {
		t.Errorf("VIN mismatch: %q", v.VIN)
	}
	if v.DisplayName != "Test Car" {
		t.Errorf("DisplayName mismatch: %q", v.DisplayName)
	}
	if v.State != "online" {
		t.Errorf("State mismatch: %q", v.State)
	}
	if v.ID != 1234567890123456 {
		t.Errorf("ID mismatch: %d", v.ID)
	}
	if v.VehicleID != 9876543210987654 {
		t.Errorf("VehicleID mismatch: %d", v.VehicleID)
	}
}

// TestListProducts_realFixture 验证 /api/1/products 含 vehicle + wall_connector
// 的混合响应被原样回传(ListProducts 返回 []map[string]any,不解结构)。
func TestListProducts_realFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "products.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/1/products") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	items, err := ListProducts(context.Background(), c)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 products, got %d", len(items))
	}
	if got := items[0]["device_type"]; got != "vehicle" {
		t.Errorf("[0].device_type: %v", got)
	}
	if got := items[1]["device_type"]; got != "energy" {
		t.Errorf("[1].device_type: %v", got)
	}
	if got := items[1]["resource_type"]; got != "wall_connector" {
		t.Errorf("[1].resource_type: %v", got)
	}
	// cached_data 字段必须不存在(脱敏时被剔除)
	if _, ok := items[0]["cached_data"]; ok {
		t.Errorf("[0].cached_data should have been stripped")
	}
}

// TestClientFixtures_noLeakedSecrets 守住 fixture 永不退化回真车。
func TestClientFixtures_noLeakedSecrets(t *testing.T) {
	forbidden := []string{
		"LRW0000000000000",
		"f2ec4168-78cf-495b",
		"f8cc0757-4fc6-4f59",
		"ta-secret.",
		"Test Car",
		"STE20240907",
		"9876543210987654",
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
				t.Errorf("%s: forbidden %q present (sanitization regression)", p, bad)
			}
		}
	}
}
