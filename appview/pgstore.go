package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	MinConns                  int32
	MaxConns                  int32
	MaxConnLifetime           time.Duration
	MaxConnIdleTime           time.Duration

	// HiddenDIDs are author DIDs whose saves are filtered out of feed/search
	// results. Temporary stopgap until proper moderation exists.
	HiddenDIDs []string
}

// Implements the [oauth.ClientAuthStore] interface, backed by PostgreSQL via pgx
type PgStore struct {
	pool *pgxpool.Pool
	cfg  *PgStoreConfig
}

type BlobSourceCandidate struct {
	URI       string
	AuthorDID string
}

type SaveBackfillMetrics struct {
	MissingVisualIdentityCount       int64
	DistinctMissingBlobCIDCount      int64
	CollectionsMissingEmbeddingCount int64
	OldestMissingCreatedAt           *time.Time
}

type annSavePage struct {
	Rows    []SaveRow
	HasMore bool
}

type BackgroundMetrics struct {
	Saves SaveBackfillMetrics
}

type RepairStats struct {
	BlobCandidates        int64
	BlobEnriched          int64
	CollectionCandidates  int64
	CollectionsRecomputed int64
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
	if cfg.MinConns < 0 {
		return nil, fmt.Errorf("MinConns must be >= 0")
	}
	if cfg.MaxConns < 0 {
		return nil, fmt.Errorf("MaxConns must be >= 0")
	}
	if cfg.MaxConns > 0 && cfg.MinConns > cfg.MaxConns {
		return nil, fmt.Errorf("MinConns cannot be greater than MaxConns")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed parsing db config: %w", err)
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
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
	DID         string
	Handle      string
	DisplayName string
	Description string
	Pronouns    string
	Website     string
	Avatar      string
	Banner      string
	CreatedAt   time.Time
	PDSEndpoint string
}

func (m *PgStore) CreateUser(ctx context.Context, u UserRecord) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO "user" (did, handle, display_name, description, pronouns, website, avatar, banner, created_at, pds_endpoint)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (did) DO UPDATE
			SET handle       = EXCLUDED.handle,
			    display_name = EXCLUDED.display_name,
			    description  = EXCLUDED.description,
			    pronouns     = EXCLUDED.pronouns,
			    website      = EXCLUDED.website,
			    avatar       = EXCLUDED.avatar,
			    banner       = EXCLUDED.banner,
			    pds_endpoint = EXCLUDED.pds_endpoint
	`, u.DID, u.Handle, u.DisplayName, u.Description, u.Pronouns, u.Website, u.Avatar, u.Banner, u.CreatedAt, u.PDSEndpoint)
	return err
}

type ActorRow struct {
	DID         string
	Handle      string
	DisplayName string
	Description string
	Pronouns    string
	Website     string
	Avatar      string
	Banner      string
	CreatedAt   *time.Time
}

func (m *PgStore) GetActorByDID(ctx context.Context, did string) (*ActorRow, error) {
	var row ActorRow
	err := m.pool.QueryRow(ctx,
		`SELECT did, COALESCE(handle, ''), COALESCE(display_name, ''), COALESCE(description, ''), COALESCE(pronouns, ''), COALESCE(website, ''), COALESCE(avatar, ''), COALESCE(banner, ''), created_at FROM "user" WHERE did = $1`,
		did,
	).Scan(&row.DID, &row.Handle, &row.DisplayName, &row.Description, &row.Pronouns, &row.Website, &row.Avatar, &row.Banner, &row.CreatedAt)
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

// LatestOAuthSessionID returns the most recently updated OAuth session for did,
// or "" when the user has none stored. The background import worker uses this to
// always act under the user's current session rather than a session id pinned
// when the job was created — which may since have been rotated by re-login.
func (m *PgStore) LatestOAuthSessionID(ctx context.Context, did string) (string, error) {
	var sid string
	err := m.pool.QueryRow(ctx,
		`SELECT session_id FROM oauth_sessions WHERE account_did = $1 ORDER BY updated_at DESC LIMIT 1`,
		did,
	).Scan(&sid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return sid, err
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

func (m *PgStore) UpsertCollection(ctx context.Context, uri, cid, authorDID, name, description, parentURI string, createdAt *time.Time) error {
	var parent *string
	if parentURI != "" {
		parent = &parentURI
	}
	_, err := m.pool.Exec(ctx, `
		INSERT INTO collection (uri, cid, author_did, name, description, parent_uri, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (uri) DO UPDATE
			SET cid = EXCLUDED.cid, name = EXCLUDED.name, description = EXCLUDED.description, parent_uri = EXCLUDED.parent_uri
	`, uri, cid, authorDID, name, description, parent, createdAt)
	return err
}

type CollectionRow struct {
	URI            string
	CID            string
	AuthorDID      string // populated by search; empty for single-actor listings
	Name           string
	Description    string
	ParentURI      string // empty for root collections
	CreatedAt      *time.Time
	LastSavedAt    *time.Time // newest save in this collection or its sub-collections
	SaveCount      int
	PreviewBlobs   []string // up to 4; each is "did,cid"
	FavouriteCount int      // total favourites of this collection across the network
	FavouriteURI   *string  // AT-URI of the viewer's favourite record; nil if not favourited / unauthenticated
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
			COALESCE(c.parent_uri, ''),
			c.created_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri
			   OR collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE (s2.collection_uri = c.uri
				       OR s2.collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))
				  AND s2.content_nsid = 'is.currents.content.image'
				  AND s2.pds_blob_cid <> ''
				  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s2.pds_blob_cid AND b.harm_state = 'blocked')
				ORDER BY (s2.collection_uri = c.uri) DESC, s2.quality_score DESC NULLS LAST, s2.uri ASC
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
		if err := rows.Scan(&row.URI, &row.CID, &row.Name, &row.Description, &row.ParentURI, &row.CreatedAt, &row.SaveCount, &row.PreviewBlobs); err != nil {
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

// GetActorCollectionsPage lists an actor's collections. parent filters the
// result: "" returns all collections, "root" returns only root-level
// collections (no parent), and any other value returns the children of that
// parent collection URI.
func (m *PgStore) GetActorCollectionsPage(ctx context.Context, actorDID, viewerDID, parent string, limit int, cursor string) ([]CollectionRow, string, error) {
	var args []any
	args = append(args, actorDID)
	args = append(args, limit)
	args = append(args, viewerDID) // $3 — empty string means unauthenticated

	parentClause := ""
	switch parent {
	case "":
		// no filter
	case "root":
		parentClause = " AND c.parent_uri IS NULL"
	default:
		args = append(args, parent)
		parentClause = fmt.Sprintf(" AND c.parent_uri = $%d", len(args))
	}

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

	// Correlated subqueries per collection (save_count / last_saved / previews, each with
	// an OR-rollup into sections) are O(collections × save-scans) and collapse on users
	// with many collections. Instead: page the collections first, map each collection and
	// its sections to the page collection it feeds ("scope"), then aggregate saves once.
	query := fmt.Sprintf(`
		WITH cols AS (
			SELECT c.uri, c.cid, c.name, c.description, c.parent_uri, c.created_at
			FROM collection c
			WHERE c.author_did = $1
			  AND c.cid IS NOT NULL
			  %s
			  %s
			ORDER BY c.created_at DESC NULLS LAST, c.uri ASC
			LIMIT $2
		),
		scope AS (
			SELECT uri AS curi, uri AS root FROM cols
			UNION ALL
			SELECT ch.uri, ch.parent_uri FROM collection ch WHERE ch.parent_uri IN (SELECT uri FROM cols)
		),
		save_stats AS (
			SELECT sc.root, count(*)::int AS cnt, max(s.created_at) AS last_saved
			FROM save s JOIN scope sc ON sc.curi = s.collection_uri
			GROUP BY sc.root
		),
		preview AS (
			SELECT sc.root, s.author_did || ',' || s.pds_blob_cid AS blob,
				ROW_NUMBER() OVER (
					PARTITION BY sc.root
					ORDER BY (sc.curi = sc.root) DESC, s.quality_score DESC NULLS LAST, s.uri ASC
				) AS rn
			FROM save s JOIN scope sc ON sc.curi = s.collection_uri
			WHERE s.content_nsid = 'is.currents.content.image'
			  AND s.pds_blob_cid <> ''
			  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		),
		preview_agg AS (
			SELECT root, array_agg(blob ORDER BY rn) AS blobs FROM preview WHERE rn <= 4 GROUP BY root
		)
		SELECT
			c.uri,
			c.cid,
			c.name,
			COALESCE(c.description, ''),
			COALESCE(c.parent_uri, ''),
			c.created_at,
			ss.last_saved AS last_saved_at,
			COALESCE(ss.cnt, 0) AS save_count,
			COALESCE(pa.blobs, ARRAY[]::text[]) AS preview_blobs,
			(SELECT COUNT(*) FROM favourite_collection WHERE collection_uri = c.uri)::int AS favourite_count,
			fc.uri AS favourite_uri
		FROM cols c
		LEFT JOIN save_stats ss ON ss.root = c.uri
		LEFT JOIN preview_agg pa ON pa.root = c.uri
		LEFT JOIN favourite_collection fc ON fc.collection_uri = c.uri AND fc.viewer_did = NULLIF($3, '')
		ORDER BY c.created_at DESC NULLS LAST, c.uri ASC
	`, parentClause, cursorClause)

	rows, err := m.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []CollectionRow
	for rows.Next() {
		var row CollectionRow
		if err := rows.Scan(&row.URI, &row.CID, &row.Name, &row.Description, &row.ParentURI, &row.CreatedAt, &row.LastSavedAt, &row.SaveCount, &row.PreviewBlobs, &row.FavouriteCount, &row.FavouriteURI); err != nil {
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

// GetFavouriteCollectionsPage returns collections that actorDID has favourited,
// most recently favourited first. viewerDID is the requesting account, used to
// hydrate each collection's FavouriteURI (may be empty when unauthenticated).
// AuthorDID is populated since favourited collections span many authors.
func (m *PgStore) GetFavouriteCollectionsPage(ctx context.Context, actorDID, viewerDID string, limit int, cursor string) ([]CollectionRow, string, error) {
	args := []any{actorDID, viewerDID}
	cursorClause := ""
	if cursor != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(cursor); err == nil {
			parts := strings.SplitN(string(raw), "|", 2)
			if len(parts) == 2 {
				if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
					args = append(args, ts.UTC(), parts[1])
					cursorClause = fmt.Sprintf(" AND (ofc.created_at < $%d OR (ofc.created_at = $%d AND c.uri > $%d))", len(args)-1, len(args)-1, len(args))
				}
			}
		}
	}
	args = append(args, limit)
	limitParam := len(args)

	// Same aggregate-once rewrite as GetActorCollectionsPage (avoids per-collection
	// correlated save subqueries): page the favourited collections, map them + their
	// sections to a scope, then aggregate saves in one pass.
	query := fmt.Sprintf(`
		WITH cols AS (
			SELECT c.uri, c.cid, c.author_did, c.name, c.description, c.parent_uri, c.created_at,
				ofc.created_at AS favourited_at
			FROM favourite_collection ofc
			JOIN collection c ON c.uri = ofc.collection_uri
			WHERE ofc.viewer_did = $1
			  AND c.cid IS NOT NULL
			  %s
			ORDER BY ofc.created_at DESC NULLS LAST, c.uri ASC
			LIMIT $%d
		),
		scope AS (
			SELECT uri AS curi, uri AS root FROM cols
			UNION ALL
			SELECT ch.uri, ch.parent_uri FROM collection ch WHERE ch.parent_uri IN (SELECT uri FROM cols)
		),
		save_stats AS (
			SELECT sc.root, count(*)::int AS cnt, max(s.created_at) AS last_saved
			FROM save s JOIN scope sc ON sc.curi = s.collection_uri
			GROUP BY sc.root
		),
		preview AS (
			SELECT sc.root, s.author_did || ',' || s.pds_blob_cid AS blob,
				ROW_NUMBER() OVER (
					PARTITION BY sc.root
					ORDER BY (sc.curi = sc.root) DESC, s.quality_score DESC NULLS LAST, s.uri ASC
				) AS rn
			FROM save s JOIN scope sc ON sc.curi = s.collection_uri
			WHERE s.content_nsid = 'is.currents.content.image'
			  AND s.pds_blob_cid <> ''
			  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		),
		preview_agg AS (
			SELECT root, array_agg(blob ORDER BY rn) AS blobs FROM preview WHERE rn <= 4 GROUP BY root
		)
		SELECT
			c.uri,
			c.cid,
			c.author_did,
			c.name,
			COALESCE(c.description, ''),
			COALESCE(c.parent_uri, ''),
			c.created_at,
			ss.last_saved AS last_saved_at,
			COALESCE(ss.cnt, 0) AS save_count,
			COALESCE(pa.blobs, ARRAY[]::text[]) AS preview_blobs,
			(SELECT COUNT(*) FROM favourite_collection WHERE collection_uri = c.uri)::int AS favourite_count,
			vfc.uri AS favourite_uri,
			c.favourited_at
		FROM cols c
		LEFT JOIN save_stats ss ON ss.root = c.uri
		LEFT JOIN preview_agg pa ON pa.root = c.uri
		LEFT JOIN favourite_collection vfc ON vfc.collection_uri = c.uri AND vfc.viewer_did = NULLIF($2, '')
		ORDER BY c.favourited_at DESC NULLS LAST, c.uri ASC
	`, cursorClause, limitParam)

	rows, err := m.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []CollectionRow
	var lastFavAt time.Time
	for rows.Next() {
		var row CollectionRow
		var favAt time.Time
		if err := rows.Scan(&row.URI, &row.CID, &row.AuthorDID, &row.Name, &row.Description, &row.ParentURI, &row.CreatedAt, &row.LastSavedAt, &row.SaveCount, &row.PreviewBlobs, &row.FavouriteCount, &row.FavouriteURI, &favAt); err != nil {
			return nil, "", err
		}
		result = append(result, row)
		lastFavAt = favAt
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(result) == limit {
		last := result[len(result)-1]
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(lastFavAt.UTC().Format(time.RFC3339Nano) + "|" + last.URI))
	}
	return result, nextCursor, nil
}

// SearchCollections returns collections whose name matches q (case-insensitive
// substring), ranked by save count. Offset-based pagination. Each row carries its
// AuthorDID so the caller can hydrate per-collection author profiles.
func (m *PgStore) SearchCollections(ctx context.Context, q, viewerDID string, limit, offset int) ([]CollectionRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT
			c.uri,
			c.cid,
			c.author_did,
			c.name,
			COALESCE(c.description, ''),
			COALESCE(c.parent_uri, ''),
			c.created_at,
			(SELECT MAX(created_at) FROM save WHERE collection_uri = c.uri
			   OR collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri)) AS last_saved_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri
			   OR collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE (s2.collection_uri = c.uri
				       OR s2.collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))
				  AND s2.content_nsid = 'is.currents.content.image'
				  AND s2.pds_blob_cid <> ''
				  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s2.pds_blob_cid AND b.harm_state = 'blocked')
				ORDER BY (s2.collection_uri = c.uri) DESC, s2.quality_score DESC NULLS LAST, s2.uri ASC
				LIMIT 4
			) AS preview_blobs,
			(SELECT COUNT(*) FROM favourite_collection WHERE collection_uri = c.uri)::int AS favourite_count,
			fc.uri AS favourite_uri
		FROM collection c
		LEFT JOIN favourite_collection fc
			ON fc.collection_uri = c.uri AND fc.viewer_did = NULLIF($2, '')
		WHERE c.cid IS NOT NULL
		  AND c.name ILIKE '%' || $1 || '%'
		ORDER BY save_count DESC, c.created_at DESC NULLS LAST, c.uri ASC
		LIMIT $3 OFFSET $4
	`, q, viewerDID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CollectionRow
	for rows.Next() {
		var row CollectionRow
		if err := rows.Scan(&row.URI, &row.CID, &row.AuthorDID, &row.Name, &row.Description, &row.ParentURI, &row.CreatedAt, &row.LastSavedAt, &row.SaveCount, &row.PreviewBlobs, &row.FavouriteCount, &row.FavouriteURI); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetImageCollectionsPage returns the distinct collections that contain a save
// of the same image (exact pds_blob_cid) as the given save URI, ordered by
// favourite count descending. Unsorted saves (collection_uri = ”) and blocked
// blobs are excluded. viewerDID hydrates each collection's FavouriteURI;
// AuthorDID is populated so the caller can flag the viewer's own collections.
// Offset-based pagination.
func (m *PgStore) GetImageCollectionsPage(ctx context.Context, saveURI, viewerDID string, limit, offset int) ([]CollectionRow, error) {
	rows, err := m.pool.Query(ctx, `
		WITH target AS (
			SELECT pds_blob_cid FROM save WHERE uri = $1 LIMIT 1
		)
		SELECT
			c.uri,
			c.cid,
			c.author_did,
			c.name,
			COALESCE(c.description, ''),
			COALESCE(c.parent_uri, ''),
			c.created_at,
			(SELECT MAX(created_at) FROM save WHERE collection_uri = c.uri
			   OR collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri)) AS last_saved_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri
			   OR collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE (s2.collection_uri = c.uri
				       OR s2.collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))
				  AND s2.content_nsid = 'is.currents.content.image'
				  AND s2.pds_blob_cid <> ''
				  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s2.pds_blob_cid AND b.harm_state = 'blocked')
				ORDER BY (s2.collection_uri = c.uri) DESC, s2.quality_score DESC NULLS LAST, s2.uri ASC
				LIMIT 4
			) AS preview_blobs,
			(SELECT COUNT(*) FROM favourite_collection WHERE collection_uri = c.uri)::int AS favourite_count,
			fc.uri AS favourite_uri
		FROM collection c
		LEFT JOIN favourite_collection fc
			ON fc.collection_uri = c.uri AND fc.viewer_did = NULLIF($2, '')
		WHERE c.cid IS NOT NULL
		  AND c.uri IN (
			SELECT DISTINCT s.collection_uri
			FROM save s, target t
			WHERE s.pds_blob_cid = t.pds_blob_cid
			  AND s.pds_blob_cid <> ''
			  AND s.collection_uri <> ''
			  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		  )
		ORDER BY favourite_count DESC, c.created_at DESC NULLS LAST, c.uri ASC
		LIMIT $3 OFFSET $4
	`, saveURI, viewerDID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CollectionRow
	for rows.Next() {
		var row CollectionRow
		if err := rows.Scan(&row.URI, &row.CID, &row.AuthorDID, &row.Name, &row.Description, &row.ParentURI, &row.CreatedAt, &row.LastSavedAt, &row.SaveCount, &row.PreviewBlobs, &row.FavouriteCount, &row.FavouriteURI); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// SearchActors returns users whose handle or display name matches q
// (case-insensitive substring). Offset-based pagination.
func (m *PgStore) SearchActors(ctx context.Context, q string, limit, offset int) ([]ActorRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT did, COALESCE(handle, ''), COALESCE(display_name, ''), COALESCE(description, ''),
		       COALESCE(pronouns, ''), COALESCE(website, ''), COALESCE(avatar, ''), COALESCE(banner, ''), created_at
		FROM "user"
		WHERE handle ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%'
		ORDER BY (handle ILIKE $1 || '%') DESC, handle ASC
		LIMIT $2 OFFSET $3
	`, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActorRow
	for rows.Next() {
		var row ActorRow
		if err := rows.Scan(&row.DID, &row.Handle, &row.DisplayName, &row.Description, &row.Pronouns, &row.Website, &row.Avatar, &row.Banner, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (m *PgStore) DeleteCollection(ctx context.Context, uri string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM collection WHERE uri = $1`, uri)
	return err
}

// --- Seen features (one-time "new feature" indicators, per user) ---

func (m *PgStore) GetSeenFeatures(ctx context.Context, viewerDID string) ([]string, error) {
	rows, err := m.pool.Query(ctx, `SELECT feature_key FROM seen_feature WHERE viewer_did = $1`, viewerDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (m *PgStore) MarkFeatureSeen(ctx context.Context, viewerDID, featureKey string) error {
	_, err := m.pool.Exec(ctx,
		`INSERT INTO seen_feature (viewer_did, feature_key) VALUES ($1, $2)
		 ON CONFLICT (viewer_did, feature_key) DO NOTHING`,
		viewerDID, featureKey)
	return err
}

// --- Moderation preferences (per-user, server-backed) ---

// GetModerationPrefs returns the user's stored preferences, or the defaults when
// no row exists yet.
func (m *PgStore) GetModerationPrefs(ctx context.Context, viewerDID string) (ModerationPrefs, error) {
	p := defaultModerationPrefs
	err := m.pool.QueryRow(ctx,
		`SELECT porn, sexual, nudity, graphic_media, ai_generated
		 FROM moderation_pref WHERE viewer_did = $1`,
		viewerDID).Scan(&p.Porn, &p.Sexual, &p.Nudity, &p.GraphicMedia, &p.AIGenerated)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultModerationPrefs, nil
	}
	if err != nil {
		return ModerationPrefs{}, err
	}
	return p, nil
}

func (m *PgStore) SetModerationPrefs(ctx context.Context, viewerDID string, p ModerationPrefs) error {
	_, err := m.pool.Exec(ctx,
		`INSERT INTO moderation_pref (viewer_did, porn, sexual, nudity, graphic_media, ai_generated, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, now())
		 ON CONFLICT (viewer_did) DO UPDATE SET
		     porn = EXCLUDED.porn, sexual = EXCLUDED.sexual, nudity = EXCLUDED.nudity,
		     graphic_media = EXCLUDED.graphic_media, ai_generated = EXCLUDED.ai_generated,
		     updated_at = now()`,
		viewerDID, p.Porn, p.Sexual, p.Nudity, p.GraphicMedia, p.AIGenerated)
	return err
}

// GetSubcollectionURIs returns the URIs of authorDID's collections whose parent
// is parentURI.
func (m *PgStore) GetSubcollectionURIs(ctx context.Context, parentURI, authorDID string) ([]string, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT uri FROM collection WHERE parent_uri = $1 AND author_did = $2`,
		parentURI, authorDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var uris []string
	for rows.Next() {
		var uri string
		if err := rows.Scan(&uri); err != nil {
			return nil, err
		}
		uris = append(uris, uri)
	}
	return uris, rows.Err()
}

func (m *PgStore) GetSaveRkeysInCollection(ctx context.Context, collectionURI, authorDID string) ([]string, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT uri FROM save WHERE collection_uri = $1 AND author_did = $2`,
		collectionURI, authorDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rkeys []string
	for rows.Next() {
		var uri string
		if err := rows.Scan(&uri); err != nil {
			return nil, err
		}
		if rk := rkeyFromURI(uri); rk != "" {
			rkeys = append(rkeys, rk)
		}
	}
	return rkeys, rows.Err()
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
			COALESCE(c.parent_uri, ''),
			c.created_at,
			(SELECT COUNT(*) FROM save WHERE collection_uri = c.uri
			   OR collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))::int AS save_count,
			ARRAY(
				SELECT s2.author_did || ',' || s2.pds_blob_cid
				FROM save s2
				WHERE (s2.collection_uri = c.uri
				       OR s2.collection_uri IN (SELECT uri FROM collection WHERE parent_uri = c.uri))
				  AND s2.content_nsid = 'is.currents.content.image'
				  AND s2.pds_blob_cid <> ''
				  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s2.pds_blob_cid AND b.harm_state = 'blocked')
				ORDER BY (s2.collection_uri = c.uri) DESC, s2.quality_score DESC NULLS LAST, s2.uri ASC
				LIMIT 4
			) AS preview_blobs,
			(SELECT COUNT(*) FROM favourite_collection WHERE collection_uri = c.uri)::int AS favourite_count,
			fc.uri AS favourite_uri
		FROM collection c
		LEFT JOIN favourite_collection fc
			ON fc.collection_uri = c.uri AND fc.viewer_did = NULLIF($2, '')
		WHERE c.uri = $1
		  AND c.cid IS NOT NULL
	`
	var row CollectionRow
	err := m.pool.QueryRow(ctx, query, collectionURI, viewerDID).
		Scan(&row.URI, &row.CID, &row.Name, &row.Description, &row.ParentURI, &row.CreatedAt, &row.SaveCount, &row.PreviewBlobs, &row.FavouriteCount, &row.FavouriteURI)
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
	ContentNSID        string
	Text               string
	OriginURL          string
	AttributionURL     string
	AttributionLicense string
	AttributionCredit  string
	ResaveOfURI        string
	ResaveOfCID        string // record CID of the referenced save; empty until backfilled
	CreatedAt          *time.Time
	ViewerSaves        json.RawMessage // null when unauthenticated; [{collectionUri,saveUri},...] when authenticated
	ViewerAttribution  json.RawMessage // null when unauthenticated or viewer has no saves of this blob
	Width              *int
	Height             *int
	DominantColors     json.RawMessage // nil when visual identity not yet resolved
	AltText            string
}

func (m *PgStore) GetSavesByURIs(ctx context.Context, saveURIs []string, viewerDID string) ([]SaveRow, error) {
	query := `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $2 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $2 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $2 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $2 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM save s
		WHERE s.uri = ANY($1)
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
	`
	rows, err := m.pool.Query(ctx, query, saveURIs, viewerDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAltByBlobCID returns the most recent non-empty alt text saved for the exact
// blob CID, or "" if none exists. Powers alt-text suggestions on re-upload.
func (m *PgStore) GetAltByBlobCID(ctx context.Context, blobCID string) (string, error) {
	var alt string
	err := m.pool.QueryRow(ctx, `
		SELECT alt_text FROM save
		WHERE pds_blob_cid = $1 AND COALESCE(alt_text, '') <> ''
		ORDER BY created_at DESC NULLS LAST
		LIMIT 1
	`, blobCID).Scan(&alt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return alt, nil
}

func (m *PgStore) GetSaveRkeysByAuthorAndBlob(ctx context.Context, authorDID, blobCID string) ([]string, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT uri FROM save WHERE author_did = $1 AND pds_blob_cid = $2`,
		authorDID, blobCID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rkeys []string
	for rows.Next() {
		var uri string
		if err := rows.Scan(&uri); err != nil {
			return nil, err
		}
		if i := strings.LastIndex(uri, "/"); i >= 0 && i < len(uri)-1 {
			rkeys = append(rkeys, uri[i+1:])
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rkeys, nil
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
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM save s
		WHERE s.collection_uri = $1
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
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
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText); err != nil {
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

// GetUnsortedSavesPage returns an actor's saves that belong to no collection
// (collection_uri = ”), newest first. Mirrors GetSavesPage but scopes by author
// instead of collection. Blocked blobs are excluded.
func (m *PgStore) GetUnsortedSavesPage(ctx context.Context, authorDID, viewerDID string, limit int, cursor string) ([]SaveRow, string, error) {
	args := []any{authorDID, limit, viewerDID}

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
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM save s
		WHERE s.author_did = $1
		  AND s.collection_uri = ''
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
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
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText); err != nil {
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

// GetLibrarySavesPage lists all of the author's saved images across every collection
// (and unsorted), deduplicated by blob CID so an image saved in multiple collections
// appears once (its most recent save). Ordered by created_at desc; keyset pagination.
func (m *PgStore) GetLibrarySavesPage(ctx context.Context, authorDID, viewerDID string, limit int, cursor string) ([]SaveRow, string, error) {
	args := []any{authorDID, limit, viewerDID}

	cursorClause := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(raw), "|", 2)
			if len(parts) == 2 {
				ts, err := time.Parse(time.RFC3339Nano, parts[0])
				if err == nil {
					args = append(args, ts.UTC(), parts[1])
					cursorClause = fmt.Sprintf(" AND (d.created_at < $%d OR (d.created_at = $%d AND d.uri > $%d))", len(args)-1, len(args)-1, len(args))
				}
			}
		}
	}

	// Dedup + paginate on lean columns first (`page`), then compute the heavy per-row
	// viewer JSON only for that page — otherwise the JSON subqueries run for every
	// distinct image in the library on each request.
	query := fmt.Sprintf(`
		WITH page AS (
			SELECT d.uri, d.created_at
			FROM (
				SELECT DISTINCT ON (s.pds_blob_cid) s.uri, s.created_at
				FROM save s
				WHERE s.author_did = $1
				  AND s.content_nsid = 'is.currents.content.image'
				  AND s.pds_blob_cid <> ''
				  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
				ORDER BY s.pds_blob_cid, s.created_at DESC NULLS LAST, s.uri
			) d
			WHERE TRUE %s
			ORDER BY d.created_at DESC NULLS LAST, d.uri ASC
			LIMIT $2
		)
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM save s
		JOIN page p ON p.uri = s.uri
		ORDER BY s.created_at DESC NULLS LAST, s.uri ASC
	`, cursorClause)

	rows, err := m.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText); err != nil {
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
	ContentNSID        string
	Text               string
	OriginURL          string
	AttributionURL     string
	AttributionLicense string
	AttributionCredit  string
	ResaveOfURI        string
	ResaveOfCID        string
	CreatedAt          *time.Time
	VisualIdentityID   *string
	QualityScore       *float32
	Width              *int
	Height             *int
	DominantColors     json.RawMessage
	AltText            string
}

func (m *PgStore) UpsertSave(ctx context.Context, p UpsertSaveParams) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO save (uri, author_did, collection_uri, pds_blob_cid, content_nsid, text, origin_url, attribution_url, attribution_license, attribution_credit, resave_of_uri, resave_of_cid, created_at, visual_identity_id, quality_score, width, height, dominant_colors, alt_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (uri) DO UPDATE
			SET collection_uri      = EXCLUDED.collection_uri,
			    pds_blob_cid        = EXCLUDED.pds_blob_cid,
			    content_nsid       = EXCLUDED.content_nsid,
			    text                = EXCLUDED.text,
			    origin_url          = EXCLUDED.origin_url,
			    attribution_url     = EXCLUDED.attribution_url,
			    attribution_license = EXCLUDED.attribution_license,
			    attribution_credit  = EXCLUDED.attribution_credit,
			    resave_of_uri       = EXCLUDED.resave_of_uri,
			    resave_of_cid       = EXCLUDED.resave_of_cid,
			    visual_identity_id  = EXCLUDED.visual_identity_id,
			    quality_score       = EXCLUDED.quality_score,
			    width               = EXCLUDED.width,
			    height              = EXCLUDED.height,
			    dominant_colors     = EXCLUDED.dominant_colors,
			    alt_text            = EXCLUDED.alt_text
	`, p.URI, p.AuthorDID, p.CollectionURI, p.PdsBlobCID, p.ContentNSID, p.Text, p.OriginURL, p.AttributionURL, p.AttributionLicense, p.AttributionCredit, p.ResaveOfURI, p.ResaveOfCID, p.CreatedAt, p.VisualIdentityID, p.QualityScore, p.Width, p.Height, []byte(p.DominantColors), p.AltText)
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
				ORDER BY (COALESCE(alt_text, '') <> '') DESC, quality_score DESC NULLS LAST
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
// umapEmbedding may be nil when the inference server has no UMAP model loaded.
func (m *PgStore) CreateVI(ctx context.Context, blobDID, blobCID string, embedding []float32, umapEmbedding []float32) (string, error) {
	var id string
	var umapVec interface{}
	if len(umapEmbedding) > 0 {
		umapVec = pgvector.NewVector(umapEmbedding)
	}
	err := m.pool.QueryRow(ctx, `
		INSERT INTO visual_identity (canonical_blob_did, canonical_blob_cid, embedding, umap_embedding)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, blobDID, blobCID, pgvector.NewVector(embedding), umapVec).Scan(&id)
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

// MaybePromoteCanonicalForAlt promotes the given save to canonical for its VI when the
// current canonical has no alt text. Callers invoke this only for saves that carry alt
// text, so a no-alt canonical (which would render inaccessible in search/feed/related)
// is replaced by an accessible one. Guarding on an empty current alt keeps it monotonic:
// a canonical that already has alt is never displaced, avoiding churn between savers.
func (m *PgStore) MaybePromoteCanonicalForAlt(ctx context.Context, viID, blobDID, blobCID, saveURI string) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE visual_identity vi
		SET canonical_blob_did = $2, canonical_blob_cid = $3, canonical_save_uri = $4
		WHERE vi.id = $1
		  AND COALESCE((SELECT alt_text FROM save WHERE uri = vi.canonical_save_uri), '') = ''
	`, viID, blobDID, blobCID, saveURI)
	return err
}

func (m *PgStore) ListBlobSourceCandidates(ctx context.Context, pdsBlobCID string) ([]BlobSourceCandidate, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (author_did) uri, author_did
		FROM save
		WHERE pds_blob_cid = $1
		ORDER BY author_did, created_at ASC NULLS LAST, uri ASC
	`, pdsBlobCID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BlobSourceCandidate
	for rows.Next() {
		var candidate BlobSourceCandidate
		if err := rows.Scan(&candidate.URI, &candidate.AuthorDID); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func (m *PgStore) ListCollectionsByBlobCID(ctx context.Context, pdsBlobCID string) ([]string, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT collection_uri
		FROM save
		WHERE pds_blob_cid = $1
		ORDER BY collection_uri ASC
	`, pdsBlobCID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []string
	for rows.Next() {
		var collectionURI string
		if err := rows.Scan(&collectionURI); err != nil {
			return nil, err
		}
		collections = append(collections, collectionURI)
	}
	return collections, rows.Err()
}

func (m *PgStore) ApplyBlobVisualIdentity(ctx context.Context, pdsBlobCID, viID string, qualityScore float32, width, height int, dominantColors json.RawMessage) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE save
		SET visual_identity_id = $2,
			quality_score = $3,
			width = $4,
			height = $5,
			dominant_colors = $6
		WHERE pds_blob_cid = $1
	`, pdsBlobCID, viID, qualityScore, width, height, []byte(dominantColors))
	return err
}

func (m *PgStore) GetBackgroundMetrics(ctx context.Context) (BackgroundMetrics, error) {
	saveMetrics, err := m.getSaveBackfillMetrics(ctx)
	if err != nil {
		return BackgroundMetrics{}, err
	}
	return BackgroundMetrics{
		Saves: saveMetrics,
	}, nil
}

func (m *PgStore) ListMissingVisualIdentityBlobCIDs(ctx context.Context) ([]string, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT pds_blob_cid
		FROM save
		WHERE visual_identity_id IS NULL
		GROUP BY pds_blob_cid
		ORDER BY MIN(created_at) ASC NULLS FIRST, pds_blob_cid ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobCIDs []string
	for rows.Next() {
		var blobCID string
		if err := rows.Scan(&blobCID); err != nil {
			return nil, err
		}
		blobCIDs = append(blobCIDs, blobCID)
	}
	return blobCIDs, rows.Err()
}

func (m *PgStore) ListCollectionsMissingEmbedding(ctx context.Context) ([]string, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT c.uri
		FROM collection c
		WHERE c.canonical_embedding IS NULL
		  AND EXISTS (
			SELECT 1
			FROM save s
			JOIN visual_identity vi ON vi.id = s.visual_identity_id
			WHERE s.collection_uri = c.uri
			  AND vi.embedding IS NOT NULL
		  )
		ORDER BY c.uri ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collectionURIs []string
	for rows.Next() {
		var collectionURI string
		if err := rows.Scan(&collectionURI); err != nil {
			return nil, err
		}
		collectionURIs = append(collectionURIs, collectionURI)
	}
	return collectionURIs, rows.Err()
}

func (m *PgStore) getSaveBackfillMetrics(ctx context.Context) (SaveBackfillMetrics, error) {
	var metrics SaveBackfillMetrics
	err := m.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS missing_visual_identity_count,
			COUNT(DISTINCT pds_blob_cid) AS distinct_missing_blob_cid_count,
			MIN(created_at) AS oldest_missing_created_at,
			(
				SELECT COUNT(*)
				FROM collection c
				WHERE c.canonical_embedding IS NULL
				  AND EXISTS (
					SELECT 1
					FROM save s
					JOIN visual_identity vi ON vi.id = s.visual_identity_id
					WHERE s.collection_uri = c.uri
					  AND vi.embedding IS NOT NULL
				  )
			) AS collections_missing_embedding_count
		FROM save
		WHERE visual_identity_id IS NULL
	`).Scan(
		&metrics.MissingVisualIdentityCount,
		&metrics.DistinctMissingBlobCIDCount,
		&metrics.OldestMissingCreatedAt,
		&metrics.CollectionsMissingEmbeddingCount,
	)
	return metrics, err
}

func searchSavesQueryLimit(limit int, excludeViewerSaves bool) int {
	queryLimit := limit + 1
	if excludeViewerSaves {
		queryLimit = max(queryLimit, limit*2)
	}
	return queryLimit
}

func searchSavesMaxScanTuples(offset, fetchLimit int) int {
	return max(20000, (offset+fetchLimit)*20)
}

func trimANNPage(rows []SaveRow, limit int) annSavePage {
	if len(rows) > limit {
		return annSavePage{Rows: rows[:limit], HasMore: true}
	}
	return annSavePage{Rows: rows}
}

func scanSaveRows(rows pgx.Rows) ([]SaveRow, error) {
	defer rows.Close()

	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *PgStore) setANNQueryOptions(ctx context.Context, tx pgx.Tx, offset, fetchLimit int) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('hnsw.ef_search', $1, true)`, strconv.Itoa(searchSavesEFSearch(offset, fetchLimit))); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('hnsw.iterative_scan', 'strict_order', true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('hnsw.max_scan_tuples', $1, true)`, strconv.Itoa(searchSavesMaxScanTuples(offset, fetchLimit))); err != nil {
		return err
	}
	return nil
}

func (m *PgStore) queryANNSavePage(ctx context.Context, query string, args []any, limit, fetchLimit, offset int) (annSavePage, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return annSavePage{}, err
	}
	defer tx.Rollback(ctx)

	if err := m.setANNQueryOptions(ctx, tx, offset, fetchLimit); err != nil {
		return annSavePage{}, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return annSavePage{}, err
	}
	result, err := scanSaveRows(rows)
	if err != nil {
		return annSavePage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return annSavePage{}, err
	}
	return trimANNPage(result, limit), nil
}

// SearchSavesByEmbedding returns saves whose visual identity is nearest to the given embedding,
// ordered by cosine distance. Offset-based pagination; pass offset=0 for the first page.
func (m *PgStore) SearchSavesByEmbedding(ctx context.Context, embedding []float32, viewerDID string, excludeViewerSaves bool, limit, offset int) ([]SaveRow, error) {
	page, err := m.searchSavesByEmbeddingPage(ctx, embedding, viewerDID, excludeViewerSaves, limit, limit, offset)
	if err != nil {
		return nil, err
	}
	return page.Rows, nil
}

func (m *PgStore) SearchSavesPageByEmbedding(ctx context.Context, embedding []float32, viewerDID string, excludeViewerSaves bool, limit, offset int) (annSavePage, error) {
	return m.searchSavesByEmbeddingPage(ctx, embedding, viewerDID, excludeViewerSaves, limit, searchSavesQueryLimit(limit, excludeViewerSaves), offset)
}

func (m *PgStore) searchSavesByEmbeddingPage(ctx context.Context, embedding []float32, viewerDID string, excludeViewerSaves bool, limit, fetchLimit, offset int) (annSavePage, error) {
	vec := pgvector.NewVector(embedding)
	applyExclude := excludeViewerSaves && viewerDID != ""
	excludeClause := ""
	if applyExclude {
		excludeClause = `AND NOT EXISTS (
			SELECT 1 FROM save v
			WHERE v.author_did = $3 AND v.pds_blob_cid = s.pds_blob_cid
		)`
	}
	query := `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM visual_identity vi
		JOIN save s ON s.uri = vi.canonical_save_uri
		WHERE vi.embedding IS NOT NULL
		  AND s.author_did <> ALL($5)
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		  ` + excludeClause + `
		ORDER BY vi.embedding <=> $1
		LIMIT $2 OFFSET $4
	`
	return m.queryANNSavePage(ctx, query, []any{vec, fetchLimit, viewerDID, offset, m.cfg.HiddenDIDs}, limit, fetchLimit, offset)
}

// SearchLibrarySavesPageByEmbedding searches the viewer's own saved images (or, when
// collectionURIs is non-empty, the saves within those specific collections) ranked by
// cosine distance to the query embedding. Offset-based pagination.
func (m *PgStore) SearchLibrarySavesPageByEmbedding(ctx context.Context, embedding []float32, viewerDID string, collectionURIs []string, limit, offset int) (annSavePage, error) {
	vec := pgvector.NewVector(embedding)
	fetchLimit := limit + 1
	// Scope: specific collections (own or favourited) if given, else the whole library.
	scopeClause := "AND s.author_did = $3"
	args := []any{vec, fetchLimit, viewerDID, offset, m.cfg.HiddenDIDs}
	if len(collectionURIs) > 0 {
		scopeClause = "AND s.collection_uri = ANY($6)"
		args = append(args, collectionURIs)
	}
	query := `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM save s
		JOIN visual_identity vi ON vi.id = s.visual_identity_id
		WHERE vi.embedding IS NOT NULL
		  AND s.content_nsid = 'is.currents.content.image'
		  AND s.author_did <> ALL($5)
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		  ` + scopeClause + `
		ORDER BY vi.embedding <=> $1
		LIMIT $2 OFFSET $4
	`
	return m.queryANNSavePage(ctx, query, args, limit, fetchLimit, offset)
}

func searchSavesEFSearch(offset, fetchLimit int) int {
	depth := offset + fetchLimit
	if depth < 100 {
		return 100
	}
	return depth
}

// GetRelatedSavesByURI returns saves whose visual-identity embedding is closest
// to the given save's embedding (cosine distance). Excludes the source save's
// own visual identity so resaves of the same image don't appear as related.
// Returns an empty slice if the source save is unknown or has no embedding.
func (m *PgStore) GetRelatedSavesByURI(ctx context.Context, uri string, viewerDID string, limit, offset int) ([]SaveRow, error) {
	page, err := m.getRelatedSavesPageByURI(ctx, uri, viewerDID, limit, limit, offset)
	if err != nil {
		return nil, err
	}
	return page.Rows, nil
}

func (m *PgStore) GetRelatedSavesPageByURI(ctx context.Context, uri string, viewerDID string, limit, offset int) (annSavePage, error) {
	return m.getRelatedSavesPageByURI(ctx, uri, viewerDID, limit, limit+1, offset)
}

func (m *PgStore) getRelatedSavesPageByURI(ctx context.Context, uri string, viewerDID string, limit, fetchLimit, offset int) (annSavePage, error) {
	query := `
		WITH src AS (
			SELECT vi.id AS vi_id, vi.embedding
			FROM save s
			JOIN visual_identity vi ON vi.id = s.visual_identity_id
			WHERE s.uri = $1 AND vi.embedding IS NOT NULL
		)
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM visual_identity vi
		JOIN save s ON s.uri = vi.canonical_save_uri
		WHERE vi.embedding IS NOT NULL
			AND s.author_did <> ALL($5)
			AND vi.id != (SELECT vi_id FROM src)
			AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		ORDER BY vi.embedding <=> (SELECT embedding FROM src)
		LIMIT $2 OFFSET $4
	`
	return m.queryANNSavePage(ctx, query, []any{uri, fetchLimit, viewerDID, offset, m.cfg.HiddenDIDs}, limit, fetchLimit, offset)
}

// FindSimilarLibrarySavesPageByURI returns the viewer's own saves (or, when collectionURIs
// is non-empty, saves within those collections) visually closest to the source save's image,
// excluding the source's own visual identity. Offset-based pagination.
func (m *PgStore) FindSimilarLibrarySavesPageByURI(ctx context.Context, uri, viewerDID string, collectionURIs []string, limit, offset int) (annSavePage, error) {
	fetchLimit := limit + 1
	scopeClause := "AND s.author_did = $3"
	args := []any{uri, fetchLimit, viewerDID, offset, m.cfg.HiddenDIDs}
	if len(collectionURIs) > 0 {
		scopeClause = "AND s.collection_uri = ANY($6)"
		args = append(args, collectionURIs)
	}
	query := `
		WITH src AS (
			SELECT vi.id AS vi_id, vi.embedding
			FROM save s
			JOIN visual_identity vi ON vi.id = s.visual_identity_id
			WHERE s.uri = $1 AND vi.embedding IS NOT NULL
		)
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $3 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $3 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM save s
		JOIN visual_identity vi ON vi.id = s.visual_identity_id
		WHERE vi.embedding IS NOT NULL
		  AND s.content_nsid = 'is.currents.content.image'
		  AND vi.id != (SELECT vi_id FROM src)
		  AND s.author_did <> ALL($5)
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		  ` + scopeClause + `
		ORDER BY vi.embedding <=> (SELECT embedding FROM src)
		LIMIT $2 OFFSET $4
	`
	return m.queryANNSavePage(ctx, query, args, limit, fetchLimit, offset)
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

type VisualIdentityEmbedding struct {
	ID        string
	Embedding []float32
}

type ClusterMedoid struct {
	ClusterID        string
	VisualIdentityID string
	Embedding        []float32
}

// GetCollectionsByImportance returns the viewer's top-N collections ranked by time-decayed save count,
// filtered to those that have a precomputed canonical embedding.
func (m *PgStore) GetCollectionsByImportance(ctx context.Context, viewerDID string, topN int) ([]CollectionImportance, error) {
	rows, err := m.pool.Query(ctx, `
		WITH ranked AS (
			SELECT
				c.uri,
				c.canonical_embedding,
				SUM(EXP(-0.01 * EXTRACT(EPOCH FROM (NOW() - s.created_at)) / 86400)) AS score
			FROM save s
			JOIN collection c ON c.uri = s.collection_uri
			WHERE s.author_did = $1
			  AND s.created_at IS NOT NULL
			  AND c.canonical_embedding IS NOT NULL
			GROUP BY c.uri, c.canonical_embedding
		)
		SELECT uri, canonical_embedding
		FROM ranked
		ORDER BY score DESC, uri ASC
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

func (m *PgStore) GetCollectionsByURIs(ctx context.Context, uris []string) ([]CollectionImportance, error) {
	if len(uris) == 0 {
		return nil, nil
	}

	rows, err := m.pool.Query(ctx, `
		SELECT uri, canonical_embedding
		FROM collection
		WHERE uri = ANY($1)
		  AND canonical_embedding IS NOT NULL
	`, uris)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byURI := make(map[string][]float32, len(uris))
	for rows.Next() {
		var uri string
		var vec pgvector.Vector
		if err := rows.Scan(&uri, &vec); err != nil {
			return nil, err
		}
		byURI[uri] = vec.Slice()
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]CollectionImportance, 0, len(uris))
	for _, uri := range uris {
		embedding, ok := byURI[uri]
		if !ok {
			continue
		}
		result = append(result, CollectionImportance{URI: uri, Embedding: embedding})
	}
	return result, nil
}

func (m *PgStore) GetVisualIdentityEmbeddingsByIDs(ctx context.Context, ids []string) ([]VisualIdentityEmbedding, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := m.pool.Query(ctx, `
		SELECT wanted.id, vi.embedding
		FROM unnest($1::text[]) WITH ORDINALITY AS wanted(id, ord)
		JOIN visual_identity vi ON vi.id = wanted.id::uuid
		WHERE vi.embedding IS NOT NULL
		ORDER BY wanted.ord
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VisualIdentityEmbedding
	for rows.Next() {
		var id string
		var vec pgvector.Vector
		if err := rows.Scan(&id, &vec); err != nil {
			return nil, err
		}
		result = append(result, VisualIdentityEmbedding{ID: id, Embedding: vec.Slice()})
	}
	return result, rows.Err()
}

func (m *PgStore) GetViewerClusterIDs(ctx context.Context, viewerDID string) ([]string, error) {
	if viewerDID == "" {
		return nil, nil
	}

	rows, err := m.pool.Query(ctx, `
		WITH latest_run AS (
			SELECT MAX(run_date) AS run_date FROM cluster
		)
		SELECT DISTINCT c.id::text
		FROM latest_run lr
		JOIN cluster c ON c.run_date = lr.run_date
		JOIN visual_identity vi ON vi.cluster_id = c.id
		JOIN save s ON s.visual_identity_id = vi.id
		WHERE s.author_did = $1
		ORDER BY c.id::text
	`, viewerDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var clusterID string
		if err := rows.Scan(&clusterID); err != nil {
			return nil, err
		}
		result = append(result, clusterID)
	}
	return result, rows.Err()
}

func (m *PgStore) GetNearestClusterMedoid(ctx context.Context, embedding []float32, excludedClusterIDs []string) (*ClusterMedoid, error) {
	vec := pgvector.NewVector(embedding)

	var medoid ClusterMedoid
	var medoidVec pgvector.Vector
	err := m.pool.QueryRow(ctx, `
		WITH latest_run AS (
			SELECT MAX(run_date) AS run_date FROM cluster
		)
		SELECT c.id::text, vi.id::text, vi.embedding
		FROM latest_run lr
		JOIN cluster c ON c.run_date = lr.run_date
		JOIN visual_identity vi ON vi.id = c.medoid_visual_identity_id
		WHERE vi.embedding IS NOT NULL
		  AND (COALESCE(array_length($2::text[], 1), 0) = 0 OR NOT (c.id::text = ANY($2::text[])))
		ORDER BY vi.embedding <=> $1, c.id
		LIMIT 1
	`, vec, excludedClusterIDs).Scan(&medoid.ClusterID, &medoid.VisualIdentityID, &medoidVec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	medoid.Embedding = medoidVec.Slice()
	return &medoid, nil
}

// GetGlobalFeedSaves returns saves from across the network, ranked by a
// time-decayed popularity score: save_count * exp(-0.01 * age_in_days).
// No minimum save_count threshold — all images with a visual identity appear,
// with popular recent images ranked highest.
func (m *PgStore) GetGlobalFeedSaves(ctx context.Context, viewerDID string, excludeViewerSaves bool, limit, offset int) ([]SaveRow, error) {
	excludeClause := ""
	if excludeViewerSaves && viewerDID != "" {
		excludeClause = `AND NOT EXISTS (
			SELECT 1 FROM save v
			WHERE v.author_did = $1 AND v.pds_blob_cid = s.pds_blob_cid
		)`
	}
	query := `
		SELECT
			s.uri,
			s.pds_blob_cid,
			s.author_did,
			s.content_nsid,
			COALESCE(s.text, ''),
			COALESCE(s.origin_url, ''),
			COALESCE(s.attribution_url, ''),
			COALESCE(s.attribution_license, ''),
			COALESCE(s.attribution_credit, ''),
			COALESCE(s.resave_of_uri, ''),
			COALESCE(s.resave_of_cid, ''),
			s.created_at,
			CASE WHEN $1 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_agg(json_build_object('collectionUri', rv.collection_uri, 'saveUri', rv.uri))
				FROM save rv WHERE rv.author_did = $1 AND rv.pds_blob_cid = s.pds_blob_cid
			) END AS viewer_saves,
			CASE WHEN $1 != '' AND s.content_nsid = 'is.currents.content.image' AND s.pds_blob_cid <> '' THEN (
				SELECT json_build_object(
					'url', COALESCE(rv.attribution_url, ''),
					'license', COALESCE(rv.attribution_license, ''),
					'credit', COALESCE(rv.attribution_credit, '')
				)
				FROM save rv
				WHERE rv.author_did = $1 AND rv.pds_blob_cid = s.pds_blob_cid
				  AND (COALESCE(rv.attribution_url, '') <> ''
				       OR COALESCE(rv.attribution_license, '') <> ''
				       OR COALESCE(rv.attribution_credit, '') <> '')
				ORDER BY rv.created_at DESC NULLS LAST
				LIMIT 1
			) END AS viewer_attribution,
			s.width,
			s.height,
			s.dominant_colors,
			COALESCE(s.alt_text, '')
		FROM visual_identity vi
		JOIN save s ON s.uri = vi.canonical_save_uri
		WHERE s.author_did <> ALL($4)
		  AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')
		  ` + excludeClause + `
		ORDER BY (vi.save_count * EXP(-0.01 * EXTRACT(EPOCH FROM (NOW() - s.created_at)) / 86400)) DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := m.pool.Query(ctx, query, viewerDID, limit, offset, m.cfg.HiddenDIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SaveRow
	for rows.Next() {
		var row SaveRow
		if err := rows.Scan(&row.URI, &row.BlobCID, &row.AuthorDID, &row.ContentNSID, &row.Text, &row.OriginURL, &row.AttributionURL, &row.AttributionLicense, &row.AttributionCredit, &row.ResaveOfURI, &row.ResaveOfCID, &row.CreatedAt, &row.ViewerSaves, &row.ViewerAttribution, &row.Width, &row.Height, &row.DominantColors, &row.AltText); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// --- Pinterest bulk import ---

type ImportJobRow struct {
	ID                  string
	SessionID           string
	OwnerDID            string
	OAuthSessionID      string
	Source              string
	SourceBoardID       string
	SourceBoardName     string
	SourceBoardURL      string
	SourceSectionID     string // non-empty when this job imports a board section
	FilterSectionPins   bool   // board job: skip pins that belong to a section
	TargetCollectionURI string
	Status              string
	ListCursor          string
	Error               string
}

type ImportItemRow struct {
	ID                  string
	JobID               string
	OwnerDID            string
	SourcePinID         string
	ImageURL            string
	SourceURL           string
	Rkey                string
	Status              string
	SaveURI             string
	Error               string
	AttemptCount        int
	TargetCollectionURI string
}

type SessionJobStatus struct {
	JobID     string
	BoardName string
	Status    string
	Queued    int
	Running   int
	Done      int
	Failed    int
}

func (m *PgStore) UpsertImportSession(ctx context.Context, id, ownerDID, username string) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO import_session (id, owner_did, username) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET username = EXCLUDED.username
	`, id, ownerDID, username)
	return err
}

type ActiveImportSession struct {
	SessionID string
	Username  string
	StartedAt time.Time
}

// GetLatestActiveSession returns the most recently created session for ownerDID
// that still has at least one non-terminal job. Returns nil if none.
func (m *PgStore) GetLatestActiveSession(ctx context.Context, ownerDID string) (*ActiveImportSession, error) {
	var s ActiveImportSession
	err := m.pool.QueryRow(ctx, `
		SELECT s.id, s.username, s.created_at
		FROM import_session s
		WHERE s.owner_did = $1
		  AND EXISTS (
		    SELECT 1 FROM import_job j
		    WHERE j.session_id = s.id
		      AND j.status NOT IN ('done', 'failed')
		  )
		ORDER BY s.created_at DESC
		LIMIT 1
	`, ownerDID).Scan(&s.SessionID, &s.Username, &s.StartedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &s, err
}

func (m *PgStore) CreateImportJob(ctx context.Context, p ImportJobRow) (string, error) {
	var id string
	err := m.pool.QueryRow(ctx, `
		INSERT INTO import_job
			(session_id, owner_did, oauth_session_id, source, source_board_id, source_board_name, source_board_url, source_section_id, filter_section_pins, target_collection_uri, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'listing')
		RETURNING id
	`, p.SessionID, p.OwnerDID, p.OAuthSessionID, p.Source, p.SourceBoardID, p.SourceBoardName, p.SourceBoardURL, p.SourceSectionID, p.FilterSectionPins, p.TargetCollectionURI).Scan(&id)
	return id, err
}

// BulkInsertImportItems inserts one row per pin with a freshly generated TID
// rkey. Conflicts on (job_id, source_pin_id) are silently dropped so the
// listing stage can be safely re-run after a crash.
func (m *PgStore) BulkInsertImportItems(ctx context.Context, jobID, ownerDID string, pins []PinterestPin) (int, error) {
	if len(pins) == 0 {
		return 0, nil
	}
	rows := make([][]any, 0, len(pins))
	clock := syntax.NewTIDClock(0)
	for _, p := range pins {
		rows = append(rows, []any{jobID, ownerDID, p.ID, p.ImageURL, p.SourceURL, clock.Next().String()})
	}
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE _import_item_in (
			job_id UUID, owner_did TEXT, source_pin_id TEXT, image_url TEXT, source_url TEXT, rkey TEXT
		) ON COMMIT DROP
	`); err != nil {
		return 0, err
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"_import_item_in"},
		[]string{"job_id", "owner_did", "source_pin_id", "image_url", "source_url", "rkey"},
		pgx.CopyFromRows(rows),
	); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO import_item (job_id, owner_did, source_pin_id, image_url, source_url, rkey)
		SELECT job_id, owner_did, source_pin_id, image_url, source_url, rkey FROM _import_item_in
		ON CONFLICT (job_id, source_pin_id) DO NOTHING
	`)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (m *PgStore) UpdateImportJobStatus(ctx context.Context, jobID, status, errMsg string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_job SET status=$2, error=$3, updated_at=now() WHERE id=$1`,
		jobID, status, errMsg,
	)
	return err
}

// FailImportJob marks the job and all its still-pending items as failed in one
// shot. Used for job-wide errors (e.g. the target collection was deleted) where
// retrying individual items is pointless.
func (m *PgStore) FailImportJob(ctx context.Context, jobID, reason string) error {
	if _, err := m.pool.Exec(ctx,
		`UPDATE import_item SET status='failed', error=$2, updated_at=now()
		   WHERE job_id=$1 AND status IN ('queued','running')`,
		jobID, reason,
	); err != nil {
		return err
	}
	_, err := m.pool.Exec(ctx,
		`UPDATE import_job SET status='failed', error=$2, updated_at=now() WHERE id=$1`,
		jobID, reason,
	)
	return err
}

func (m *PgStore) UpdateImportJobCursor(ctx context.Context, jobID, cursor string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_job SET list_cursor=$2, updated_at=now() WHERE id=$1`,
		jobID, cursor,
	)
	return err
}

func (m *PgStore) ListJobsByOwnerStatus(ctx context.Context, ownerDID, status string) ([]ImportJobRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT id, session_id, owner_did, oauth_session_id, source,
		       source_board_id, source_board_name, source_board_url, source_section_id, filter_section_pins, target_collection_uri,
		       status, list_cursor, error
		FROM import_job
		WHERE owner_did = $1 AND status = $2
		ORDER BY created_at
	`, ownerDID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImportJobRow
	for rows.Next() {
		var j ImportJobRow
		if err := rows.Scan(&j.ID, &j.SessionID, &j.OwnerDID, &j.OAuthSessionID, &j.Source,
			&j.SourceBoardID, &j.SourceBoardName, &j.SourceBoardURL, &j.SourceSectionID, &j.FilterSectionPins, &j.TargetCollectionURI,
			&j.Status, &j.ListCursor, &j.Error); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ClaimNextImportItem atomically transitions one queued item for ownerDID to
// 'running' and returns it joined with the job's target_collection_uri. Returns
// (nil, nil) when nothing is queued.
func (m *PgStore) ClaimNextImportItem(ctx context.Context, ownerDID string) (*ImportItemRow, error) {
	var i ImportItemRow
	err := m.pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id FROM import_item
			WHERE owner_did = $1 AND status = 'queued'
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE import_item AS i
		   SET status = 'running',
		       attempt_count = attempt_count + 1,
		       updated_at = now()
		  FROM next, import_job AS j
		 WHERE i.id = next.id
		   AND j.id = i.job_id
		RETURNING i.id, i.job_id, i.owner_did, i.source_pin_id, i.image_url, i.source_url, i.rkey,
		          i.status, i.save_uri, i.error, i.attempt_count, j.target_collection_uri
	`, ownerDID).Scan(
		&i.ID, &i.JobID, &i.OwnerDID, &i.SourcePinID, &i.ImageURL, &i.SourceURL, &i.Rkey,
		&i.Status, &i.SaveURI, &i.Error, &i.AttemptCount, &i.TargetCollectionURI,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// CountRecentImportDone returns how many import items for ownerDID completed
// in the last hour and in the last 24 hours, across all their jobs. Used to
// budget PDS write points so imports never starve the user's own activity.
func (m *PgStore) CountRecentImportDone(ctx context.Context, ownerDID string) (hourly, daily int, err error) {
	err = m.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE updated_at > now() - interval '1 hour'), count(*)
		  FROM import_item
		 WHERE owner_did = $1 AND status = 'done' AND updated_at > now() - interval '24 hours'`,
		ownerDID,
	).Scan(&hourly, &daily)
	return hourly, daily, err
}

func (m *PgStore) MarkImportItemDone(ctx context.Context, itemID, saveURI string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_item SET status='done', save_uri=$2, error='', updated_at=now() WHERE id=$1`,
		itemID, saveURI,
	)
	return err
}

func (m *PgStore) MarkImportItemFailed(ctx context.Context, itemID, errMsg string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_item SET status='failed', error=$2, updated_at=now() WHERE id=$1`,
		itemID, errMsg,
	)
	return err
}

func (m *PgStore) RequeueImportItem(ctx context.Context, itemID, lastError string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_item SET status='queued', error=$2, updated_at=now() WHERE id=$1`,
		itemID, lastError,
	)
	return err
}

// PauseImportItem returns a claimed item to the queue without consuming a retry
// attempt. Used when the user's OAuth session is temporarily unavailable, so the
// import resumes cleanly once they have a valid session again.
func (m *PgStore) PauseImportItem(ctx context.Context, itemID string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_item SET status='queued', attempt_count = GREATEST(attempt_count - 1, 0), updated_at=now() WHERE id=$1`,
		itemID,
	)
	return err
}

// MaybeFinalizeJob flips a 'running' job to 'done' once no items remain in
// queued or running state. No-op when items remain, or when the job isn't
// in the 'running' state.
func (m *PgStore) MaybeFinalizeJob(ctx context.Context, jobID string) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE import_job
		   SET status = 'done', updated_at = now()
		 WHERE id = $1
		   AND status = 'running'
		   AND NOT EXISTS (
		       SELECT 1 FROM import_item
		        WHERE job_id = $1 AND status IN ('queued','running')
		   )
	`, jobID)
	return err
}

// ResetRunningItemsForUser flips 'running' items back to 'queued' so a
// restarted worker can re-claim them. Runs once per user at worker startup.
func (m *PgStore) ResetRunningItemsForUser(ctx context.Context, ownerDID string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE import_item SET status='queued', updated_at=now()
		   WHERE owner_did=$1 AND status='running'`,
		ownerDID,
	)
	return err
}

// ListInflightUsers returns DIDs that have a queued/running item OR a job
// in the 'listing' state.
func (m *PgStore) ListInflightUsers(ctx context.Context) ([]string, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT j.owner_did
		FROM import_job j
		WHERE j.status = 'listing'
		   OR EXISTS (
		       SELECT 1 FROM import_item i
		        WHERE i.job_id = j.id AND i.status IN ('queued','running')
		   )
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, err
		}
		out = append(out, did)
	}
	return out, rows.Err()
}

