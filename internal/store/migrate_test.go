package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMigrate_IsIdempotent(t *testing.T) {
	pool := testPool(t) // already applies migrations once via testPool

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Applying again should be a safe no-op, not an error.
	if err := Migrate(ctx, pool, os.DirFS(migrationsDir(t))); err != nil {
		t.Fatalf("second Migrate call failed: %v", err)
	}
	if err := Migrate(ctx, pool, os.DirFS(migrationsDir(t))); err != nil {
		t.Fatalf("third Migrate call failed: %v", err)
	}
}

func TestMigrate_CreatesExpectedTables(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, table := range []string{"documents", "operations", "snapshots", "schema_migrations"} {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("checking for table %q: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after Migrate, it does not", table)
		}
	}
}
