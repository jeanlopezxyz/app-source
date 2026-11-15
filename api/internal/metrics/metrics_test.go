package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestRegistryRendersCounterAndHistogram(t *testing.T) {
	r := NewRegistry("1.2.3")
	r.Observe("GET", "/api/products", 200, 3*time.Millisecond)
	r.Observe("GET", "/api/products", 200, 300*time.Millisecond)
	r.Observe("GET", "/api/products", 500, 10*time.Second)

	var out strings.Builder
	r.Render(&out)
	text := out.String()

	for _, want := range []string{
		`catalogo_build_info{version="1.2.3"} 1`,
		`catalogo_http_requests_total{method="GET",route="/api/products",code="200"} 2`,
		`catalogo_http_requests_total{method="GET",route="/api/products",code="500"} 1`,
		`catalogo_http_request_duration_seconds_bucket{route="/api/products",le="0.005"} 1`,
		`catalogo_http_request_duration_seconds_bucket{route="/api/products",le="0.5"} 2`,
		`catalogo_http_request_duration_seconds_bucket{route="/api/products",le="5"} 2`,
		`catalogo_http_request_duration_seconds_bucket{route="/api/products",le="+Inf"} 3`,
		`catalogo_http_request_duration_seconds_count{route="/api/products"} 3`,
		"# TYPE catalogo_http_request_duration_seconds histogram",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics output is missing %q\n%s", want, text)
		}
	}
}
