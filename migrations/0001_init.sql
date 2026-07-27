-- Initial schema: documents, operations (the source of truth), and
-- snapshots (an optimization, used starting Phase 8)

CREATE TABLE documents (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The operation log: the durable source of truth.
CREATE TABLE operations (
    -- sequence_id is a server-local ordering used ONLY for catch-up
    -- pagination during reconnect sync (docs/synchronization-protocol.md).
    sequence_id       BIGSERIAL PRIMARY KEY,

    -- operation_id and the element-reference columns store a
    -- crdt.Identifier{ClientID, Counter} encoded as 16 bytes
    -- (big-endian ClientID || big-endian Counter).
    operation_id      BYTEA  NOT NULL,
    document_id       TEXT   NOT NULL REFERENCES documents(id),
    client_id         BIGINT NOT NULL,
    logical_clock     BIGINT NOT NULL,

    -- 1 = insert, 2 = delete. See internal/store/operation_store.go.
    operation_type    SMALLINT NOT NULL,

    -- Populated for inserts only (NULL for deletes).
    parent_element_id BYTEA,
    value             TEXT,

    -- Populated for deletes only (NULL for inserts).
    target_element_id BYTEA,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- The idempotency backstop (Invariant 6): even under concurrent
    -- retried writes, only one row can ever exist per operation.
    CONSTRAINT operations_document_operation_unique UNIQUE (document_id, operation_id)
);

-- Supports both catch-up pagination and full-document replay.
CREATE INDEX idx_operations_document_seq ON operations (document_id, sequence_id);

CREATE TABLE snapshots (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id        TEXT NOT NULL REFERENCES documents(id),
    snapshot_version   BIGINT NOT NULL,
    state              BYTEA NOT NULL,
    included_up_to_seq BIGINT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_snapshots_document_version ON snapshots (document_id, snapshot_version DESC);