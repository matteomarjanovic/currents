package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func applyTapRecord(ctx context.Context, handler *TapHandler, ev *TapRecordEvent) error {
	unlock, err := handler.Store.lockRepository(ctx, ev.DID)
	if err != nil {
		return err
	}
	defer unlock()
	var deleted bool
	if err := handler.Store.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tap_deleted_repo WHERE did = $1)`, ev.DID).Scan(&deleted); err != nil {
		return err
	}
	if deleted {
		return nil // Ack stale buffered records without restoring deleted content.
	}
	return handleTapRecord(ctx, handler, ev)
}

// Caller holds lockRepository. Persist the marker first so partial cleanup is
// safe to retry and concurrent/buffered TAP events cannot recreate the rows.
// This never writes to the PDS or changes another user's records or billing.
func (m *PgStore) deleteIndexedAccount(ctx context.Context, did string) error {
	if _, err := m.pool.Exec(ctx, `INSERT INTO tap_deleted_repo (did) VALUES ($1) ON CONFLICT DO NOTHING`, did); err != nil {
		return err
	}
	if err := m.ClearIndexedRepo(ctx, did); err != nil {
		return err
	}
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`UPDATE import_job SET status = 'failed', error = 'AT Protocol account deleted', updated_at = now()
		 WHERE owner_did = $1 AND status IN ('listing', 'running')`,
		`UPDATE import_item SET status = 'failed', error = 'AT Protocol account deleted', updated_at = now()
		 WHERE owner_did = $1 AND status IN ('queued', 'running')`,
		`DELETE FROM collection_delete_job WHERE owner_did = $1`,
		`DELETE FROM orphan_record WHERE owner_did = $1`,
		`DELETE FROM pds_wipe WHERE did = $1`,
		`DELETE FROM oauth_sessions WHERE account_did = $1`,
	} {
		if _, err := tx.Exec(ctx, q, did); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.Info("deleted AT Protocol account removed from index", "did", did)
	return nil
}

type repoAccountStatus struct {
	DID    string `json:"did"`
	Active *bool  `json:"active"`
	Status string `json:"status"`
}

// Use the same public-only transport as blob fetching: PDS endpoints come from
// DID documents. HTTP errors are uncertainty, except the explicit RepoNotFound
// response used to confirm an already-deleted account at its current PDS.
func readRepoAccountStatus(ctx context.Context, endpoint, did string) (repoAccountStatus, bool, error) {
	var status repoAccountStatus
	u, err := validateFetchURL(endpoint)
	if err != nil {
		return status, false, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/xrpc/com.atproto.sync.getRepoStatus"
	u.RawQuery = url.Values{"did": {did}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return status, false, err
	}
	req.Header.Set("User-Agent", blobUserAgent)
	resp, err := blobHTTPClient.Do(req)
	if err != nil {
		return status, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var body struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err == nil &&
			resp.StatusCode == http.StatusBadRequest && body.Error == "RepoNotFound" {
			return status, true, nil
		}
		return status, false, fmt.Errorf("getRepoStatus at %s: HTTP %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&status); err != nil {
		return status, false, err
	}
	if status.DID != did || status.Active == nil || *status.Active && status.Status == "deleted" {
		return status, false, fmt.Errorf("invalid getRepoStatus response at %s", endpoint)
	}
	return status, false, nil
}

func confirmDeletedAccount(ctx context.Context, dir identity.Directory, relay, did string) (bool, error) {
	status, missing, err := readRepoAccountStatus(ctx, relay, did)
	if err != nil {
		return false, err
	}
	if missing {
		return false, fmt.Errorf("relay has no status for %s", did)
	}
	if *status.Active || status.Status != "deleted" {
		return false, nil
	}
	// Resolve afresh so a stale PDS endpoint after migration cannot confirm deletion.
	if err := dir.Purge(ctx, syntax.DID(did).AtIdentifier()); err != nil {
		return false, err
	}
	ident, err := dir.LookupDID(ctx, syntax.DID(did))
	if err != nil {
		return false, err
	}
	status, missing, err = readRepoAccountStatus(ctx, ident.PDSEndpoint(), did)
	if err != nil {
		return false, err
	}
	return missing || !*status.Active && status.Status == "deleted", nil
}

// Default is inspection only. Include owners without profile rows and deletion
// markers left by interrupted cleanup, not just accounts in the user table.
func purgeDeletedAccounts(ctx context.Context, store *PgStore, dir identity.Directory, relay, did string, apply bool) error {
	rows, err := store.pool.Query(ctx, `SELECT did FROM (
		SELECT did FROM "user" UNION SELECT author_did FROM save
		UNION SELECT author_did FROM collection UNION SELECT follower_did FROM follow
		UNION SELECT viewer_did FROM favourite_collection UNION SELECT did FROM tap_deleted_repo
	) owners WHERE $1 = '' OR did = $1 ORDER BY did`, did)
	if err != nil {
		return err
	}
	var owners []string
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			rows.Close()
			return err
		}
		owners = append(owners, owner)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if did != "" && len(owners) == 0 {
		return fmt.Errorf("no indexed account for %s", did)
	}
	deleted, failed := 0, 0
	for _, owner := range owners {
		// Coordinate the check and cleanup with TAP, imports and account writes.
		unlock, err := store.lockRepository(ctx, owner)
		if err != nil {
			return err
		}
		confirmed, err := confirmDeletedAccount(ctx, dir, relay, owner)
		if err == nil && confirmed {
			slog.Info("confirmed deleted AT Protocol account", "did", owner, "apply", apply)
			if apply {
				err = store.deleteIndexedAccount(ctx, owner)
			}
			deleted++
		}
		unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			slog.Warn("account deletion audit deferred", "did", owner, "err", err)
			failed++
		}
	}
	slog.Info("account deletion audit finished", "accounts", len(owners), "deleted", deleted, "failed", failed, "apply", apply)
	if failed != 0 {
		return fmt.Errorf("%d accounts could not be checked or cleaned up", failed)
	}
	return nil
}