func (m *PgStore) GetSessionStatus(ctx context.Context, sessionID, ownerDID string) ([]SessionJobStatus, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT j.id, j.source_board_name, j.status,
		       SUM(CASE WHEN i.status='queued'  THEN 1 ELSE 0 END)::int AS queued,
		       SUM(CASE WHEN i.status='running' THEN 1 ELSE 0 END)::int AS running,
		       SUM(CASE WHEN i.status='done'    THEN 1 ELSE 0 END)::int AS done,
		       SUM(CASE WHEN i.status='failed'  THEN 1 ELSE 0 END)::int AS failed
		FROM import_job j
		LEFT JOIN import_item i ON i.job_id = j.id
		WHERE j.session_id = $1 AND j.owner_did = $2
		GROUP BY j.id
		ORDER BY j.created_at
	`, sessionID, ownerDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionJobStatus
	for rows.Next() {
		var s SessionJobStatus
		if err := rows.Scan(&s.JobID, &s.BoardName, &s.Status, &s.Queued, &s.Running, &s.Done, &s.Failed); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
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

// UpdateUserPDSEndpoint updates the cached PDS endpoint for a known user.
func (m *PgStore) UpdateUserPDSEndpoint(ctx context.Context, did, endpoint string) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE "user" SET pds_endpoint = $2 WHERE did = $1`,
		did, endpoint,
	)
	return err
}

