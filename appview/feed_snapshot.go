package main

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const globalFeedRefreshInterval = 3 * time.Hour
const globalFeedRefreshLock int64 = 0x6375727266656564

var errFeedCursorExpired = errors.New("feed cursor expired")

type globalFeedPage struct {
	SnapshotID int64
	Rows       []SaveRow
	Positions  []int
}

// RefreshGlobalFeed publishes a complete ranking atomically, retaining the last
// two versions. A failed build rolls back, leaving the previous version usable.
func (m *PgStore) RefreshGlobalFeed(ctx context.Context, now time.Time) (int64, int64, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, globalFeedRefreshLock).Scan(&locked); err != nil || !locked {
		return 0, 0, err
	}
	var latest time.Time
	err = tx.QueryRow(ctx, `SELECT created_at FROM global_feed_snapshot ORDER BY id DESC LIMIT 1`).Scan(&latest)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	if err == nil && now.Sub(latest) < globalFeedRefreshInterval {
		return 0, 0, nil
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO global_feed_snapshot (created_at) VALUES ($1) RETURNING id`, now).Scan(&id); err != nil {
		return 0, 0, err
	}
	// All users share the three-hour seed. Visibility is intentionally not baked
	// into the snapshot: a takedown, unhide or junk-score change takes effect live.
	seed := strconv.FormatInt(now.Unix()/int64(globalFeedRefreshInterval/time.Second), 10)
	tag, err := tx.Exec(ctx, `
		INSERT INTO global_feed_rank (snapshot_id, position, visual_identity_id)
		SELECT $1, ROW_NUMBER() OVER (ORDER BY (
			COALESCE(LN(NULLIF(vi.saver_count, 0)::double precision), '-Infinity'::double precision)
			- 0.1::double precision * EXTRACT(EPOCH FROM ($2::timestamptz - s.created_at))::double precision / 86400
			+ LN(0.8::double precision + 0.4::double precision * (hashtext(s.uri || $3) & 2147483647) / 2147483648.0)
		) DESC, s.uri, vi.id), vi.id
		FROM visual_identity vi
		JOIN save s ON s.uri = vi.canonical_save_uri
		WHERE s.created_at IS NOT NULL
	`, id, now, seed)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM global_feed_snapshot
		WHERE id < (SELECT id FROM global_feed_snapshot ORDER BY id DESC OFFSET 1 LIMIT 1)
	`); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return id, tag.RowsAffected(), nil
}

func (m *PgStore) RunGlobalFeedRefresh(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for ctx.Err() == nil {
		started := time.Now()
		buildCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		id, count, err := m.RefreshGlobalFeed(buildCtx, started)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Error("global feed refresh failed", "err", err)
		} else if id != 0 {
			slog.Info("global feed refreshed", "snapshot", id, "images", count, "duration", time.Since(started))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// GetGlobalFeedPage uses absolute snapshot positions, so removing or hiding an
// earlier image cannot shift an offset and skip an unseen image on the next page.
func (m *PgStore) GetGlobalFeedPage(ctx context.Context, viewerDID string, excludeViewerSaves bool, snapshotID int64, after, limit int) (globalFeedPage, error) {
	page := globalFeedPage{}
	// Keep the generation lookup and page read on one snapshot even if cleanup
	// retires this generation between the two statements.
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return page, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		SELECT id FROM global_feed_snapshot
		WHERE ($1::bigint = 0 OR id = $1) ORDER BY id DESC LIMIT 1
	`, snapshotID).Scan(&page.SnapshotID)
	if errors.Is(err, pgx.ErrNoRows) {
		if snapshotID != 0 {
			return page, errFeedCursorExpired
		}
		return page, nil // First build is still running; the caller uses the live feed.
	}
	if err != nil {
		return page, err
	}
	excludeClause := ""
	if excludeViewerSaves && viewerDID != "" {
		excludeClause = `AND NOT EXISTS (
			SELECT 1 FROM save v WHERE v.author_did = $3 AND v.pds_blob_cid = s.pds_blob_cid
		)`
	}
	rows, err := tx.Query(ctx, `
		WITH page AS MATERIALIZED (
			SELECT r.position, s.uri
			FROM global_feed_rank r
			JOIN visual_identity vi ON vi.id = r.visual_identity_id
			JOIN save s ON s.uri = vi.canonical_save_uri
			WHERE r.snapshot_id = $1 AND r.position > $2
			  AND s.created_at IS NOT NULL
			  AND s.author_did <> ALL($5)
			  AND (vi.junk_score IS NULL OR vi.junk_score < $6)
			  AND NOT EXISTS (
				SELECT 1 FROM hidden_feed_image h WHERE h.viewer_did = $3 AND h.visual_identity_id = vi.id
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked'
			  )
			  `+excludeClause+`
			ORDER BY r.position LIMIT $4
		)
		SELECT `+saveRowProjection+`, page.position
		FROM page JOIN save s ON s.uri = page.uri
		ORDER BY page.position
	`, page.SnapshotID, after, viewerDID, limit, m.cfg.HiddenDIDs, feedJunkScoreMax)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var row SaveRow
		var position int
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.OriginAtprotoURI, &row.OriginAtprotoCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText, &row.MimeType, &position); err != nil {
			rows.Close()
			return page, err
		}
		page.Rows = append(page.Rows, row)
		page.Positions = append(page.Positions, position)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if err := tx.Commit(ctx); err != nil {
		return page, err
	}
	return page, nil
}
