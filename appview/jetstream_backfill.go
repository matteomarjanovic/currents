package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

var jetstreamCollections = []string{
	currentsProfileNSID, collectionNSID, followNSID, favouriteNSID, saveNSID,
}

func runJetstreamBackfill(ctx context.Context, handler *TapHandler) {
	for ctx.Err() == nil {
		did, due, attempts, err := handler.Store.NextJetstreamBackfill(ctx)
		if err != nil {
			slog.Warn("Jetstream backfill queue", "err", err)
		} else if did != "" {
			err = reconcileJetstreamRepo(ctx, handler, did)
			if err == nil {
				err = handler.Store.CompleteJetstreamBackfill(ctx, did, due)
			}
			if err != nil {
				slog.Warn("Jetstream PDS backfill deferred", "did", did, "err", err)
				delay := min(3600, 30<<min(attempts, 7))
				if retryErr := handler.Store.RetryJetstreamBackfill(ctx, did, due, delay); retryErr != nil {
					slog.Error("Jetstream backfill retry", "did", did, "err", retryErr)
				}
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

func pdsRepoRevision(ctx context.Context, c *atclient.APIClient, did string) (string, string, error) {
	var out struct {
		CID string `json:"cid"`
		Rev string `json:"rev"`
	}
	err := c.Get(ctx, "com.atproto.sync.getLatestCommit", map[string]any{"did": did}, &out)
	if err == nil && (out.CID == "" || out.Rev == "") {
		err = fmt.Errorf("PDS returned an incomplete commit")
	}
	return out.CID, out.Rev, err
}

// A complete stable PDS snapshot is required before changing any index rows.
func readJetstreamRepo(ctx context.Context, c *atclient.APIClient, did string) (map[string][]repositoryRecord, string, error) {
	beforeCID, rev, err := pdsRepoRevision(ctx, c, did)
	if err != nil {
		return nil, "", err
	}
	records := make(map[string][]repositoryRecord, len(jetstreamCollections))
	for _, nsid := range jetstreamCollections {
		records[nsid], err = listRepositoryRecords(ctx, c, did, nsid)
		if err != nil {
			return nil, "", err
		}
		for _, record := range records[nsid] {
			uri, err := syntax.ParseATURI(record.URI)
			if err != nil || uri.Authority().String() != did || uri.Collection().String() != nsid ||
				uri.RecordKey().String() == "" || record.CID == "" || record.Value == nil {
				return nil, "", fmt.Errorf("invalid PDS record %q", record.URI)
			}
		}
	}
	afterCID, afterRev, err := pdsRepoRevision(ctx, c, did)
	if err != nil {
		return nil, "", err
	}
	if beforeCID != afterCID || rev != afterRev {
		return nil, "", fmt.Errorf("repository changed during Jetstream backfill")
	}
	return records, rev, nil
}

func reconcileJetstreamRepo(ctx context.Context, handler *TapHandler, did string) error {
	unlock, err := handler.Store.lockRepository(ctx, did)
	if err != nil {
		return err
	}
	defer unlock()
	state, _, err := handler.Store.JetstreamRepoState(ctx, did)
	if err != nil || state != "active" {
		return err
	}
	ident, err := handler.Dir.LookupDID(ctx, syntax.DID(did))
	if err != nil {
		return err
	}
	client := atclient.NewAPIClient(ident.PDSEndpoint())
	client.Client = &http.Client{Timeout: 30 * time.Second}
	records, rev, err := readJetstreamRepo(ctx, client, did)
	if err != nil {
		return err
	}

	present := make(map[string]bool)
	for _, group := range records {
		for _, record := range group {
			present[record.URI] = true
		}
	}
	rows, err := handler.Store.pool.Query(ctx, `
		SELECT 'save', uri FROM save WHERE author_did = $1
		UNION ALL SELECT 'collection', uri FROM collection WHERE author_did = $1
		UNION ALL SELECT 'follow', uri FROM follow WHERE follower_did = $1
		UNION ALL SELECT 'favourite', uri FROM favourite_collection WHERE viewer_did = $1
	`, did)
	if err != nil {
		return err
	}
	type indexedRecord struct{ kind, uri string }
	var indexed []indexedRecord
	for rows.Next() {
		var r indexedRecord
		if err := rows.Scan(&r.kind, &r.uri); err != nil {
			rows.Close()
			return err
		}
		indexed = append(indexed, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range indexed {
		if present[r.uri] {
			continue
		}
		switch r.kind {
		case "save":
			err = handler.Store.DeleteSave(ctx, r.uri)
		case "collection":
			err = handler.Store.DeleteCollection(ctx, r.uri)
		case "follow":
			err = handler.Store.DeleteFollow(ctx, r.uri)
		case "favourite":
			err = handler.Store.DeleteFavourite(ctx, r.uri)
		}
		if err != nil {
			return fmt.Errorf("removing stale %s %s: %w", r.kind, r.uri, err)
		}
	}
	for _, nsid := range jetstreamCollections {
		for _, record := range records[nsid] {
			uri, _ := syntax.ParseATURI(record.URI)
			value, err := json.Marshal(record.Value)
			if err != nil {
				return err
			}
			if err := handleTapRecord(ctx, handler, &TapRecordEvent{
				DID: did, Collection: nsid, Rkey: uri.RecordKey().String(),
				Action: "update", CID: record.CID, Record: value,
			}); err != nil {
				if errors.Is(err, errSkipRecord) {
					slog.Warn("Jetstream PDS record skipped", "uri", record.URI, "err", err)
					continue
				}
				return fmt.Errorf("backfilling %s: %w", record.URI, err)
			}
		}
	}
	if err := handler.Store.SetJetstreamReconciledRev(ctx, did, rev); err != nil {
		return err
	}
	slog.Info("Jetstream repo reconciled with PDS", "did", did, "rev", rev, "records", len(present))
	return nil
}
