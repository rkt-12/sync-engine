package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DocumentStore persists document metadata.
type DocumentStore struct {
	pool *pgxpool.Pool
}

// NewDocumentStore constructs a DocumentStore backed by pool.
func NewDocumentStore(pool *pgxpool.Pool) *DocumentStore {
	return &DocumentStore{pool: pool}
}

// CreateDocument inserts a new document row.
func (s *DocumentStore) CreateDocument(ctx context.Context, id, title string) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO documents (id, title) VALUES ($1, $2)`, id, title,
	); err != nil {
		return fmt.Errorf("store: creating document %q: %w", id, err)
	}
	return nil
}

// CreateDocumentIfNotExists inserts a new document row only if one with this id doesn't already exist.
func (s *DocumentStore) CreateDocumentIfNotExists(ctx context.Context, id, title string) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO documents (id, title) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, id, title,
	); err != nil {
		return fmt.Errorf("store: creating document %q: %w", id, err)
	}
	return nil
}

// DocumentExists reports whether a document with this id has been created.
func (s *DocumentStore) DocumentExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM documents WHERE id = $1)`, id,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("store: checking document %q: %w", id, err)
	}
	return exists, nil
}
