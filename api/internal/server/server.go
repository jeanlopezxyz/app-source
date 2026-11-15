// Package server wires the HTTP endpoints of the catalogo API.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jeanlopezxyz/app-source/api/internal/database"
	"github.com/jeanlopezxyz/app-source/api/internal/metrics"
)

// ProductStore is what the handlers need from the database.
type ProductStore interface {
	Ping(ctx context.Context) error
	ListProducts(ctx context.Context) ([]database.Product, error)
}

// versionColors gives each release its own color, so the canary of
// session 8 is visible in the browser. Unknown versions are gray.
var versionColors = map[string]string{
	"1.0.0": "#2563eb", // blue
	"1.1.0": "#16a34a", // green
}

const defaultColor = "#6b7280"

// ColorFor returns the display color of a version ("v1.0.0" or "1.0.0").
func ColorFor(version string) string {
	if color, ok := versionColors[strings.TrimPrefix(version, "v")]; ok {
		return color
	}
	return defaultColor
}

// VersionInfo is the body of /api/version.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Color   string `json:"color"`
}

// New returns the HTTP handler with every route registered.
func New(store ProductStore, registry *metrics.Registry, info VersionInfo) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			slog.Warn("readiness check failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	mux.HandleFunc("GET /api/products", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		products, err := store.ListProducts(ctx)
		if err != nil {
			slog.Error("list products failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "catalog unavailable"})
			return
		}
		if products == nil {
			products = []database.Product{}
		}
		writeJSON(w, http.StatusOK, products)
	})

	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, info)
	})

	mux.Handle("GET /metrics", registry.Handler())

	return instrument(mux, registry)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write response failed", "error", err)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// instrument records a counter and a latency histogram per route pattern.
func instrument(mux *http.ServeMux, registry *metrics.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		_, pattern := mux.Handler(r)
		mux.ServeHTTP(rec, r)

		route := "unmatched"
		if _, path, ok := strings.Cut(pattern, " "); ok {
			route = path
		}
		registry.Observe(r.Method, route, rec.status, time.Since(start))
	})
}
