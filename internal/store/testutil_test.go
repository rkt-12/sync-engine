package store

import (
	"context"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testDSN returns the DSN to use for integration tests. Override with
// TEST_DATABASE_URL for CI or a different local setup; defaults to the
// local instance this project's Docker Compose / local Postgres install
// is expected to provide.
func testDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5432/sync_engine_test?sslmode=disable"
}

// testPool connects to Postgres and applies migrations once per test
// binary run. If no database is reachable, every test using this helper
// is skipped rather than failed -- these are integration tests that
// require a real Postgres instance (see the "Running Locally" /
// "Testing" sections of the eventual README), and a missing database is
// an environment fact, not a code defect.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := Connect(ctx, testDSN())
	if err != nil {
		t.Skipf("skipping: no reachable test database at %s: %v", testDSN(), err)
	}
	t.Cleanup(pool.Close)

	if err := Migrate(ctx, pool, os.DirFS(migrationsDir(t))); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	return pool
}

// migrationsDir locates the repo's migrations/ directory relative to
// this test file, so tests work regardless of the working directory
// `go test` is invoked from.
func migrationsDir(t *testing.T) string {
	t.Helper()
	return "../../migrations"
}

// testDocumentID counter guarantees a unique document id per test, so
// tests can run against a real, persistent database without needing to
// truncate tables between runs or coordinate cleanup.
var testDocumentIDCounter int64

func testDocumentID(t *testing.T) string {
	t.Helper()
	n := atomic.AddInt64(&testDocumentIDCounter, 1)
	return "test-doc-" + t.Name() + "-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + strconv.FormatInt(n, 10)
}
