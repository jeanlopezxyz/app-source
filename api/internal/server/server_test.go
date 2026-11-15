package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeanlopezxyz/app-source/api/internal/database"
	"github.com/jeanlopezxyz/app-source/api/internal/metrics"
)

type fakeStore struct {
	err      error
	products []database.Product
}

func (f fakeStore) Ping(context.Context) error { return f.err }

func (f fakeStore) ListProducts(context.Context) ([]database.Product, error) {
	return f.products, f.err
}

func newTestServer(store ProductStore) http.Handler {
	return New(store, metrics.NewRegistry("1.0.0"),
		VersionInfo{Version: "1.0.0", Commit: "abc123", Color: ColorFor("1.0.0")})
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthAndReadiness(t *testing.T) {
	healthy := newTestServer(fakeStore{})
	if code := get(t, healthy, "/healthz").Code; code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", code)
	}
	if code := get(t, healthy, "/readyz").Code; code != http.StatusOK {
		t.Errorf("/readyz = %d, want 200", code)
	}

	broken := newTestServer(fakeStore{err: errors.New("connection refused")})
	if code := get(t, broken, "/healthz").Code; code != http.StatusOK {
		t.Errorf("/healthz without database = %d, want 200", code)
	}
	if code := get(t, broken, "/readyz").Code; code != http.StatusServiceUnavailable {
		t.Errorf("/readyz without database = %d, want 503", code)
	}
}

func TestProducts(t *testing.T) {
	h := newTestServer(fakeStore{products: []database.Product{{ID: 1, SKU: "ARGO-001", Name: "Taza Argo"}}})
	rec := get(t, h, "/api/products")
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/products = %d, want 200", rec.Code)
	}
	var products []database.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &products); err != nil || len(products) != 1 {
		t.Fatalf("unexpected body %q (%v)", rec.Body.String(), err)
	}

	if code := get(t, newTestServer(fakeStore{err: errors.New("down")}), "/api/products").Code; code != http.StatusServiceUnavailable {
		t.Errorf("/api/products without database = %d, want 503", code)
	}
}

func TestVersion(t *testing.T) {
	rec := get(t, newTestServer(fakeStore{}), "/api/version")
	var info VersionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.0.0" || info.Color != "#2563eb" {
		t.Errorf("unexpected version info: %+v", info)
	}
}

func TestColorFor(t *testing.T) {
	if got := ColorFor("v1.0.0"); got != "#2563eb" {
		t.Errorf("ColorFor(v1.0.0) = %s", got)
	}
	if got := ColorFor("dev"); got != defaultColor {
		t.Errorf("ColorFor(dev) = %s, want default", got)
	}
}

func TestMetricsUseRoutePatterns(t *testing.T) {
	h := newTestServer(fakeStore{})
	get(t, h, "/api/version")
	get(t, h, "/does-not-exist")
	body := get(t, h, "/metrics").Body.String()
	for _, want := range []string{
		`catalogo_http_requests_total{method="GET",route="/api/version",code="200"} 1`,
		`catalogo_http_requests_total{method="GET",route="unmatched",code="404"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q\n%s", want, body)
		}
	}
}
