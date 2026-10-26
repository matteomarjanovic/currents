package main

import (
	"context"
	"errors"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/jackc/pgx/v5"
)

var errImportCancelled = errors.New("import no longer listing")

func (s *SessionJobStatus) summarize() {
	s.ListingIncomplete = s.ListingError != ""
	listed := s.Queued + s.Running + s.Done + s.Failed
	if s.Listed != nil {
		listed = *s.Listed
	}
	if s.Expected > 0 {
		s.Missing = max(0, s.Expected-listed)
	}
	if s.Status == "done" || s.Status == "failed" {
		s.Outcome = "complete"
		if s.Missing > 0 || s.Failed > 0 || s.ListingError != "" || s.Unsupported > 0 || s.Status == "failed" {
			s.Outcome = "partial"
			if s.Done == 0 {
				s.Outcome = "failed"
			}
		}
		s.CanRetry = s.Listed == nil || s.Missing > 0 || s.Failed > 0 || s.ListingError != "" || s.Status == "failed"
	}
}

// Queue a complete page and advance its cursor in the same transaction. The
// import history, rather than the current save table, remembers images a user
// deliberately moved or deleted after an earlier successful import.
func (m *PgStore) storeImportPage(ctx context.Context, jobID, ownerDID string, page PinterestPage, checkpoint bool) (int, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	// Serialize image deduplication across jobs/processes for this account.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 60))`, ownerDID); err != nil {
		return 0, err
	}
	var listing bool
	var target string
	if err = tx.QueryRow(ctx, `SELECT status = 'listing', target_collection_uri FROM import_job WHERE id=$1 AND owner_did=$2 FOR UPDATE`, jobID, ownerDID).Scan(&listing, &target); err != nil {
		return 0, err
	}
	if !listing {
		return 0, errImportCancelled
	}
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE _import_item_in (source_pin_id TEXT, image_url TEXT, source_url TEXT, rkey TEXT) ON COMMIT DROP`); err != nil {
		return 0, err
	}
	rows := make([][]any, 0, len(page.Pins))
	clock := syntax.NewTIDClock(0)
	for _, p := range page.Pins {
		rows = append(rows, []any{p.ID, p.ImageURL, p.SourceURL, clock.Next().String()})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"_import_item_in"}, []string{"source_pin_id", "image_url", "source_url", "rkey"}, pgx.CopyFromRows(rows)); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO import_item (job_id, owner_did, source_pin_id, image_url, source_url, rkey)
		SELECT $1, $2, p.source_pin_id, p.image_url, p.source_url, p.rkey
		FROM (SELECT DISTINCT ON (image_url) * FROM _import_item_in ORDER BY image_url, source_pin_id) p
		WHERE NOT EXISTS (
			SELECT 1 FROM import_item i JOIN import_job j ON j.id=i.job_id
			WHERE i.owner_did=$2 AND md5(i.image_url)=md5(p.image_url) AND i.image_url=p.image_url
			  AND j.target_collection_uri=$3 AND (i.job_id=$1 OR i.status IN ('done','queued','running'))
		)
		ON CONFLICT (job_id, source_pin_id) DO NOTHING`, jobID, ownerDID, target)
	if err != nil {
		return 0, err
	}
	if checkpoint {
		_, err = tx.Exec(ctx, `UPDATE import_job SET list_cursor=$2,
			listed_count=CASE WHEN $3 THEN $4 ELSE COALESCE(listed_count,0)+$4 END,
			unsupported_count=CASE WHEN $3 THEN $5 ELSE unsupported_count+$5 END,
			phase='reading', updated_at=now() WHERE id=$1`, jobID, page.Next, page.First, page.Count, page.Unsupported)
		if err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (m *PgStore) finishImportListing(ctx context.Context, id, message string) error {
	_, err := m.pool.Exec(ctx, `UPDATE import_job SET status='running', phase='saving', listing_error=$2, updated_at=now() WHERE id=$1 AND status='listing'`, id, message)
	return err
}

func (m *PgStore) setImportPhase(ctx context.Context, did, jobID, phase string) error {
	_, err := m.pool.Exec(ctx, `UPDATE import_job SET phase=$3 WHERE owner_did=$1
		AND ($2='' OR id::text=$2) AND status IN ('listing','running') AND phase<>$3`, did, jobID, phase)
	return err
}

func (m *PgStore) listImportSessions(ctx context.Context, did string) ([]ActiveImportSession, error) {
	rows, err := m.pool.Query(ctx, `SELECT s.id, s.username, s.created_at,
		EXISTS(SELECT 1 FROM import_job j WHERE j.session_id=s.id AND j.status IN ('listing','running')) AS active
		FROM import_session s WHERE s.owner_did=$1 ORDER BY active DESC, s.created_at DESC LIMIT 20`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActiveImportSession{}
	for rows.Next() {
		var s ActiveImportSession
		if err := rows.Scan(&s.SessionID, &s.Username, &s.StartedAt, &s.Active); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Caller verifies destinations against the PDS while holding lockRepository.
// Completed items remain untouched, including historical saves now deleted.
func (m *PgStore) retryImportJobs(ctx context.Context, sessionID, did string, ids []string) (int, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `UPDATE import_job j SET status='listing', list_cursor='',
		listed_count=NULL, unsupported_count=0, listing_error='', error='', phase='reading',
		previously_done=(SELECT count(*) FROM import_item i WHERE i.job_id=j.id AND i.status='done'), updated_at=now()
		WHERE j.session_id=$1 AND j.owner_did=$2 AND j.id::text=ANY($3) AND j.status IN ('done','failed')
		AND NOT EXISTS(SELECT 1 FROM collection_delete_job d WHERE d.collection_uri=j.target_collection_uri)
		RETURNING j.id::text`, sessionID, did, ids)
	if err != nil {
		return 0, err
	}
	retried := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		retried = append(retried, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `UPDATE import_item SET status='queued', attempt_count=0, error='', updated_at=now() WHERE job_id::text=ANY($1) AND status='failed'`, retried); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(retried), nil
}
