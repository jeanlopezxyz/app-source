package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// migrationLockID serializes concurrent "migrate" runs with an advisory lock.
const migrationLockID = 727274

// schema is idempotent: it can run on every deploy (Argo CD PreSync hook).
var schema = []string{
	`CREATE TABLE IF NOT EXISTS products (
		id          BIGSERIAL PRIMARY KEY,
		sku         TEXT NOT NULL UNIQUE,
		name        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		price_cents BIGINT NOT NULL CHECK (price_cents >= 0)
	)`,
}

var seed = []Product{
	{SKU: "ARGO-001", Name: "Taza Argo", Description: "Taza de ceramica con el pulpo de Argo", PriceCents: 1500},
	{SKU: "ARGO-002", Name: "Polo GitOps", Description: "Polo de algodon: Git es la fuente de verdad", PriceCents: 4500},
	{SKU: "ARGO-003", Name: "Stickers sync", Description: "Pack de stickers Synced y Healthy", PriceCents: 800},
	{SKU: "ARGO-004", Name: "Gorra drift", Description: "Gorra para cuando el self-heal llega tarde", PriceCents: 3200},
	{SKU: "ARGO-005", Name: "Cuaderno waves", Description: "Cuaderno para planear sync waves", PriceCents: 1200},
}

// Migrate creates or updates the schema and inserts the sample products.
// It retries the connection for up to wait, so it can start before the database.
func (s *Store) Migrate(ctx context.Context, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		err := s.Ping(ctx)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database not reachable after %s: %w", wait, err)
		}
		slog.Info("waiting for database", "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("take migration lock: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID) //nolint:errcheck

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, stmt := range schema {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	inserted := 0
	for _, p := range seed {
		tag, err := tx.Exec(ctx,
			`INSERT INTO products (sku, name, description, price_cents)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (sku) DO NOTHING`,
			p.SKU, p.Name, p.Description, p.PriceCents)
		if err != nil {
			return fmt.Errorf("seed product %s: %w", p.SKU, err)
		}
		inserted += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	slog.Info("migration finished", "statements", len(schema), "products_inserted", inserted)
	return nil
}
