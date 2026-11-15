// Package database connects to PostgreSQL and queries the catalog.
package database

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanlopezxyz/app-source/api/internal/config"
)

// Product is one catalog item.
type Product struct {
	ID          int64  `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int64  `json:"stock"`
}

// Store wraps the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// ReadPassword returns the trimmed content of the password file.
func ReadPassword(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read database password file: %w", err)
	}
	password := strings.TrimSpace(string(raw))
	if password == "" {
		return "", fmt.Errorf("database password file %s is empty", path)
	}
	return password, nil
}

// Open creates a pool. The password is read again from the file before every
// new connection, so a rotated Secret is picked up without a restart.
func Open(ctx context.Context, cfg config.Config) (*Store, error) {
	dsn := fmt.Sprintf("postgres://%s/%s?sslmode=%s",
		net.JoinHostPort(cfg.DBHost, strconv.Itoa(cfg.DBPort)), cfg.DBName, cfg.DBSSLMode)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	poolCfg.ConnConfig.User = cfg.DBUser
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second
	poolCfg.MaxConns = 5
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.BeforeConnect = func(_ context.Context, cc *pgx.ConnConfig) error {
		password, err := ReadPassword(cfg.PasswordFile)
		if err != nil {
			return err
		}
		cc.Password = password
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases every connection.
func (s *Store) Close() { s.pool.Close() }

// Ping checks that the database accepts queries.
func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.pool.QueryRow(ctx, "SELECT 1").Scan(&one)
}

// ListProducts returns the catalog ordered by id.
func (s *Store) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, sku, name, description, price_cents, COALESCE(stock, 0) FROM products ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Product, error) {
		var p Product
		err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.PriceCents, &p.Stock)
		return p, err
	})
}
