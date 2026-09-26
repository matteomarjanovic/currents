package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (m *PgStore) saveCollectionURI(ctx context.Context, uri string) (string, error) {
	var collection string
	err := m.pool.QueryRow(ctx, `SELECT collection_uri FROM save WHERE uri = $1`, uri).Scan(&collection)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return collection, err
}

// lockRepository coordinates Currents writes, import writes and maintenance
// across processes. PDS compare-and-swap additionally protects external edits.
func (m *PgStore) lockRepository(ctx context.Context, did string) (func(), error) {
	// Waiting for this lock must not exhaust the query pool: its owner still
	// needs pool connections to validate destinations and persist progress.
	conn, err := pgx.Connect(ctx, m.cfg.DSN)
	if err != nil {
		return nil, err
	}
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 54))`, did); err != nil {
		conn.Close(context.Background())
		return nil, err
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Closing this dedicated session releases its advisory lock, including
		// on cancelled requests. It is never returned to the query pool.
		conn.Close(ctx)
	}, nil
}

func (m *PgStore) collectionDeleting(ctx context.Context, uris ...string) (bool, error) {
	var deleting bool
	err := m.pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM collection_delete_job WHERE collection_uri = ANY($1)
	)`, uris).Scan(&deleting)
	return deleting, err
}

// Caller holds lockRepository. A persisted request is the point at which the
// deletion is accepted; no PDS record is removed by the HTTP handler.
func (m *PgStore) queueCollectionDelete(ctx context.Context, did, uri string) error {
	_, err := m.pool.Exec(ctx, `INSERT INTO collection_delete_job (collection_uri, owner_did)
		VALUES ($1, $2) ON CONFLICT (collection_uri) DO NOTHING`, uri, did)
	return err
}

func (m *PgStore) cancelCollectionImports(ctx context.Context, did string, uris []string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE import_job SET status = 'failed', error = 'collection deletion requested', updated_at = now()
		WHERE owner_did = $1 AND target_collection_uri = ANY($2) AND status IN ('listing', 'running')`, did, uris); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE import_item SET status = 'failed', error = 'collection deletion requested', updated_at = now()
		WHERE owner_did = $1 AND status IN ('queued', 'running') AND job_id IN (
			SELECT id FROM import_job WHERE owner_did = $1 AND target_collection_uri = ANY($2)
		)`, did, uris); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (m *PgStore) repositoryBusy(ctx context.Context, did string) (bool, error) {
	var busy bool
	err := m.pool.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM collection_delete_job WHERE owner_did = $1) OR
		EXISTS (SELECT 1 FROM pds_wipe WHERE did = $1) OR
		EXISTS (SELECT 1 FROM import_job WHERE owner_did = $1 AND status IN ('listing', 'running')) OR
		EXISTS (SELECT 1 FROM import_item WHERE owner_did = $1 AND status IN ('queued', 'running'))`, did).Scan(&busy)
	return busy, err
}

// Use the appview only to find accounts worth checking. The repair itself
// re-enumerates their PDS; index lag is never evidence for rewriting a record.
func (m *PgStore) orphanOwners(ctx context.Context, did string) ([]string, error) {
	rows, err := m.pool.Query(ctx, `SELECT DISTINCT owner_did FROM (
		SELECT c.author_did AS owner_did FROM collection c
		LEFT JOIN collection p ON p.uri = c.parent_uri
		WHERE c.parent_uri IS NOT NULL AND p.uri IS NULL
		UNION SELECT s.author_did FROM save s LEFT JOIN collection c ON c.uri = s.collection_uri
		WHERE s.collection_uri <> '' AND c.uri IS NULL
		UNION SELECT owner_did FROM orphan_record
	) candidates WHERE ($1 = '' OR owner_did = $1)
	AND EXISTS (SELECT 1 FROM "user" u WHERE u.did = owner_did)
	ORDER BY owner_did`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var owners []string
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}

func (m *PgStore) observeOrphan(ctx context.Context, did, uri, target string, dryRun bool) (time.Time, error) {
	var firstSeen time.Time
	if dryRun {
		err := m.pool.QueryRow(ctx, `SELECT COALESCE((SELECT first_seen_at FROM orphan_record
			WHERE uri = $1 AND target_uri = $2), now())`, uri, target).Scan(&firstSeen)
		return firstSeen, err
	}
	err := m.pool.QueryRow(ctx, `INSERT INTO orphan_record (uri, owner_did, target_uri) VALUES ($1, $2, $3)
		ON CONFLICT (uri) DO UPDATE SET target_uri = EXCLUDED.target_uri,
		first_seen_at = CASE WHEN orphan_record.target_uri = EXCLUDED.target_uri THEN orphan_record.first_seen_at ELSE now() END
		RETURNING first_seen_at`, uri, did, target).Scan(&firstSeen)
	return firstSeen, err
}
