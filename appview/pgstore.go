package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type PgStoreConfig struct {
	DSN string

	SessionExpiryDuration     time.Duration
	SessionInactivityDuration time.Duration
	AuthRequestExpiryDuration time.Duration
}

// Implements the [oauth.ClientAuthStore] interface, backed by PostgreSQL via pgx
type PgStore struct {
	pool *pgxpool.Pool
	cfg  *PgStoreConfig
}

var _ oauth.ClientAuthStore = &PgStore{}

func NewPgStore(ctx context.Context, cfg *PgStoreConfig) (*PgStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("missing cfg")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("missing DSN")
	}
	if cfg.SessionExpiryDuration == 0 {
		return nil, fmt.Errorf("missing SessionExpiryDuration")
	}
	if cfg.SessionInactivityDuration == 0 {
		return nil, fmt.Errorf("missing SessionInactivityDuration")
	}
	if cfg.AuthRequestExpiryDuration == 0 {
		return nil, fmt.Errorf("missing AuthRequestExpiryDuration")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed parsing db config: %w", err)
	}
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
			return err
		}
		return pgxvector.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to db: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed pinging db: %w", err)
	}

	store := &PgStore{pool: pool, cfg: cfg}
	if err := store.migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed migrating db: %w", err)
	}

	return store, nil
}

func (s *PgStore) migrate(_ context.Context) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	dsn := s.cfg.DSN
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(dsn, prefix) {
			dsn = "pgx5://" + dsn[len(prefix):]
			break
		}
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

type UserRecord struct {
	DID          string
	Handle       string
	DisplayName  string
	Description  string
	Pronouns     string
	Website      string
	Avatar       string
	Banner       string
	CreatedAt    time.Time
	PDSEndpoint  string
}

func (m *PgStore) CreateUser(ctx context.Context, u UserRecord) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO "user" (did, handle, display_name, description, pronouns, website, avatar, banner, created_at, pds_endpoint)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (did) DO UPDATE
			SET handle       = EXCLUDED.handle,
			    display_name = EXCLUDED.display_name,
			    avatar       = EXCLUDED.avatar,
			    pds_endpoint = EXCLUDED.pds_endpoint
	`, u.DID, u.Handle, u.DisplayName, u.Description, u.Pronouns, u.Website, u.Avatar, u.Banner, u.CreatedAt, u.PDSEndpoint)
	return err
}

type ActorRow struct {
	DID         string
	Handle      string
	DisplayName string
	Avatar      string
}

func (m *PgStore) GetActorByDID(ctx context.Context, did string) (*ActorRow, error) {
	var row ActorRow
	err := m.pool.QueryRow(ctx,
		`SELECT did, COALESCE(handle, ''), COALESCE(display_name, ''), COALESCE(avatar, '') FROM "user" WHERE did = $1`,
		did,
	).Scan(&row.DID, &row.Handle, &row.DisplayName, &row.Avatar)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (m *PgStore) GetSession(ctx context.Context, did syntax.DID, sessionID string) (*oauth.ClientSessionData, error) {
	expiryThreshold := time.Now().Add(-m.cfg.SessionExpiryDuration)
	inactiveThreshold := time.Now().Add(-m.cfg.SessionInactivityDuration)
	_, _ = m.pool.Exec(ctx,
		`DELETE FROM oauth_sessions WHERE created_at < $1 OR updated_at < $2`,
		expiryThreshold, inactiveThreshold,
	)

	row := m.pool.QueryRow(ctx,
		`SELECT data FROM oauth_sessions WHERE account_did = $1 AND session_id = $2`,
		did.String(), sessionID,
	)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	var data oauth.ClientSessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("deserializing session data: %w", err)
	}
	return &data, nil
}

func (m *PgStore) SaveSession(ctx context.Context, sess oauth.ClientSessionData) error {
	raw, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("serializing session data: %w", err)
	}
	_, err = m.pool.Exec(ctx, `
		INSERT INTO oauth_sessions (account_did, session_id, data, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (account_did, session_id) DO UPDATE
			SET data = EXCLUDED.data, updated_at = now()
	`, sess.AccountDID.String(), sess.SessionID, raw)
	return err
}

func (m *PgStore) DeleteSession(ctx context.Context, did syntax.DID, sessionID string) error {
	_, err := m.pool.Exec(ctx,
		`DELETE FROM oauth_sessions WHERE account_did = $1 AND session_id = $2`,
		did.String(), sessionID,
	)
	return err
}

func (m *PgStore) GetAuthRequestInfo(ctx context.Context, state string) (*oauth.AuthRequestData, error) {
	threshold := time.Now().Add(-m.cfg.AuthRequestExpiryDuration)
	_, _ = m.pool.Exec(ctx,
		`DELETE FROM oauth_auth_requests WHERE created_at < $1`,
		threshold,
	)

	row := m.pool.QueryRow(ctx,
		`SELECT data FROM oauth_auth_requests WHERE state = $1`,
		state,
	)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	var data oauth.AuthRequestData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("deserializing auth request data: %w", err)
	}
	return &data, nil
}

func (m *PgStore) SaveAuthRequestInfo(ctx context.Context, info oauth.AuthRequestData) error {
	raw, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("serializing auth request data: %w", err)
	}
	_, err = m.pool.Exec(ctx,
		`INSERT INTO oauth_auth_requests (state, data, created_at) VALUES ($1, $2, now())`,
		info.State, raw,
	)
	return err
}

func (m *PgStore) DeleteAuthRequestInfo(ctx context.Context, state string) error {
	_, err := m.pool.Exec(ctx,
		`DELETE FROM oauth_auth_requests WHERE state = $1`,
		state,
	)
	return err
}

func (m *PgStore) UpsertCollection(ctx context.Context, uri, cid, authorDID, name, description string, createdAt *time.Time) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO collection (uri, cid, author_did, name, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (uri) DO UPDATE
			SET cid = EXCLUDED.cid, name = EXCLUDED.name, description = EXCLUDED.description
	`, uri, cid, authorDID, name, description, createdAt)
	return err
}

