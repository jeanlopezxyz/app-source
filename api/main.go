// Command catalogo-api serves the catalog over HTTP ("serve", the default)
// or applies the database schema ("migrate").
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jeanlopezxyz/app-source/api/internal/config"
	"github.com/jeanlopezxyz/app-source/api/internal/database"
	"github.com/jeanlopezxyz/app-source/api/internal/metrics"
	"github.com/jeanlopezxyz/app-source/api/internal/server"
)

// Set at build time with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch command {
	case "serve":
		err = serve(ctx)
	case "migrate":
		err = migrate(ctx)
	case "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q (use serve, migrate or version)", command)
	}
	if err != nil {
		slog.Error("command failed", "command", command, "error", err)
		os.Exit(1)
	}
}

func openStore(ctx context.Context) (*database.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	slog.Info("database settings", "host", cfg.DBHost, "port", cfg.DBPort, "name", cfg.DBName,
		"user", cfg.DBUser, "password_file", cfg.PasswordFile)
	return database.Open(ctx, cfg)
}

func migrate(ctx context.Context) error {
	store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.Migrate(ctx, 2*time.Minute)
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	info := server.VersionInfo{Version: version, Commit: commit, Color: server.ColorFor(version)}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.New(store, metrics.NewRegistry(version), info),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("catalogo-api listening", "addr", cfg.ListenAddr, "version", version, "commit", commit)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
