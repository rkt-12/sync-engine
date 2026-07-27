package store

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate applies every *.sql file found in migrationsFS, in filename order, exactly once.
// Filenames are expected to sort in the order they should apply (e.g.
// "0001_init.sql", "0002_add_foo.sql", ...).
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("store: creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("store: reading migrations directory: %w", err)
	}

	var filenames []string
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".sql" {
			continue
		}
		filenames = append(filenames, e.Name())
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		if err := applyMigrationIfNeeded(ctx, pool, migrationsFS, filename); err != nil {
			return err
		}
	}
	return nil
}

func applyMigrationIfNeeded(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS, filename string) error {
	var alreadyApplied bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`,
		filename,
	).Scan(&alreadyApplied)
	if err != nil {
		return fmt.Errorf("store: checking migration %s: %w", filename, err)
	}
	if alreadyApplied {
		return nil
	}

	contents, err := fs.ReadFile(migrationsFS, filename)
	if err != nil {
		return fmt.Errorf("store: reading migration %s: %w", filename, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: beginning transaction for migration %s: %w", filename, err)
	}
	defer tx.Rollback(ctx) // no-op if Commit succeeds first

	if _, err := tx.Exec(ctx, string(contents)); err != nil {
		return fmt.Errorf("store: applying migration %s: %w", filename, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, filename); err != nil {
		return fmt.Errorf("store: recording migration %s: %w", filename, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: committing migration %s: %w", filename, err)
	}
	return nil
}