type CollectionRow struct {
	URI          string
	CID          string
	Name         string
	Description  string
	CreatedAt    *time.Time
	SaveCount    int
	PreviewBlobs []string // up to 4; each is "did,cid"
	Starred      *bool    // nil when unauthenticated
}

func (m *PgStore) GetCollectionsPage(ctx context.Context, authorDID string, limit int, cursor string) ([]CollectionRow, string, error) {
	var args []any
	args = append(args, authorDID)
	args = append(args, limit)

	cursorClause := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(raw), "|", 2)
			if len(parts) == 2 {
				ts, err := time.Parse(time.RFC3339Nano, parts[0])
				if err == nil {
					args = append(args, ts.UTC(), parts[1])
					cursorClause = fmt.Sprintf(" AND (c.created_at < $%d OR (c.created_at = $%d AND c.uri > $%d))", len(args)-1, len(args)-1, len(args))
				}
			}
		}
	}

	query := fmt.Sprintf(`
		SELECT
			c.uri,
			c.cid,
			c.name,
			COALESCE(c.description, ''),
			c.created_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri)::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE s2.collection_uri = c.uri
				ORDER BY s2.quality_score DESC NULLS LAST, s2.uri ASC
				LIMIT 4
			) AS preview_blobs
		FROM collection c
		WHERE c.author_did = $1
		  AND c.cid IS NOT NULL
		  %s
		ORDER BY c.created_at DESC NULLS LAST, c.uri ASC
		LIMIT $2
	`, cursorClause)

	rows, err := m.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []CollectionRow
	for rows.Next() {
		var row CollectionRow
		if err := rows.Scan(&row.URI, &row.CID, &row.Name, &row.Description, &row.CreatedAt, &row.SaveCount, &row.PreviewBlobs); err != nil {
			return nil, "", err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(result) == limit {
		last := result[len(result)-1]
		ts := ""
		if last.CreatedAt != nil {
			ts = last.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(ts + "|" + last.URI))
	}
	return result, nextCursor, nil
}

func (m *PgStore) GetActorCollectionsPage(ctx context.Context, actorDID, viewerDID string, limit int, cursor string) ([]CollectionRow, string, error) {
	var args []any
	args = append(args, actorDID)
	args = append(args, limit)
	args = append(args, viewerDID) // $3 — empty string means unauthenticated

	cursorClause := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(raw), "|", 2)
			if len(parts) == 2 {
				ts, err := time.Parse(time.RFC3339Nano, parts[0])
				if err == nil {
					args = append(args, ts.UTC(), parts[1])
					cursorClause = fmt.Sprintf(" AND (c.created_at < $%d OR (c.created_at = $%d AND c.uri > $%d))", len(args)-1, len(args)-1, len(args))
				}
			}
		}
	}

	query := fmt.Sprintf(`
		SELECT
			c.uri,
			c.cid,
			c.name,
			COALESCE(c.description, ''),
			c.created_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri)::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE s2.collection_uri = c.uri
				ORDER BY s2.quality_score DESC NULLS LAST, s2.uri ASC
				LIMIT 4
			) AS preview_blobs,
			CASE WHEN $3 != '' THEN (sc.viewer_did IS NOT NULL) END AS starred
		FROM collection c
		LEFT JOIN starred_collection sc
			ON sc.collection_uri = c.uri AND sc.viewer_did = NULLIF($3, '')
		WHERE c.author_did = $1
		  AND c.cid IS NOT NULL
		  %s
		ORDER BY c.created_at DESC NULLS LAST, c.uri ASC
		LIMIT $2
	`, cursorClause)

	rows, err := m.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []CollectionRow
	for rows.Next() {
		var row CollectionRow
		if err := rows.Scan(&row.URI, &row.CID, &row.Name, &row.Description, &row.CreatedAt, &row.SaveCount, &row.PreviewBlobs, &row.Starred); err != nil {
			return nil, "", err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(result) == limit {
		last := result[len(result)-1]
		ts := ""
		if last.CreatedAt != nil {
			ts = last.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(ts + "|" + last.URI))
	}
	return result, nextCursor, nil
}

func (m *PgStore) DeleteCollection(ctx context.Context, uri string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM collection WHERE uri = $1`, uri)
	return err
}

// GetCollectionByURI returns the collection row for the given URI, or nil if not found.
// viewerDID may be empty for unauthenticated requests.
func (m *PgStore) GetCollectionByURI(ctx context.Context, collectionURI, viewerDID string) (*CollectionRow, error) {
	query := `
		SELECT
			c.uri,
			c.cid,
			c.name,
			COALESCE(c.description, ''),
			c.created_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri)::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE s2.collection_uri = c.uri
				ORDER BY s2.quality_score DESC NULLS LAST, s2.uri ASC
				LIMIT 4
			) AS preview_blobs,
			CASE WHEN $2 != '' THEN (sc.viewer_did IS NOT NULL) END AS starred
		FROM collection c
		LEFT JOIN starred_collection sc
			ON sc.collection_uri = c.uri AND sc.viewer_did = NULLIF($2, '')
		WHERE c.uri = $1
		  AND c.cid IS NOT NULL
	`
	var row CollectionRow
	err := m.pool.QueryRow(ctx, query, collectionURI, viewerDID).
		Scan(&row.URI, &row.CID, &row.Name, &row.Description, &row.CreatedAt, &row.SaveCount, &row.PreviewBlobs, &row.Starred)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

type SaveRow struct {
	URI                string
	BlobCID            string
	AuthorDID          string
	Text               string
	OriginURL          string
	AttributionURL     string
	AttributionLicense string
	AttributionCredit  string
	ResaveOfURI        string
	ResaveOfCID        string          // pds_blob_cid of the referenced save; empty if not in DB
	CreatedAt          *time.Time
	ViewerSaves        json.RawMessage // null when unauthenticated; [{collectionUri,saveUri},...] when authenticated
	Width              *int
	Height             *int
	DominantColors     json.RawMessage // nil when visual identity not yet resolved
}

func (m *PgStore) GetSavesByURIs(ctx context.Context, saveURIs []string, viewerDID string) ([]SaveRow, error) {
	query := `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(ros.pds_blob_cid, ''),
			s.created_at,
			CASE WHEN $2 != '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $2 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			s.width,
			s.height,
			s.dominant_colors
		FROM save s
		LEFT JOIN save ros ON ros.uri = s.resave_of_uri
		WHERE s.uri = ANY($1)
	`
	rows, err := m.pool.Query(ctx, query, saveURIs, viewerDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.Width, &row.Height, &row.DominantColors); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *PgStore) GetSavesPage(ctx context.Context, collectionURI, viewerDID string, limit int, cursor string) ([]SaveRow, string, error) {
	var args []any
	args = append(args, collectionURI)
	args = append(args, limit)
	args = append(args, viewerDID)

	cursorClause := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(raw), "|", 2)
			if len(parts) == 2 {
				ts, err := time.Parse(time.RFC3339Nano, parts[0])
				if err == nil {
					args = append(args, ts.UTC(), parts[1])
					cursorClause = fmt.Sprintf(" AND (s.created_at < $%d OR (s.created_at = $%d AND s.uri > $%d))", len(args)-1, len(args)-1, len(args))
				}
			}
		}
	}

	query := fmt.Sprintf(`
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(ros.pds_blob_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			s.width,
			s.height,
			s.dominant_colors
		FROM save s
		LEFT JOIN save ros ON ros.uri = s.resave_of_uri
		WHERE s.collection_uri = $1
		  %s
		ORDER BY s.created_at DESC NULLS LAST, s.uri ASC
		LIMIT $2
	`, cursorClause)

	rows, err := m.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.Width, &row.Height, &row.DominantColors); err != nil {
			return nil, "", err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(result) == limit {
		last := result[len(result)-1]
		ts := ""
		if last.CreatedAt != nil {
			ts = last.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(ts + "|" + last.URI))
	}
	return result, nextCursor, nil
}

type UpsertSaveParams struct {
	URI                string
	AuthorDID          string
	CollectionURI      string
	PdsBlobCID         string
	Text               string
	OriginURL          string
	AttributionURL     string
	AttributionLicense string
	AttributionCredit  string
	ResaveOfURI        string
	CreatedAt          *time.Time
	VisualIdentityID   *string
	QualityScore       *float32
	Width              *int
	Height             *int
	DominantColors     json.RawMessage
}

func (m *PgStore) UpsertSave(ctx context.Context, p UpsertSaveParams) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO save (uri, author_did, collection_uri, pds_blob_cid, text, origin_url, attribution_url, attribution_license, attribution_credit, resave_of_uri, created_at, visual_identity_id, quality_score, width, height, dominant_colors)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (uri) DO UPDATE
			SET collection_uri      = EXCLUDED.collection_uri,
			    pds_blob_cid        = EXCLUDED.pds_blob_cid,
			    text                = EXCLUDED.text,
			    origin_url          = EXCLUDED.origin_url,
			    attribution_url     = EXCLUDED.attribution_url,
			    attribution_license = EXCLUDED.attribution_license,
			    attribution_credit  = EXCLUDED.attribution_credit,
			    resave_of_uri       = EXCLUDED.resave_of_uri,
			    visual_identity_id  = EXCLUDED.visual_identity_id,
			    quality_score       = EXCLUDED.quality_score,
			    width               = EXCLUDED.width,
			    height              = EXCLUDED.height,
			    dominant_colors     = EXCLUDED.dominant_colors
	`, p.URI, p.AuthorDID, p.CollectionURI, p.PdsBlobCID, p.Text, p.OriginURL, p.AttributionURL, p.AttributionLicense, p.AttributionCredit, p.ResaveOfURI, p.CreatedAt, p.VisualIdentityID, p.QualityScore, p.Width, p.Height, []byte(p.DominantColors))
	return err
}

