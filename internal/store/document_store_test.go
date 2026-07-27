package store

import (
	"context"
	"testing"
	"time"
)

func TestDocumentStore_CreateAndExists(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := testDocumentID(t)

	exists, err := docs.DocumentExists(ctx, id)
	if err != nil {
		t.Fatalf("DocumentExists before creation failed: %v", err)
	}
	if exists {
		t.Fatalf("expected document %q not to exist yet", id)
	}

	if err := docs.CreateDocument(ctx, id, "My Document"); err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	exists, err = docs.DocumentExists(ctx, id)
	if err != nil {
		t.Fatalf("DocumentExists after creation failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected document %q to exist after CreateDocument", id)
	}
}

func TestDocumentStore_CreateDocument_DuplicateID_Errors(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := testDocumentID(t)
	if err := docs.CreateDocument(ctx, id, "First"); err != nil {
		t.Fatalf("first CreateDocument failed: %v", err)
	}

	if err := docs.CreateDocument(ctx, id, "Second"); err == nil {
		t.Fatal("expected an error creating a document with a duplicate id via CreateDocument, got nil")
	}
}

func TestDocumentStore_CreateDocumentIfNotExists_IsIdempotent(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := testDocumentID(t)

	if err := docs.CreateDocumentIfNotExists(ctx, id, "First"); err != nil {
		t.Fatalf("first CreateDocumentIfNotExists failed: %v", err)
	}
	if err := docs.CreateDocumentIfNotExists(ctx, id, "Second"); err != nil {
		t.Fatalf("second CreateDocumentIfNotExists should be a no-op, got error: %v", err)
	}

	exists, err := docs.DocumentExists(ctx, id)
	if err != nil {
		t.Fatalf("DocumentExists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected document to exist")
	}
}
