package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (m *PgStore) JetstreamCursor(ctx context.Context) (uint64, error) {
	var seq int64
	err := m.pool.QueryRow(ctx, `SELECT seq FROM jetstream_cursor WHERE singleton`).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if seq < 0 {
		return 0, fmt.Errorf("negative Jetstream cursor")
	}
	return uint64(seq), nil
}

func (m *PgStore) AdvanceJetstreamCursor(ctx context.Context, seq uint64) error {
	_, err := m.pool.Exec(ctx, `UPDATE jetstream_cursor SET seq = GREATEST(seq, $1) WHERE singleton`, int64(seq))
	return err
}

func (m *PgStore) JetstreamRepoState(ctx context.Context, did string) (string, string, error) {
	var state, rev string
	err := m.pool.QueryRow(ctx, `SELECT state, reconciled_rev FROM jetstream_repo WHERE did = $1`, did).Scan(&state, &rev)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return state, rev, err
}

// A profile signal enrolls a new repo, but never revives an account that the
// user explicitly removed from Currents.
func (m *PgStore) RegisterJetstreamRepo(ctx context.Context, did string) (bool, error) {
	result, err := m.pool.Exec(ctx, `INSERT INTO jetstream_repo (did, state) VALUES ($1, 'active')
		ON CONFLICT DO NOTHING`, did)
	return result.RowsAffected() != 0, err
}

func (m *PgStore) EnableJetstreamRepo(ctx context.Context, did string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO jetstream_repo (did, state) VALUES ($1, 'active')
		ON CONFLICT (did) DO UPDATE SET state = 'active', reconciled_rev = ''`, did); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO jetstream_backfill (did, full_scan) VALUES ($1, TRUE)
		ON CONFLICT (did) DO UPDATE SET due_at = NOW(), attempts = 0, full_scan = TRUE, missing_since = NULL`, did); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (m *PgStore) OptOutJetstreamRepo(ctx context.Context, did string) error {
	_, err := m.pool.Exec(ctx, `INSERT INTO jetstream_repo (did, state) VALUES ($1, 'opted_out')
		ON CONFLICT (did) DO UPDATE SET state = 'opted_out'`, did)
	return err
}

func (m *PgStore) QueueJetstreamBackfill(ctx context.Context, did string) error {
	_, err := m.pool.Exec(ctx, `INSERT INTO jetstream_backfill (did, full_scan) VALUES ($1, TRUE)
		ON CONFLICT (did) DO UPDATE SET due_at = NOW(), attempts = 0, full_scan = TRUE, missing_since = NULL`, did)
	return err
}

func (m *PgStore) SetJetstreamRepoInactive(ctx context.Context, did string) error {
	_, err := m.pool.Exec(ctx, `UPDATE jetstream_repo SET state = 'inactive'
		WHERE did = $1 AND state = 'active'`, did)
	return err
}

func (m *PgStore) ReactivateJetstreamRepo(ctx context.Context, did string) (bool, error) {
	result, err := m.pool.Exec(ctx, `UPDATE jetstream_repo SET state = 'active', reconciled_rev = ''
		WHERE did = $1 AND state = 'inactive'`, did)
	if err != nil || result.RowsAffected() == 0 {
		return false, err
	}
	return true, m.QueueJetstreamBackfill(ctx, did)
}

func (m *PgStore) SetJetstreamReconciledRev(ctx context.Context, did, rev string) error {
	_, err := m.pool.Exec(ctx, `UPDATE jetstream_repo SET reconciled_rev = $2
		WHERE did = $1 AND state = 'active'`, did, rev)
	return err
}

// Remove only records mirrored from the PDS. OAuth sessions, user preferences,
// follows of this DID, and other viewers' favourites remain intact.
func (m *PgStore) ClearIndexedRepo(ctx context.Context, did string) error {
	rows, err := m.pool.Query(ctx, `SELECT uri FROM save WHERE author_did = $1`, did)
	if err != nil {
		return err
	}
	var saves []string
	for rows.Next() {
		var uri string
		if err := rows.Scan(&uri); err != nil {
			rows.Close()
			return err
		}
		saves = append(saves, uri)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, uri := range saves {
		if err := m.DeleteSave(ctx, uri); err != nil {
			return fmt.Errorf("deleting indexed save %s: %w", uri, err)
		}
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`DELETE FROM collection WHERE author_did = $1`,
		`DELETE FROM follow WHERE follower_did = $1`,
		`DELETE FROM favourite_collection WHERE viewer_did = $1`,
		`DELETE FROM "user" WHERE did = $1`,
	} {
		if _, err := tx.Exec(ctx, q, did); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type JetstreamBackfillJob struct {
	DID          string
	Due          time.Time
	Attempts     int
	Full         bool
	MissingSince *time.Time
}

func (m *PgStore) NextJetstreamBackfill(ctx context.Context) (JetstreamBackfillJob, error) {
	var job JetstreamBackfillJob
	err := m.pool.QueryRow(ctx, `SELECT did, due_at, attempts, full_scan, missing_since FROM jetstream_backfill
		WHERE due_at <= NOW() ORDER BY due_at LIMIT 1`).Scan(&job.DID, &job.Due, &job.Attempts, &job.Full, &job.MissingSince)
	if errors.Is(err, pgx.ErrNoRows) {
		return JetstreamBackfillJob{}, nil
	}
	return job, err
}

func (m *PgStore) CompleteJetstreamBackfill(ctx context.Context, did string, due time.Time) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM jetstream_backfill WHERE did = $1 AND due_at = $2`, did, due)
	return err
}

func (m *PgStore) RetryJetstreamBackfill(ctx context.Context, did string, due time.Time, delaySeconds int) error {
	_, err := m.pool.Exec(ctx, `UPDATE jetstream_backfill
		SET attempts = attempts + 1, due_at = NOW() + $3 * INTERVAL '1 second'
		WHERE did = $1 AND due_at = $2`, did, due, delaySeconds)
	return err
}

func (m *PgStore) ObserveJetstreamRepoMissing(ctx context.Context, did string, due time.Time) error {
	_, err := m.pool.Exec(ctx, `UPDATE jetstream_backfill
		SET missing_since = COALESCE(missing_since, NOW())
		WHERE did = $1 AND due_at = $2`, did, due)
	return err
}

func (m *PgStore) JetstreamAuditPlan(ctx context.Context, did string) (int64, int64, error) {
	var repos, saves int64
	err := m.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(COALESCE(s.n, 0)), 0)
		FROM jetstream_repo r
		LEFT JOIN (SELECT author_did, count(*) n FROM save WHERE $1 = '' OR author_did = $1 GROUP BY author_did) s ON s.author_did = r.did
		WHERE r.state = 'active' AND ($1 = '' OR r.did = $1)
	`, did).Scan(&repos, &saves)
	return repos, saves, err
}

func (m *PgStore) QueueJetstreamAudit(ctx context.Context, did string) (int64, error) {
	result, err := m.pool.Exec(ctx, `
		INSERT INTO jetstream_backfill (did, full_scan)
		SELECT did, $1 <> '' FROM jetstream_repo WHERE state = 'active' AND ($1 = '' OR did = $1)
		ON CONFLICT (did) DO UPDATE SET due_at = NOW(), attempts = 0,
			full_scan = jetstream_backfill.full_scan OR EXCLUDED.full_scan
	`, did)
	return result.RowsAffected(), err
}