// ── Moderation methods ───────────────────────────────────────────────────────

// SafetyScores are the three classification-head outputs persisted on a blob.
type SafetyScores struct {
	NSFW        float32 `json:"nsfw"`
	Violence    float32 `json:"violence"`
	AIGenerated float32 `json:"ai_generated"`
}

const (
	HarmStateClean     = "clean"
	HarmStateSuspected = "suspected"
	HarmStateBlocked   = "blocked"
)

// BlobModerationStateRow mirrors a row in blob_moderation_state.
type BlobModerationStateRow struct {
	BlobCID      string
	HarmState    string
	AIGenerated  bool
	DecidedBy    string
	DecidedAt    *time.Time
	SafetyScores *SafetyScores
	Notes        string
	UpdatedAt    time.Time
}

// LabelRow mirrors a row in label.
type LabelRow struct {
	ID        int64
	Src       string
	URI       string
	CID       string
	Val       string
	Neg       bool
	CTS       time.Time
	Exp       *time.Time
	Sig       []byte
	Ver       int
	BlobCID   string
	CreatedAt time.Time
}

// ReviewItemRow mirrors a row in review_item.
type ReviewItemRow struct {
	ID         int64
	Source     string
	SourceRef  *int64
	SubjectURI string
	SubjectCID string
	BlobCID    string
	Category   string
	// LabelVal is the specific atproto label val applied for label_applied items
	// (e.g. "porn", "nudity", "sexual", "graphic-media", "currents-ai-generated").
	// Empty for source='ai' items where no label has been applied yet.
	LabelVal  string
	Score     *float32
	Status    string
	Priority  int
	CreatedAt time.Time

	// Populated by list/detail queries via a LEFT JOIN on report when source='report'.
	// Empty strings on non-report items. Surfaces the original report context
	// (raw reasonType, free text, reporter DID) so moderators see what was
	// actually reported, not just the coarse `category` bucket.
	ReportReasonType  string
	ReportReasonText  string
	ReportReporterDID string

	// Author self-attestation state. Disputed = true means the save's author
	// pushed back on the auto-flag; the suspected label still stands but a
	// moderator should weigh the dispute when deciding.
	Disputed   bool
	DisputedAt *time.Time
}

