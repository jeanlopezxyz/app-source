// Package metrics exposes HTTP metrics in the Prometheus text format
// (https://prometheus.io/docs/instrumenting/exposition_formats/) using only the stdlib.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Buckets are the latency histogram upper bounds, in seconds.
var Buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

type requestKey struct {
	method, route string
	code          int
}

type histogram struct {
	counts []uint64 // one per bucket, non-cumulative
	sum    float64
	total  uint64
}

// Registry stores the counters and histograms of the API.
type Registry struct {
	mu        sync.Mutex
	version   string
	requests  map[requestKey]uint64
	durations map[string]*histogram
}

// NewRegistry returns an empty registry that reports the given build version.
func NewRegistry(version string) *Registry {
	return &Registry{
		version:   version,
		requests:  map[requestKey]uint64{},
		durations: map[string]*histogram{},
	}
}

// Observe records one finished request. route must be a fixed pattern,
// never the raw URL, to keep label cardinality bounded.
func (r *Registry) Observe(method, route string, code int, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests[requestKey{method, route, code}]++

	h, ok := r.durations[route]
	if !ok {
		h = &histogram{counts: make([]uint64, len(Buckets))}
		r.durations[route] = h
	}
	seconds := elapsed.Seconds()
	for i, upper := range Buckets {
		if seconds <= upper {
			h.counts[i]++
			break
		}
	}
	h.sum += seconds
	h.total++
}

// Render renders every metric in the Prometheus text format.
func (r *Registry) Render(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fmt.Fprintln(w, "# HELP catalogo_build_info Build information of the catalogo API.")
	fmt.Fprintln(w, "# TYPE catalogo_build_info gauge")
	fmt.Fprintf(w, "catalogo_build_info{version=%q} 1\n", r.version)

	fmt.Fprintln(w, "# HELP catalogo_http_requests_total HTTP requests handled, by method, route and status code.")
	fmt.Fprintln(w, "# TYPE catalogo_http_requests_total counter")
	keys := make([]requestKey, 0, len(r.requests))
	for k := range r.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.route != b.route {
			return a.route < b.route
		}
		if a.method != b.method {
			return a.method < b.method
		}
		return a.code < b.code
	})
	for _, k := range keys {
		fmt.Fprintf(w, "catalogo_http_requests_total{method=%q,route=%q,code=\"%d\"} %d\n",
			k.method, k.route, k.code, r.requests[k])
	}

	fmt.Fprintln(w, "# HELP catalogo_http_request_duration_seconds HTTP request latency, by route.")
	fmt.Fprintln(w, "# TYPE catalogo_http_request_duration_seconds histogram")
	routes := make([]string, 0, len(r.durations))
	for route := range r.durations {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		h := r.durations[route]
		var cumulative uint64
		for i, upper := range Buckets {
			cumulative += h.counts[i]
			fmt.Fprintf(w, "catalogo_http_request_duration_seconds_bucket{route=%q,le=%q} %d\n",
				route, strconv.FormatFloat(upper, 'g', -1, 64), cumulative)
		}
		fmt.Fprintf(w, "catalogo_http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", route, h.total)
		fmt.Fprintf(w, "catalogo_http_request_duration_seconds_sum{route=%q} %s\n",
			route, strconv.FormatFloat(h.sum, 'g', -1, 64))
		fmt.Fprintf(w, "catalogo_http_request_duration_seconds_count{route=%q} %d\n", route, h.total)
	}
}

// Handler serves the metrics endpoint.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.Render(w)
	})
}
