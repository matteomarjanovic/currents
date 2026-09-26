package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

const orphanGrace = 24 * time.Hour

type repositoryRecord struct {
	URI   string                     `json:"uri"`
	CID   string                     `json:"cid"`
	Value map[string]json.RawMessage `json:"value"`
}

func recordRef(record repositoryRecord, field string) string {
	var ref struct {
		URI string `json:"uri"`
	}
	_ = json.Unmarshal(record.Value[field], &ref)
	return ref.URI
}

func ownCollection(did, uri string) bool {
	p, err := syntax.ParseATURI(uri)
	return err == nil && p.Authority().String() == did && p.Collection().String() == collectionNSID && p.RecordKey().String() != ""
}

func listRepositoryRecords(ctx context.Context, c *atclient.APIClient, did, nsid string) ([]repositoryRecord, error) {
	var records []repositoryRecord
	cursor := ""
	for {
		params := map[string]any{"repo": did, "collection": nsid, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var page struct {
			Records []repositoryRecord `json:"records"`
			Cursor  string             `json:"cursor"`
		}
		if err := c.Get(ctx, "com.atproto.repo.listRecords", params, &page); err != nil {
			return nil, err
		}
		records = append(records, page.Records...)
		if page.Cursor == "" {
			return records, nil
		}
		if page.Cursor == cursor {
			return nil, fmt.Errorf("PDS repeated pagination cursor")
		}
		cursor = page.Cursor
	}
}

type repositorySnapshot struct {
	Commit      string
	Collections []repositoryRecord
	Saves       []repositoryRecord
}

func repositoryCommit(ctx context.Context, c *atclient.APIClient, did string) (string, error) {
	var out struct {
		CID string `json:"cid"`
	}
	err := c.Get(ctx, "com.atproto.sync.getLatestCommit", map[string]any{"did": did}, &out)
	if err == nil && out.CID == "" {
		err = fmt.Errorf("PDS returned an empty commit")
	}
	return out.CID, err
}

// Enumerate a stable PDS snapshot, never a partial list or the TAP mirror.
// A repo-wide swapCommit on every write also protects a missing parent that
// another client recreates between this scan and the repair.
func readRepository(ctx context.Context, c *atclient.APIClient, did string) (repositorySnapshot, error) {
	var snap repositorySnapshot
	var err error
	if snap.Commit, err = repositoryCommit(ctx, c, did); err != nil {
		return snap, err
	}
	if snap.Collections, err = listRepositoryRecords(ctx, c, did, collectionNSID); err != nil {
		return snap, err
	}
	if snap.Saves, err = listRepositoryRecords(ctx, c, did, saveNSID); err != nil {
		return snap, err
	}
	after, err := repositoryCommit(ctx, c, did)
	if err == nil && after != snap.Commit {
		err = fmt.Errorf("repository changed during enumeration; retry later")
	}
	return snap, err
}

type orphanRepair struct {
	Record repositoryRecord
	Field  string
	Target string
}

func (repair orphanRepair) write() map[string]any {
	value := maps.Clone(repair.Record.Value)
	delete(value, repair.Field) // Preserve unknown fields, self-labels and all content verbatim.
	uri, _ := syntax.ParseATURI(repair.Record.URI)
	return map[string]any{"$type": "com.atproto.repo.applyWrites#update", "collection": uri.Collection().String(), "rkey": uri.RecordKey().String(), "value": value}
}

func findOrphans(did string, snap repositorySnapshot) []orphanRepair {
	collections := make(map[string]bool, len(snap.Collections))
	for _, r := range snap.Collections {
		collections[r.URI] = true
	}
	var repairs []orphanRepair
	// Promote sections first. Their saves stay in the same collection and need
	// no rewrite: only a save whose collection itself is gone becomes Unsorted.
	for _, group := range []struct {
		records []repositoryRecord
		field   string
	}{
		{snap.Collections, "parent"}, {snap.Saves, "collection"},
	} {
		for _, record := range group.records {
			target := recordRef(record, group.field)
			if ownCollection(did, target) && !collections[target] {
				repairs = append(repairs, orphanRepair{record, group.field, target})
			}
		}
	}
	return repairs
}

// Each batch is atomic. On 429, InvalidSwap or any other failure the caller
// stops; the next pass re-reads the PDS instead of replaying stale payloads.
func applyRepositoryWrites(ctx context.Context, c *atclient.APIClient, did, commit string, writes []map[string]any) (string, error) {
	var out struct {
		Commit struct {
			CID string `json:"cid"`
		} `json:"commit"`
	}
	err := c.Post(ctx, "com.atproto.repo.applyWrites", map[string]any{
		"repo": did, "swapCommit": commit, "writes": writes,
	}, &out)
	if err != nil {
		return commit, err
	}
	if out.Commit.CID == "" {
		return "", fmt.Errorf("PDS omitted the resulting commit; re-read before continuing")
	}
	return out.Commit.CID, nil
}

type RepositoryMaintenance struct {
	Context context.Context
	Store   *PgStore
	OAuth   *oauth.ClientApp
	Dir     identity.Directory
}

func (w *RepositoryMaintenance) client(ctx context.Context, did string) (*atclient.APIClient, error) {
	sid, err := w.Store.LatestOAuthSessionID(ctx, did)
	if err != nil {
		return nil, err
	}
	if sid == "" {
		return nil, fmt.Errorf("no usable OAuth session; waiting for reconnect")
	}
	sess, err := w.OAuth.ResumeSession(ctx, syntax.DID(did), sid)
	if err != nil {
		return nil, err
	}
	return sess.APIClient(), nil
}

func (w *RepositoryMaintenance) Run() {
	nextRepair := time.Time{}
	for w.Context.Err() == nil {
		if err := w.deleteCollections(w.Context); err != nil {
			slog.Warn("collection deletion pass", "err", err)
		}
		if time.Now().After(nextRepair) {
			if _, err := w.repairOrphans(w.Context, "", false, false); err != nil {
				slog.Warn("orphan repair pass", "err", err)
			}
			nextRepair = time.Now().Add(24 * time.Hour)
			// A restart can lose an in-memory debounce timer. Invalidated medoids
			// remain NULL and are picked up here without re-embedding any images.
			collections, err := w.Store.ListCollectionsMissingEmbedding(w.Context)
			if err != nil {
				slog.Warn("listing missing collection embeddings", "err", err)
			}
			for _, uri := range collections {
				if err := recomputeCollectionEmbedding(w.Context, w.Store, uri); err != nil {
					slog.Warn("repairing collection embedding", "uri", uri, "err", err)
				}
			}
		}
		select {
		case <-w.Context.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

type OrphanRepairReport struct {
	Accounts   int `json:"accounts"`
	Candidates int `json:"candidates"`
	Eligible   int `json:"eligible"`
	Updated    int `json:"updated"`
}

func (w *RepositoryMaintenance) repairOrphans(ctx context.Context, did string, dryRun, now bool) (OrphanRepairReport, error) {
	var report OrphanRepairReport
	owners, err := w.Store.orphanOwners(ctx, did)
	if err != nil {
		return report, err
	}
	var failures []error
	for _, owner := range owners {
		accountCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		err := w.repairOwner(accountCtx, owner, dryRun, now, &report)
		cancel()
		if err != nil {
			slog.Warn("orphan repair deferred", "did", owner, "err", err)
			failures = append(failures, fmt.Errorf("%s: %w", owner, err))
		}
	}
	slog.Info("orphan repair finished", "dry_run", dryRun, "accounts", report.Accounts, "candidates", report.Candidates, "eligible", report.Eligible, "updated", report.Updated)
	return report, errors.Join(failures...)
}

func (w *RepositoryMaintenance) repairOwner(ctx context.Context, did string, dryRun, now bool, report *OrphanRepairReport) error {
	unlock, err := w.Store.lockRepository(ctx, did)
	if err != nil {
		return err
	}
	defer unlock()
	busy, err := w.Store.repositoryBusy(ctx, did)
	if err != nil {
		return err
	}
	if busy {
		slog.Info("orphan repair skipped: import or deletion in progress", "did", did)
		return nil
	}
	ident, err := w.Dir.LookupDID(ctx, syntax.DID(did))
	if err != nil {
		return err
	}
	public := atclient.NewAPIClient(ident.PDSEndpoint())
	public.Client = &http.Client{Timeout: 30 * time.Second}
	snap, err := readRepository(ctx, public, did)
	if err != nil {
		return err
	}
	report.Accounts++
	repairs := findOrphans(did, snap)
	report.Candidates += len(repairs)
	seen := make([]string, 0, len(repairs))
	var writes []map[string]any
	var writtenURIs []string
	for _, repair := range repairs {
		seen = append(seen, repair.Record.URI)
		firstSeen, err := w.Store.observeOrphan(ctx, did, repair.Record.URI, repair.Target, dryRun)
		if err != nil {
			return err
		}
		eligible := now || time.Since(firstSeen) >= orphanGrace
		slog.Info("orphan repair candidate", "uri", repair.Record.URI, "remove_field", repair.Field, "missing", repair.Target, "eligible", eligible, "dry_run", dryRun)
		if !eligible {
			continue
		}
		report.Eligible++
		writes = append(writes, repair.write())
		writtenURIs = append(writtenURIs, repair.Record.URI)
	}
	if dryRun {
		return nil
	}
	// Clear observations when records/references changed or the parent returned.
	if _, err := w.Store.pool.Exec(ctx, `DELETE FROM orphan_record WHERE owner_did = $1 AND NOT (uri = ANY($2))`, did, seen); err != nil {
		return err
	}
	if len(writes) == 0 {
		return nil
	}
	c, err := w.client(ctx, did)
	if err != nil {
		return err
	}
	start := 0
	for batch := range slices.Chunk(writes, 200) {
		snap.Commit, err = applyRepositoryWrites(ctx, c, did, snap.Commit, batch)
		if err != nil {
			return err
		}
		report.Updated += len(batch)
		// A second orphaning of the same URI must earn a fresh grace period.
		// Remove observations batch by batch so a later 429 cannot retain old
		// timestamps for the records this pass already repaired.
		if _, err := w.Store.pool.Exec(ctx, `DELETE FROM orphan_record WHERE owner_did = $1 AND uri = ANY($2)`,
			did, writtenURIs[start:start+len(batch)]); err != nil {
			return err
		}
		start += len(batch)
	}
	return nil
}

// The parent is last so a stalled delete never hides surviving sections.
// Re-enumeration also catches sections/saves that TAP hadn't indexed when the
// request arrived. A completed job remains a tombstone until TAP catches up.
func deletionWrites(root string, snap repositorySnapshot) ([]string, []map[string]any) {
	targets := []string{root}
	for _, r := range snap.Collections {
		if recordRef(r, "parent") == root {
			targets = append(targets, r.URI)
		}
	}
	var writes []map[string]any
	appendDelete := func(uri string) {
		p, _ := syntax.ParseATURI(uri)
		writes = append(writes, map[string]any{"$type": "com.atproto.repo.applyWrites#delete", "collection": p.Collection().String(), "rkey": p.RecordKey().String()})
	}
	for _, r := range snap.Saves {
		if slices.Contains(targets, recordRef(r, "collection")) {
			appendDelete(r.URI)
		}
	}
	for _, r := range snap.Collections {
		if r.URI != root && slices.Contains(targets, r.URI) {
			appendDelete(r.URI)
		}
	}
	for _, r := range snap.Collections {
		if r.URI == root {
			appendDelete(r.URI)
		}
	}
	return targets, writes
}

func (w *RepositoryMaintenance) deleteCollections(ctx context.Context) error {
	if _, err := w.Store.pool.Exec(ctx, `DELETE FROM collection_delete_job j WHERE completed_at IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM collection c WHERE c.uri = j.collection_uri OR c.parent_uri = j.collection_uri)`); err != nil {
		return err
	}
	rows, err := w.Store.pool.Query(ctx, `SELECT owner_did, collection_uri FROM collection_delete_job WHERE completed_at IS NULL ORDER BY created_at`)
	if err != nil {
		return err
	}
	var jobs [][2]string
	for rows.Next() {
		var job [2]string
		if err := rows.Scan(&job[0], &job[1]); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, job := range jobs {
		jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		err := w.deleteCollection(jobCtx, job[0], job[1])
		cancel()
		if err != nil {
			slog.Warn("collection deletion deferred", "uri", job[1], "err", err)
			if _, saveErr := w.Store.pool.Exec(ctx, `UPDATE collection_delete_job SET error = $2 WHERE collection_uri = $1`, job[1], err.Error()); saveErr != nil {
				return saveErr
			}
		}
	}
	return nil
}

func (w *RepositoryMaintenance) deleteCollection(ctx context.Context, did, root string) error {
	unlock, err := w.Store.lockRepository(ctx, did)
	if err != nil {
		return err
	}
	defer unlock()
	c, err := w.client(ctx, did)
	if err != nil {
		return err
	}
	return w.deleteCollectionRecords(ctx, c, did, root)
}

func (w *RepositoryMaintenance) deleteCollectionRecords(ctx context.Context, c *atclient.APIClient, did, root string) error {
	snap, err := readRepository(ctx, c, did)
	if err != nil {
		return err
	}
	targets, writes := deletionWrites(root, snap)
	if err := w.Store.cancelCollectionImports(ctx, did, targets); err != nil {
		return err
	}
	for batch := range slices.Chunk(writes, 200) {
		snap.Commit, err = applyRepositoryWrites(ctx, c, did, snap.Commit, batch)
		if err != nil {
			return err
		}
	}
	_, err = w.Store.pool.Exec(ctx, `UPDATE collection_delete_job SET completed_at = now(), error = '' WHERE collection_uri = $1`, root)
	if err == nil {
		slog.Info("collection deletion completed", "uri", root, "records", len(writes))
	}
	return err
}