// UpsertBlobModerationState records the latest safety scores for a blob.
// Preserves any existing human decision (harm_state, ai_generated, notes) — only
// safety_scores and updated_at are refreshed on conflict.
func (m *PgStore) UpsertBlobModerationState(ctx context.Context, blobCID string, scores SafetyScores) error {
	scoresJSON, err := json.Marshal(scores)
	if err != nil {
		return err
	}
	_, err = m.pool.Exec(ctx, `
		INSERT INTO blob_moderation_state (blob_cid, safety_scores, decided_by, decided_at)
		VALUES ($1, $2, 'auto', now())
		ON CONFLICT (blob_cid) DO UPDATE
			SET safety_scores = EXCLUDED.safety_scores, updated_at = now()
	`, blobCID, scoresJSON)
	return err
}

// GetBlobModerationState returns the moderation state for a blob, if any.
func (m *PgStore) GetBlobModerationState(ctx context.Context, blobCID string) (*BlobModerationStateRow, error) {
	var row BlobModerationStateRow
	var decidedBy, notes *string
	var scoresJSON []byte
	err := m.pool.QueryRow(ctx, `
		SELECT blob_cid, harm_state, ai_generated, decided_by, decided_at, safety_scores, notes, updated_at
		FROM blob_moderation_state WHERE blob_cid = $1
	`, blobCID).Scan(&row.BlobCID, &row.HarmState, &row.AIGenerated, &decidedBy, &row.DecidedAt, &scoresJSON, &notes, &row.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if decidedBy != nil {
		row.DecidedBy = *decidedBy
	}
	if notes != nil {
		row.Notes = *notes
	}
	if len(scoresJSON) > 0 {
		var s SafetyScores
		if err := json.Unmarshal(scoresJSON, &s); err == nil {
			row.SafetyScores = &s
		}
	}
	return &row, nil
}

// UpsertReviewItem enqueues a moderation review item. Idempotent per (blob_cid, category)
// when blob_cid is set, or per (subject_uri, category) otherwise. Re-enqueueing the
// same blob+axis while a pending entry exists is a no-op.
// Returns true if a new row was created.
func (m *PgStore) UpsertReviewItem(ctx context.Context, item ReviewItemRow) (bool, error) {
	cmd, err := m.pool.Exec(ctx, `
		INSERT INTO review_item (source, source_ref, subject_uri, subject_cid, blob_cid, category, label_val, score, priority)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9
		WHERE NOT EXISTS (
			SELECT 1 FROM review_item
			WHERE (
				($5 <> '' AND blob_cid = $5) OR
				($5 = ''  AND subject_uri = $3)
			)
			AND COALESCE(category, '') = COALESCE($6, '')
			AND status = 'pending'
		)
	`, item.Source, item.SourceRef, item.SubjectURI, nullableString(item.SubjectCID), nullableString(item.BlobCID), nullableString(item.Category), nullableString(item.LabelVal), item.Score, item.Priority)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

// InsertLabel writes a signed label row and returns the new id.
func (m *PgStore) InsertLabel(ctx context.Context, l LabelRow) (int64, error) {
	var id int64
	err := m.pool.QueryRow(ctx, `
		INSERT INTO label (src, uri, cid, val, neg, cts, exp, sig, ver, blob_cid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, l.Src, l.URI, nullableString(l.CID), l.Val, l.Neg, l.CTS, l.Exp, l.Sig, l.Ver, nullableString(l.BlobCID)).Scan(&id)
	return id, err
}

// GetLabelsByURIs returns the currently active labels for each requested save URI.
// Active = not negated by a later row with the same (src, uri, val). Returns a map keyed by URI.
func (m *PgStore) GetLabelsByURIs(ctx context.Context, uris []string) (map[string][]LabelRow, error) {
	out := make(map[string][]LabelRow, len(uris))
	if len(uris) == 0 {
		return out, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (src, uri, val)
			id, src, uri, COALESCE(cid, ''), val, neg, cts, exp, sig, ver, COALESCE(blob_cid, ''), created_at
		FROM label
		WHERE uri = ANY($1)
		ORDER BY src, uri, val, id DESC
	`, uris)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r LabelRow
		if err := rows.Scan(&r.ID, &r.Src, &r.URI, &r.CID, &r.Val, &r.Neg, &r.CTS, &r.Exp, &r.Sig, &r.Ver, &r.BlobCID, &r.CreatedAt); err != nil {
			return nil, err
		}
		if r.Neg {
			continue // the latest row negates this (src, uri, val) — skip
		}
		out[r.URI] = append(out[r.URI], r)
	}
	return out, rows.Err()
}

// ModerationEventRow mirrors a row in moderation_event.
type ModerationEventRow struct {
	ID         int64
	ActorDID   string
	Action     string
	SubjectURI string
	SubjectCID string
	BlobCID    string
	Payload    json.RawMessage
	CreatedAt  time.Time
}

// QueryLabels returns labels matching uri patterns + optional source DID filter,
// paginated by id. Cursor of 0 means start from the beginning. Returns the rows
// and the next cursor (0 when no more pages).
//
// uriPatterns syntax: per the atproto spec, '*' is the only wildcard and may
// only appear at the end of a pattern. We translate to SQL LIKE by mapping
// '*' → '%'.
func (m *PgStore) QueryLabels(ctx context.Context, uriPatterns, sources []string, cursor int64, limit int) ([]LabelRow, int64, error) {
	if len(uriPatterns) == 0 {
		return nil, 0, nil
	}
	likePatterns := make([]string, len(uriPatterns))
	for i, p := range uriPatterns {
		likePatterns[i] = strings.ReplaceAll(p, "*", "%")
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id, src, uri, COALESCE(cid, ''), val, neg, cts, exp, sig, ver, COALESCE(blob_cid, ''), created_at
		FROM label
		WHERE uri LIKE ANY($1)
		  AND (CARDINALITY($2::text[]) = 0 OR src = ANY($2))
		  AND id > $3
		ORDER BY id ASC
		LIMIT $4
	`, likePatterns, sources, cursor, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []LabelRow
	var nextCursor int64
	for rows.Next() {
		var r LabelRow
		if err := rows.Scan(&r.ID, &r.Src, &r.URI, &r.CID, &r.Val, &r.Neg, &r.CTS, &r.Exp, &r.Sig, &r.Ver, &r.BlobCID, &r.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
		nextCursor = r.ID
	}
	if int64(len(out)) < int64(limit) {
		nextCursor = 0 // no more pages
	}
	return out, nextCursor, rows.Err()
}

// ListLabelsSince returns labels with id > cursor, ordered by id ASC, capped at
// limit. Used by subscribeLabels to deliver backlog on connect before live tailing.
func (m *PgStore) ListLabelsSince(ctx context.Context, cursor int64, limit int) ([]LabelRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT id, src, uri, COALESCE(cid, ''), val, neg, cts, exp, sig, ver, COALESCE(blob_cid, ''), created_at
		FROM label
		WHERE id > $1
		ORDER BY id ASC
		LIMIT $2
	`, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LabelRow
	for rows.Next() {
		var r LabelRow
		if err := rows.Scan(&r.ID, &r.Src, &r.URI, &r.CID, &r.Val, &r.Neg, &r.CTS, &r.Exp, &r.Sig, &r.Ver, &r.BlobCID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListPendingReviewItems returns the head of the review queue, filtered by
// source and category. Empty string fields match all values, except that an
// empty source omits undisputed 'label_applied' items — those are owner-facing
// dispute notifications (served via ListPendingAttestationsByAuthor), not
// moderator tasks. Once an owner disputes one it surfaces here for review.
// Pass source='label_applied' explicitly to see them all.
// order controls sort: "priority" (default) = priority DESC + created_at ASC,
// "oldest" = created_at ASC, "newest" = created_at DESC.
func (m *PgStore) ListPendingReviewItems(ctx context.Context, source, category, order string, limit, offset int) ([]ReviewItemRow, error) {
	orderClause := "ri.priority DESC, ri.created_at ASC"
	if order == "oldest" {
		orderClause = "ri.created_at ASC"
	} else if order == "newest" {
		orderClause = "ri.created_at DESC"
	}
	rows, err := m.pool.Query(ctx, `
		SELECT ri.id, ri.source, ri.source_ref, ri.subject_uri,
		       COALESCE(ri.subject_cid, ''), COALESCE(ri.blob_cid, ''),
		       COALESCE(ri.category, ''), ri.score, ri.status, ri.priority, ri.created_at,
		       COALESCE(r.reason_type, ''), COALESCE(r.reason_text, ''), COALESCE(r.reporter_did, ''),
		       ri.disputed, ri.disputed_at
		FROM review_item ri
		LEFT JOIN report r ON ri.source = 'report' AND ri.source_ref = r.id
		WHERE ri.status = 'pending'
		  AND ($1 = '' OR ri.source = $1)
		  AND ($1 <> '' OR ri.source <> 'label_applied' OR ri.disputed)
		  AND ($2 = '' OR ri.category = $2)
		ORDER BY `+orderClause+`
		LIMIT $3 OFFSET $4
	`, source, category, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReviewItemRow
	for rows.Next() {
		var r ReviewItemRow
		if err := rows.Scan(
			&r.ID, &r.Source, &r.SourceRef, &r.SubjectURI, &r.SubjectCID, &r.BlobCID,
			&r.Category, &r.Score, &r.Status, &r.Priority, &r.CreatedAt,
			&r.ReportReasonType, &r.ReportReasonText, &r.ReportReporterDID,
			&r.Disputed, &r.DisputedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReviewItem returns a single review_item by id, or nil if missing.
func (m *PgStore) GetReviewItem(ctx context.Context, id int64) (*ReviewItemRow, error) {
	var r ReviewItemRow
	err := m.pool.QueryRow(ctx, `
		SELECT ri.id, ri.source, ri.source_ref, ri.subject_uri,
		       COALESCE(ri.subject_cid, ''), COALESCE(ri.blob_cid, ''),
		       COALESCE(ri.category, ''), COALESCE(ri.label_val, ''), ri.score, ri.status, ri.priority, ri.created_at,
		       COALESCE(rep.reason_type, ''), COALESCE(rep.reason_text, ''), COALESCE(rep.reporter_did, ''),
		       ri.disputed, ri.disputed_at
		FROM review_item ri
		LEFT JOIN report rep ON ri.source = 'report' AND ri.source_ref = rep.id
		WHERE ri.id = $1
	`, id).Scan(
		&r.ID, &r.Source, &r.SourceRef, &r.SubjectURI, &r.SubjectCID, &r.BlobCID,
		&r.Category, &r.LabelVal, &r.Score, &r.Status, &r.Priority, &r.CreatedAt,
		&r.ReportReasonType, &r.ReportReasonText, &r.ReportReporterDID,
		&r.Disputed, &r.DisputedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// ListPendingAttestationsByAuthor returns pending review_items for blobs that
// the given DID has saved. Joins through blob_cid so all co-owners of a blob
// (original saver + resavers) see the same review_item.
func (m *PgStore) ListPendingAttestationsByAuthor(ctx context.Context, authorDID string, limit int) ([]ReviewItemRow, error) {
	if authorDID == "" {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id, source, source_ref, subject_uri, subject_cid, blob_cid,
		       category, label_val, score, status, priority, created_at,
		       reason_type, reason_text, reporter_did, disputed, disputed_at
		FROM (
			SELECT DISTINCT ON (ri.id)
			       ri.id, ri.source, ri.source_ref, ri.subject_uri,
			       COALESCE(ri.subject_cid, '')  AS subject_cid,
			       COALESCE(ri.blob_cid, '')     AS blob_cid,
			       COALESCE(ri.category, '')     AS category,
			       COALESCE(ri.label_val, '')    AS label_val,
			       ri.score, ri.status, ri.priority, ri.created_at,
			       COALESCE(r.reason_type, '')   AS reason_type,
			       COALESCE(r.reason_text, '')   AS reason_text,
			       COALESCE(r.reporter_did, '')  AS reporter_did,
			       ri.disputed, ri.disputed_at
			FROM review_item ri
			JOIN save s ON s.pds_blob_cid = ri.blob_cid
			LEFT JOIN report r ON ri.source = 'report' AND ri.source_ref = r.id
			WHERE s.author_did = $1
			  AND ri.status = 'pending'
			  AND ri.source IN ('ai', 'label_applied')
			ORDER BY ri.id
		) deduped
		ORDER BY priority DESC, created_at ASC
		LIMIT $2
	`, authorDID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReviewItemRow
	for rows.Next() {
		var r ReviewItemRow
		if err := rows.Scan(
			&r.ID, &r.Source, &r.SourceRef, &r.SubjectURI, &r.SubjectCID, &r.BlobCID,
			&r.Category, &r.LabelVal, &r.Score, &r.Status, &r.Priority, &r.CreatedAt,
			&r.ReportReasonType, &r.ReportReasonText, &r.ReportReporterDID,
			&r.Disputed, &r.DisputedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkReviewItemDisputed records an author's dispute of an auto-flag on their
// own content: sets disputed=true and bumps priority by priorityBump. Does NOT
// change status — the suspected label stays up and a moderator still owns the
// decision.
func (m *PgStore) MarkReviewItemDisputed(ctx context.Context, id int64, priorityBump int) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE review_item
		   SET disputed = TRUE,
		       disputed_at = now(),
		       priority = priority + $2
		 WHERE id = $1
		   AND disputed = FALSE
	`, id, priorityBump)
	return err
}

// ResolveLabelAppliedItems resolves any pending 'label_applied' review_items for
// a blob+val. Called when a label is negated: the dispute notification is moot
// once the label it pointed at is gone, so it shouldn't linger on the owner's
// notifications or (if disputed) in the moderator queue.
func (m *PgStore) ResolveLabelAppliedItems(ctx context.Context, blobCID, val string) error {
	if blobCID == "" || val == "" {
		return nil
	}
	_, err := m.pool.Exec(ctx, `
		UPDATE review_item
		   SET status = 'resolved'
		 WHERE source = 'label_applied'
		   AND status = 'pending'
		   AND blob_cid = $1
		   AND label_val = $2
	`, blobCID, val)
	return err
}

// GetSaveAuthor returns the author DID for a save URI, or "" if unknown.
// Used by self-attestation endpoints to enforce ownership.
func (m *PgStore) GetSaveAuthor(ctx context.Context, uri string) (string, error) {
	var did string
	err := m.pool.QueryRow(ctx, `SELECT author_did FROM save WHERE uri = $1`, uri).Scan(&did)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return did, nil
}

// ResolveReviewItem marks an item resolved or dismissed. resolverDID is required
// (no nil — auto-resolution isn't a thing yet).
func (m *PgStore) ResolveReviewItem(ctx context.Context, id int64, status, resolverDID string) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE review_item SET status = $2 WHERE id = $1 AND status = 'pending'
	`, id, status)
	return err
}

// CountPendingReviewItemsByBlobCID returns the number of pending review_items
// for a blob. Used by the ignore endpoint to clear harm_state once all items
// are resolved.
func (m *PgStore) CountPendingReviewItemsByBlobCID(ctx context.Context, blobCID string) (int64, error) {
	var count int64
	err := m.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM review_item
		WHERE blob_cid = $1 AND status = 'pending'
	`, blobCID).Scan(&count)
	return count, err
}

// ListSaveURIsByBlobCID returns every save URI sharing the given blob CID, in
// creation order. Used by the admin actions that need to apply a label or
// negation to every save of a moderated image.
func (m *PgStore) ListSaveURIsByBlobCID(ctx context.Context, blobCID string) ([]string, error) {
	if blobCID == "" {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT uri FROM save WHERE pds_blob_cid = $1 ORDER BY created_at ASC NULLS LAST, uri ASC
	`, blobCID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetHarmState updates blob_moderation_state.harm_state (clean | suspected | blocked).
// Used by TAP enrichment (suspected), the takedown action (blocked), and dismiss/ignore (clean).
func (m *PgStore) SetHarmState(ctx context.Context, blobCID, state, decidedBy, notes string) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO blob_moderation_state (blob_cid, harm_state, decided_by, decided_at, notes)
		VALUES ($1, $2, $3, now(), $4)
		ON CONFLICT (blob_cid) DO UPDATE
			SET harm_state = EXCLUDED.harm_state,
			    decided_by = EXCLUDED.decided_by,
			    decided_at = EXCLUDED.decided_at,
			    notes      = EXCLUDED.notes,
			    updated_at = now()
	`, blobCID, state, decidedBy, nullableString(notes))
	return err
}

// ListModerationEventsByBlobCID returns the audit-log entries for a blob, newest first.
func (m *PgStore) ListModerationEventsByBlobCID(ctx context.Context, blobCID string, limit int) ([]ModerationEventRow, error) {
	if blobCID == "" {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id, actor_did, action, COALESCE(subject_uri, ''), COALESCE(subject_cid, ''), COALESCE(blob_cid, ''),
		       payload, created_at
		FROM moderation_event
		WHERE blob_cid = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, blobCID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModerationEventRow
	for rows.Next() {
		var e ModerationEventRow
		if err := rows.Scan(&e.ID, &e.ActorDID, &e.Action, &e.SubjectURI, &e.SubjectCID, &e.BlobCID, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ModeratedBlobRow is one card in the moderation history list: a blob with
// activity, its most-recent event, and a deterministic sample save for preview.
type ModeratedBlobRow struct {
	BlobCID       string
	LatestAction  string
	LatestActor   string
	LatestEventAt time.Time
	SampleURI     string // deterministic sample save sharing the blob
	SampleAuthor  string // author_did of SampleURI, for the preview URL
}

// ListModeratedBlobs returns distinct blobs that have moderation_event activity,
// newest activity first, paginated. q (optional) matches a blob_cid prefix OR a
// save uri substring. Each row carries a deterministic sample save (earliest by
// created_at, then uri — same tiebreak as ListSaveURIsByBlobCID) for the preview
// thumbnail; the sample is empty when no save row survives for the blob.
func (m *PgStore) ListModeratedBlobs(ctx context.Context, q string, limit, offset int) ([]ModeratedBlobRow, error) {
	rows, err := m.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (blob_cid)
			       blob_cid, action, actor_did, created_at
			FROM moderation_event
			WHERE blob_cid IS NOT NULL AND blob_cid <> ''
			ORDER BY blob_cid, created_at DESC, id DESC
		),
		sample AS (
			SELECT DISTINCT ON (pds_blob_cid)
			       pds_blob_cid, uri, author_did
			FROM save
			WHERE pds_blob_cid <> ''
			ORDER BY pds_blob_cid, created_at ASC NULLS LAST, uri ASC
		)
		SELECT l.blob_cid, l.action, l.actor_did, l.created_at,
		       COALESCE(s.uri, ''), COALESCE(s.author_did, '')
		FROM latest l
		LEFT JOIN sample s ON s.pds_blob_cid = l.blob_cid
		WHERE ($1 = ''
		       OR l.blob_cid ILIKE $1 || '%'
		       OR EXISTS (
		           SELECT 1 FROM save sv
		           WHERE sv.pds_blob_cid = l.blob_cid
		             AND sv.uri ILIKE '%' || $1 || '%'
		       ))
		ORDER BY l.created_at DESC
		LIMIT $2 OFFSET $3
	`, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModeratedBlobRow
	for rows.Next() {
		var b ModeratedBlobRow
		if err := rows.Scan(&b.BlobCID, &b.LatestAction, &b.LatestActor, &b.LatestEventAt, &b.SampleURI, &b.SampleAuthor); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UnclassifiedBlob carries one row from ListUnclassifiedBlobsBatch: a blob CID
// that lacks a blob_moderation_state row, paired with a sample save URI for
// review-item attribution and the existing 768-d embedding for classification.
type UnclassifiedBlob struct {
	BlobCID   string
	SampleURI string
	Embedding []float32
}

// ListUnclassifiedBlobsBatch returns up to limit blobs that have an embedding
// but no blob_moderation_state row yet. Naturally resumable: each successful
// classification adds a blob_moderation_state row, removing it from this set,
// so re-calling with the same limit walks toward exhaustion.
func (m *PgStore) ListUnclassifiedBlobsBatch(ctx context.Context, limit int) ([]UnclassifiedBlob, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (s.pds_blob_cid)
			s.pds_blob_cid, s.uri, vi.embedding
		FROM save s
		JOIN visual_identity vi ON vi.id = s.visual_identity_id
		WHERE vi.embedding IS NOT NULL
		  AND s.pds_blob_cid <> ''
		  AND NOT EXISTS (
		    SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid
		  )
		ORDER BY s.pds_blob_cid, s.created_at ASC NULLS LAST, s.uri ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnclassifiedBlob
	for rows.Next() {
		var b UnclassifiedBlob
		var vec pgvector.Vector
		if err := rows.Scan(&b.BlobCID, &b.SampleURI, &vec); err != nil {
			return nil, err
		}
		b.Embedding = vec.Slice()
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetSuspectedBlobCIDs returns the subset of the given blob CIDs that have
// harm_state='suspected'. Used to hydrate viewer.suspected in XRPC save responses.
func (m *PgStore) GetSuspectedBlobCIDs(ctx context.Context, blobCIDs []string) (map[string]bool, error) {
	out := make(map[string]bool)
	if len(blobCIDs) == 0 {
		return out, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT blob_cid FROM blob_moderation_state
		WHERE blob_cid = ANY($1) AND harm_state = 'suspected'
	`, blobCIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return nil, err
		}
		out[cid] = true
	}
	return out, rows.Err()
}

// CountUnclassifiedBlobs is a one-shot count of blobs that need backfill.
// Used for progress logging in the backfill CLI.
func (m *PgStore) CountUnclassifiedBlobs(ctx context.Context) (int64, error) {
	var n int64
	err := m.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT s.pds_blob_cid)
		FROM save s
		JOIN visual_identity vi ON vi.id = s.visual_identity_id
		WHERE vi.embedding IS NOT NULL
		  AND s.pds_blob_cid <> ''
		  AND NOT EXISTS (
		    SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid
		  )
	`).Scan(&n)
	return n, err
}

// GetActiveLabelsByBlobCID returns the currently-active label values for a blob.
// Used by handleSaveUpsert to materialize labels onto new save URIs sharing the blob.
// Active = latest row per (src, val) is not negated.
func (m *PgStore) GetActiveLabelsByBlobCID(ctx context.Context, blobCID string) ([]string, error) {
	if blobCID == "" {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (src, val) val, neg
		FROM label
		WHERE blob_cid = $1
		ORDER BY src, val, id DESC
	`, blobCID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var val string
		var neg bool
		if err := rows.Scan(&val, &neg); err != nil {
			return nil, err
		}
		if !neg {
			out = append(out, val)
		}
	}
	return out, rows.Err()
}

// GetActiveLabelsByBlobCIDs is the batched form of GetActiveLabelsByBlobCID:
// it returns active label values keyed by blob CID for all requested CIDs in a
// single query. Used to hydrate collection-preview labels for a page of cards.
func (m *PgStore) GetActiveLabelsByBlobCIDs(ctx context.Context, blobCIDs []string) (map[string][]string, error) {
	out := make(map[string][]string)
	if len(blobCIDs) == 0 {
		return out, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (blob_cid, src, val) blob_cid, val, neg
		FROM label
		WHERE blob_cid = ANY($1)
		ORDER BY blob_cid, src, val, id DESC
	`, blobCIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, val string
		var neg bool
		if err := rows.Scan(&cid, &val, &neg); err != nil {
			return nil, err
		}
		if !neg {
			out[cid] = append(out[cid], val)
		}
	}
	return out, rows.Err()
}

// SelfLabelBackfillRow is a historical URI-scoped self-label paired with the
// blob CID of its save. Used by the self-label backfill to make the label
// blob-keyed so it propagates like a newly-declared one.
type SelfLabelBackfillRow struct {
	URI     string
	Val     string
	BlobCID string
}

// ListURIScopedSelfLabels returns active self-labels that predate blob-keyed
// propagation: label rows with NULL blob_cid whose val is a content-warning
// value, joined to their save's blob CID. Moderator/auto labels always carry a
// blob_cid, so the NULL filter isolates self-labels.
func (m *PgStore) ListURIScopedSelfLabels(ctx context.Context) ([]SelfLabelBackfillRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT l.uri, l.val, s.pds_blob_cid
		FROM label l
		JOIN save s ON s.uri = l.uri
		WHERE l.blob_cid IS NULL
		  AND l.neg = FALSE
		  AND l.val = ANY($1)
		  AND s.pds_blob_cid <> ''
	`, selfLabelValsList())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SelfLabelBackfillRow
	for rows.Next() {
		var r SelfLabelBackfillRow
		if err := rows.Scan(&r.URI, &r.Val, &r.BlobCID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetSelfLabelBlobCID backfills blob_cid onto a historical URI-scoped self-label
// so it joins the blob's active-label set and propagates to future resaves.
func (m *PgStore) SetSelfLabelBlobCID(ctx context.Context, uri, val, blobCID string) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE label SET blob_cid = $3
		WHERE uri = $1 AND val = $2 AND blob_cid IS NULL AND neg = FALSE
	`, uri, val, blobCID)
	return err
}

// InsertReport persists a user-submitted report and returns the new id.
func (m *PgStore) InsertReport(ctx context.Context, reporterDID, subjectURI, subjectCID, reasonType, reasonText string) (int64, error) {
	var id int64
	err := m.pool.QueryRow(ctx, `
		INSERT INTO report (reporter_did, subject_uri, subject_cid, reason_type, reason_text)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, reporterDID, subjectURI, nullableString(subjectCID), reasonType, nullableString(reasonText)).Scan(&id)
	return id, err
}

// InsertModerationEvent appends an audit-log entry.
func (m *PgStore) InsertModerationEvent(ctx context.Context, actorDID, action, subjectURI, subjectCID, blobCID string, payload []byte) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO moderation_event (actor_did, action, subject_uri, subject_cid, blob_cid, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, actorDID, action, nullableString(subjectURI), nullableString(subjectCID), nullableString(blobCID), nullableJSON(payload))
	return err
}

// IsModerator returns the moderator's role if the DID is active, or empty string.
func (m *PgStore) IsModerator(ctx context.Context, did string) (string, error) {
	var role string
	err := m.pool.QueryRow(ctx, `
		SELECT role FROM moderator WHERE did = $1 AND disabled_at IS NULL
	`, did).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

// RegistrationBucket is one UTC-day bucket of new user registrations.
type RegistrationBucket struct {
	Date  time.Time
	Count int
}

// RegistrationsByDay returns the count of users first indexed on each UTC day,
// ordered oldest-first. Only days with at least one registration appear; the
// caller (or the client) fills gaps and re-buckets to coarser granularities.
func (m *PgStore) RegistrationsByDay(ctx context.Context) ([]RegistrationBucket, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT date_trunc('day', created_at AT TIME ZONE 'UTC') AS day, count(*)
		FROM "user"
		GROUP BY day
		ORDER BY day
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []RegistrationBucket
	for rows.Next() {
		var b RegistrationBucket
		if err := rows.Scan(&b.Date, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func (m *PgStore) UpsertFollow(ctx context.Context, uri, followerDID, subjectDID string) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO follow (uri, follower_did, subject_did)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, uri, followerDID, subjectDID)
	return err
}

func (m *PgStore) DeleteFollow(ctx context.Context, uri string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM follow WHERE uri = $1`, uri)
	return err
}

func (m *PgStore) UpsertFavourite(ctx context.Context, uri, viewerDID, collectionURI string) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO favourite_collection (uri, viewer_did, collection_uri)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, uri, viewerDID, collectionURI)
	return err
}

func (m *PgStore) DeleteFavourite(ctx context.Context, uri string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM favourite_collection WHERE uri = $1`, uri)
	return err
}

// GetFollowURI returns the AT-URI of the follow record from followerDID to subjectDID, or "" if none.
func (m *PgStore) GetFollowURI(ctx context.Context, followerDID, subjectDID string) (string, error) {
	var uri string
	err := m.pool.QueryRow(ctx, `
		SELECT uri FROM follow WHERE follower_did = $1 AND subject_did = $2
	`, followerDID, subjectDID).Scan(&uri)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return uri, nil
}

// CountFollows returns how many actors follow did (followers) and how many did follows (follows).
func (m *PgStore) CountFollows(ctx context.Context, did string) (followers, follows int, err error) {
	err = m.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM follow WHERE subject_did = $1),
			(SELECT COUNT(*) FROM follow WHERE follower_did = $1)
	`, did).Scan(&followers, &follows)
	return
}

// GetFollowers returns the actors that follow subjectDID, newest follow first.
func (m *PgStore) GetFollowers(ctx context.Context, subjectDID string, limit, offset int) ([]ActorRow, error) {
	return m.queryFollowActors(ctx, `
		SELECT u.did, COALESCE(u.handle, ''), COALESCE(u.display_name, ''), COALESCE(u.description, ''),
		       COALESCE(u.pronouns, ''), COALESCE(u.website, ''), COALESCE(u.avatar, ''), COALESCE(u.banner, ''), u.created_at
		FROM follow f
		JOIN "user" u ON u.did = f.follower_did
		WHERE f.subject_did = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`, subjectDID, limit, offset)
}

// GetFollows returns the actors that followerDID follows, newest follow first.
func (m *PgStore) GetFollows(ctx context.Context, followerDID string, limit, offset int) ([]ActorRow, error) {
	return m.queryFollowActors(ctx, `
		SELECT u.did, COALESCE(u.handle, ''), COALESCE(u.display_name, ''), COALESCE(u.description, ''),
		       COALESCE(u.pronouns, ''), COALESCE(u.website, ''), COALESCE(u.avatar, ''), COALESCE(u.banner, ''), u.created_at
		FROM follow f
		JOIN "user" u ON u.did = f.subject_did
		WHERE f.follower_did = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`, followerDID, limit, offset)
}

func (m *PgStore) queryFollowActors(ctx context.Context, query, did string, limit, offset int) ([]ActorRow, error) {
	rows, err := m.pool.Query(ctx, query, did, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActorRow
	for rows.Next() {
		var row ActorRow
		if err := rows.Scan(&row.DID, &row.Handle, &row.DisplayName, &row.Description, &row.Pronouns, &row.Website, &row.Avatar, &row.Banner, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// FindFollowableCurrentsUsers returns the Currents users among candidateDIDs that
// viewerDID does not already follow (excluding the viewer themselves) — the
// candidates for the "import Bluesky follows" dialog. Reuses the ActorRow shape.
func (m *PgStore) FindFollowableCurrentsUsers(ctx context.Context, viewerDID string, candidateDIDs []string) ([]ActorRow, error) {
	if len(candidateDIDs) == 0 {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT u.did, COALESCE(u.handle, ''), COALESCE(u.display_name, ''), COALESCE(u.description, ''),
		       COALESCE(u.pronouns, ''), COALESCE(u.website, ''), COALESCE(u.avatar, ''), COALESCE(u.banner, ''), u.created_at
		FROM "user" u
		WHERE u.did = ANY($2)
		  AND u.did <> $1
		  AND NOT EXISTS (SELECT 1 FROM follow f WHERE f.follower_did = $1 AND f.subject_did = u.did)
		ORDER BY u.display_name NULLS LAST, u.handle
	`, viewerDID, candidateDIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActorRow
	for rows.Next() {
		var row ActorRow
		if err := rows.Scan(&row.DID, &row.Handle, &row.DisplayName, &row.Description, &row.Pronouns, &row.Website, &row.Avatar, &row.Banner, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// FollowerNotificationRow is one entry in a user's Activity feed: someone who
// followed them, plus whether the viewer follows that person back.
type FollowerNotificationRow struct {
	DID           string
	Handle        string
	DisplayName   string
	Avatar        string
	FollowedAt    time.Time
	FollowBackURI string // AT-URI of the viewer's follow-back record, "" if not following back
}

// ListFollowerNotifications returns the actors that follow did (newest first),
// joined with each one's follow-back record from did (for the "Following" state).
func (m *PgStore) ListFollowerNotifications(ctx context.Context, did string, limit, offset int) ([]FollowerNotificationRow, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT u.did, COALESCE(u.handle, ''), COALESCE(u.display_name, ''), COALESCE(u.avatar, ''),
		       f.created_at, COALESCE(fb.uri, '')
		FROM follow f
		JOIN "user" u ON u.did = f.follower_did
		LEFT JOIN follow fb ON fb.follower_did = $1 AND fb.subject_did = f.follower_did
		WHERE f.subject_did = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`, did, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FollowerNotificationRow
	for rows.Next() {
		var row FollowerNotificationRow
		if err := rows.Scan(&row.DID, &row.Handle, &row.DisplayName, &row.Avatar, &row.FollowedAt, &row.FollowBackURI); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// CountFollowersSince counts how many actors followed did after the given time.
func (m *PgStore) CountFollowersSince(ctx context.Context, did string, since time.Time) (int, error) {
	var n int
	err := m.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM follow WHERE subject_did = $1 AND created_at > $2
	`, did, since).Scan(&n)
	return n, err
}

// GetSocialSeenAt returns when the user last marked their Activity tab seen, or
// the zero time if they never have (so every follower counts as unseen).
func (m *PgStore) GetSocialSeenAt(ctx context.Context, did string) (time.Time, error) {
	var t time.Time
	err := m.pool.QueryRow(ctx, `
		SELECT social_seen_at FROM notification_seen WHERE viewer_did = $1
	`, did).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

// MarkSocialSeen records that the user has seen their Activity tab as of now.
func (m *PgStore) MarkSocialSeen(ctx context.Context, did string) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO notification_seen (viewer_did, social_seen_at)
		VALUES ($1, NOW())
		ON CONFLICT (viewer_did) DO UPDATE SET social_seen_at = NOW()
	`, did)
	return err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