// DeleteSave deletes a save and re-elects the canonical blob for its visual identity if needed.
func (m *PgStore) DeleteSave(ctx context.Context, uri string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Fetch the save's VI link and blob info before deleting.
	var viID *string
	var authorDID, pdsBlobCID string
	err = tx.QueryRow(ctx,
		`SELECT visual_identity_id, author_did, pds_blob_cid FROM save WHERE uri = $1`,
		uri,
	).Scan(&viID, &authorDID, &pdsBlobCID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tx.Commit(ctx) // already gone
		}
		return err
	}

	// Delete the save (trigger decrements save_count).
	if _, err := tx.Exec(ctx, `DELETE FROM save WHERE uri = $1`, uri); err != nil {
		return err
	}

	if viID != nil {
		// Check if deleted save was the current canonical.
		var canonicalCID *string
		if err := tx.QueryRow(ctx,
			`SELECT canonical_blob_cid FROM visual_identity WHERE id = $1`,
			*viID,
		).Scan(&canonicalCID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if canonicalCID != nil && *canonicalCID == pdsBlobCID {
			// Re-elect: find the best remaining save.
			var newDID, newCID, newURI *string
			_ = tx.QueryRow(ctx, `
				SELECT author_did, pds_blob_cid, uri FROM save
				WHERE visual_identity_id = $1
				ORDER BY quality_score DESC NULLS LAST
				LIMIT 1
			`, *viID).Scan(&newDID, &newCID, &newURI)

			if _, err := tx.Exec(ctx, `
				UPDATE visual_identity
				SET canonical_blob_did = $2, canonical_blob_cid = $3, canonical_save_uri = $4
				WHERE id = $1
			`, *viID, newDID, newCID, newURI); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// ── Visual identity methods ───────────────────────────────────────────────────

// GetSaveViIDAndQuality returns the visual_identity_id, quality_score, and image metadata of a save by URI.
func (m *PgStore) GetSaveViIDAndQuality(ctx context.Context, uri string) (*string, *float32, *int, *int, json.RawMessage, error) {
	var viID *string
	var qs *float32
	var width, height *int
	var colors json.RawMessage
	err := m.pool.QueryRow(ctx,
		`SELECT visual_identity_id, quality_score, width, height, dominant_colors FROM save WHERE uri = $1`,
		uri,
	).Scan(&viID, &qs, &width, &height, &colors)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return viID, qs, width, height, colors, nil
}

// GetViIDAndQualityByCID returns the visual_identity_id, quality_score, and image metadata of any save
// sharing the given blob CID (same CID = same image = same dimensions/colors).
func (m *PgStore) GetViIDAndQualityByCID(ctx context.Context, pdsBlobCID string) (*string, *float32, *int, *int, json.RawMessage, error) {
	var viID *string
	var qs *float32
	var width, height *int
	var colors json.RawMessage
	err := m.pool.QueryRow(ctx, `
		SELECT visual_identity_id, quality_score, width, height, dominant_colors FROM save
		WHERE pds_blob_cid = $1 AND visual_identity_id IS NOT NULL
		LIMIT 1
	`, pdsBlobCID).Scan(&viID, &qs, &width, &height, &colors)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return viID, qs, width, height, colors, nil
}

// FindNearestVI finds the nearest visual identity by cosine distance.
// Returns nil if no VI is within the given threshold distance.
func (m *PgStore) FindNearestVI(ctx context.Context, embedding []float32, threshold float32) (*string, error) {
	vec := pgvector.NewVector(embedding)
	var id string
	var dist float32
	err := m.pool.QueryRow(ctx, `
		SELECT id, embedding <=> $1 AS dist
		FROM visual_identity
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT 1
	`, vec).Scan(&id, &dist)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if dist > threshold {
		return nil, nil
	}
	return &id, nil
}

// CreateVI inserts a new visual_identity row and returns its UUID.
// canonical_save_uri is set separately via SetVICanonicalSave after the save is upserted.
func (m *PgStore) CreateVI(ctx context.Context, blobDID, blobCID string, embedding []float32) (string, error) {
	var id string
	err := m.pool.QueryRow(ctx, `
		INSERT INTO visual_identity (canonical_blob_did, canonical_blob_cid, embedding)
		VALUES ($1, $2, $3)
		RETURNING id
	`, blobDID, blobCID, pgvector.NewVector(embedding)).Scan(&id)
	return id, err
}

// SetVICanonicalSave sets canonical_save_uri on a visual_identity row.
// Called after UpsertSave for newly created VIs.
func (m *PgStore) SetVICanonicalSave(ctx context.Context, viID, saveURI string) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE visual_identity SET canonical_save_uri = $2 WHERE id = $1
	`, viID, saveURI)
	return err
}

// MaybePromoteCanonical updates the canonical blob (and canonical save) of a VI if the new quality score is strictly better.
func (m *PgStore) MaybePromoteCanonical(ctx context.Context, viID, blobDID, blobCID, saveURI string, score float32) error {
	// Fetch current canonical's quality score.
	var currentScore *float32
	_ = m.pool.QueryRow(ctx, `
		SELECT quality_score FROM save
		WHERE visual_identity_id = $1 AND pds_blob_cid = (
			SELECT canonical_blob_cid FROM visual_identity WHERE id = $1
		)
		ORDER BY quality_score DESC NULLS LAST
		LIMIT 1
	`, viID).Scan(&currentScore)

	if currentScore != nil && *currentScore >= score {
		return nil
	}

	_, err := m.pool.Exec(ctx, `
		UPDATE visual_identity
		SET canonical_blob_did = $2, canonical_blob_cid = $3, canonical_save_uri = $4
		WHERE id = $1
	`, viID, blobDID, blobCID, saveURI)
	return err
}

// SearchSavesByEmbedding returns saves whose visual identity is nearest to the given embedding,
// ordered by cosine distance. Offset-based pagination; pass offset=0 for the first page.
func (m *PgStore) SearchSavesByEmbedding(ctx context.Context, embedding []float32, viewerDID string, limit, offset int) ([]SaveRow, error) {
	vec := pgvector.NewVector(embedding)
	rows, err := m.pool.Query(ctx, `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(ros.pds_blob_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			s.width,
			s.height,
			s.dominant_colors
		FROM visual_identity vi
		JOIN save s ON s.uri = vi.canonical_save_uri
		LEFT JOIN save ros ON ros.uri = s.resave_of_uri
		WHERE vi.embedding IS NOT NULL
		ORDER BY vi.embedding <=> $1
		LIMIT $2 OFFSET $4
	`, vec, limit, viewerDID, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.Width, &row.Height, &row.DominantColors); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ── Feed methods ─────────────────────────────────────────────────────────────


// GetCollectionEmbeddings returns all visual-identity embeddings for saves in a collection.
// Used to compute the collection's canonical (medoid) embedding.
func (m *PgStore) GetCollectionEmbeddings(ctx context.Context, collectionURI string) ([][]float32, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT vi.embedding
		FROM save s
		JOIN visual_identity vi ON vi.id = s.visual_identity_id
		WHERE s.collection_uri = $1
		  AND vi.embedding IS NOT NULL
	`, collectionURI)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result [][]float32
	for rows.Next() {
		var vec pgvector.Vector
		if err := rows.Scan(&vec); err != nil {
			return nil, err
		}
		result = append(result, vec.Slice())
	}
	return result, rows.Err()
}

// UpdateCollectionEmbedding stores the precomputed canonical embedding for a collection.
func (m *PgStore) UpdateCollectionEmbedding(ctx context.Context, collectionURI string, embedding []float32) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE collection SET canonical_embedding = $2 WHERE uri = $1
	`, collectionURI, pgvector.NewVector(embedding))
	return err
}

// CollectionImportance pairs a collection URI with its precomputed canonical embedding.
type CollectionImportance struct {
	URI       string
	Embedding []float32
}

// GetCollectionsByImportance returns the viewer's top-N collections ranked by time-decayed save count,
// filtered to those that have a precomputed canonical embedding.
func (m *PgStore) GetCollectionsByImportance(ctx context.Context, viewerDID string, topN int) ([]CollectionImportance, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT c.uri, c.canonical_embedding
		FROM save s
		JOIN collection c ON c.uri = s.collection_uri
		WHERE s.author_did = $1
		  AND s.created_at IS NOT NULL
		  AND c.canonical_embedding IS NOT NULL
		GROUP BY c.uri, c.canonical_embedding
		ORDER BY SUM(EXP(-0.01 * EXTRACT(EPOCH FROM (NOW() - s.created_at)) / 86400)) DESC
		LIMIT $2
	`, viewerDID, topN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CollectionImportance
	for rows.Next() {
		var uri string
		var vec pgvector.Vector
		if err := rows.Scan(&uri, &vec); err != nil {
			return nil, err
		}
		result = append(result, CollectionImportance{URI: uri, Embedding: vec.Slice()})
	}
	return result, rows.Err()
}

// GetGlobalFeedSaves returns saves from across the network, ranked by a
// time-decayed popularity score: save_count * exp(-0.01 * age_in_days).
// No minimum save_count threshold — all images with a visual identity appear,
// with popular recent images ranked highest.
func (m *PgStore) GetGlobalFeedSaves(ctx context.Context, viewerDID string, limit, offset int) ([]SaveRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(ros.pds_blob_cid, ''),
			s.created_at,
			CASE WHEN $1 != '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $1 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			s.width,
			s.height,
			s.dominant_colors
		FROM visual_identity vi
		JOIN save s ON s.uri = vi.canonical_save_uri
		LEFT JOIN save ros ON ros.uri = s.resave_of_uri
		ORDER BY (vi.save_count * EXP(-0.01 * EXTRACT(EPOCH FROM (NOW() - s.created_at)) / 86400)) DESC
		LIMIT $2 OFFSET $3
	`, viewerDID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.Width, &row.Height, &row.DominantColors); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetUserPDSEndpoint returns the PDS endpoint for a known user from the local DB.
func (m *PgStore) GetUserPDSEndpoint(ctx context.Context, did string) (string, error) {
	var endpoint string
	err := m.pool.QueryRow(ctx,
		`SELECT COALESCE(pds_endpoint, '') FROM "user" WHERE did = $1`,
		did,
	).Scan(&endpoint)
	return endpoint, err
}
