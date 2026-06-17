package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

// runBackfillSelfLabels makes historical creator self-labels propagate like
// newly-declared ones. Self-labels used to be URI-scoped (NULL blob_cid), so a
// resave never inherited them. This sets blob_cid on each one (so future resaves
// inherit via materialization) and fans the warning out to copies of the same
// bytes that are already indexed. Idempotent: re-running is a no-op once a row is
// blob-keyed and every sibling carries the label.
func runBackfillSelfLabels(cctx *cli.Context) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dryRun := cctx.Bool("dry-run")

	store, err := NewPgStore(ctx, &PgStoreConfig{
		DSN:                       cctx.String("database-url"),
		SessionExpiryDuration:     time.Hour * 24 * 90,
		SessionInactivityDuration: time.Hour * 24 * 14,
		AuthRequestExpiryDuration: time.Minute * 30,
		MinConns:                  int32(cctx.Int("db-min-conns")),
		MaxConns:                  int32(cctx.Int("db-max-conns")),
		MaxConnLifetime:           cctx.Duration("db-max-conn-lifetime"),
		MaxConnIdleTime:           cctx.Duration("db-max-conn-idle-time"),
	})
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}

	signer, err := NewLabelerSigner(cctx.String("labeler-did"), cctx.String("labeler-signing-key"))
	if err != nil {
		return fmt.Errorf("labeler signer: %w", err)
	}
	labeler := NewLabelerIssuer(signer, store)
	if labeler == nil && !dryRun {
		return errors.New("labeler not configured: set LABELER_DID and LABELER_SIGNING_KEY (or use --dry-run)")
	}

	items, err := store.ListURIScopedSelfLabels(ctx)
	if err != nil {
		return fmt.Errorf("list uri-scoped self-labels: %w", err)
	}

	// Distinct (blob, val) for fan-out; the full list drives the blob_cid backfill.
	type blobVal struct{ blob, val string }
	seen := map[blobVal]bool{}
	var fanout []blobVal
	for _, it := range items {
		k := blobVal{it.BlobCID, it.Val}
		if !seen[k] {
			seen[k] = true
			fanout = append(fanout, k)
		}
	}

	slog.Info("self-label backfill starting", "self_label_rows", len(items), "distinct_blob_val", len(fanout), "dry_run", dryRun)

	if dryRun {
		for _, k := range fanout {
			uris, _ := store.ListSaveURIsByBlobCID(ctx, k.blob)
			slog.Info("self-label backfill DRY", "blob_cid", k.blob, "val", k.val, "sibling_uris", len(uris))
		}
		slog.Info("self-label backfill: --dry-run, no writes")
		return nil
	}

	// 1. Make the original self-label rows blob-keyed so future resaves inherit.
	for _, it := range items {
		if ctx.Err() != nil {
			slog.Info("self-label backfill interrupted")
			return nil
		}
		if err := store.SetSelfLabelBlobCID(ctx, it.URI, it.Val, it.BlobCID); err != nil {
			slog.Warn("set self-label blob_cid", "uri", it.URI, "val", it.Val, "err", err)
		}
	}

	// 2. Fan out each (blob, val) to existing copies that don't already carry it.
	issued := 0
	for _, k := range fanout {
		if ctx.Err() != nil {
			slog.Info("self-label backfill interrupted", "labels_issued", issued)
			return nil
		}
		uris, err := store.ListSaveURIsByBlobCID(ctx, k.blob)
		if err != nil {
			slog.Warn("list sibling uris", "blob_cid", k.blob, "err", err)
			continue
		}
		existing, err := store.GetLabelsByURIs(ctx, uris)
		if err != nil {
			slog.Warn("get labels by uris", "blob_cid", k.blob, "err", err)
			continue
		}
		for _, u := range uris {
			if labelRowsHaveVal(existing[u], k.val) {
				continue
			}
			if _, err := labeler.IssueLabel(ctx, IssueLabelParams{
				Actor:   "auto",
				Action:  ActionLabelAdd,
				URI:     u,
				BlobCID: k.blob,
				Val:     k.val,
			}); err != nil {
				slog.Warn("issue backfill label", "uri", u, "val", k.val, "err", err)
				continue
			}
			issued++
		}
	}

	slog.Info("self-label backfill complete", "labels_issued", issued)
	return nil
}

func labelRowsHaveVal(rows []LabelRow, val string) bool {
	for _, r := range rows {
		if r.Val == val {
			return true
		}
	}
	return false
}
