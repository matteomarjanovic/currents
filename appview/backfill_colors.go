package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/urfave/cli/v2"
)

// runBackfillColors re-extracts the dominant-color palette of every visual
// identity that has no color-index rows yet, via the inference server's
// /palette endpoint, and refreshes the stored palettes on the saves sharing
// each canonical blob. Resumable: populated VIs drop out of the pool, and a
// rerun retries blobs that failed to fetch.
//
// Use --dry-run to preview the first batch's palettes without writing anything.
func runBackfillColors(cctx *cli.Context) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	batchSize := cctx.Int("batch-size")
	interval := cctx.Duration("interval")
	limit := cctx.Int("limit")
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

	inference := NewInferenceClient(cctx.String("inference-url"))
	dir := identity.DefaultDirectory()

	pending, err := store.CountVIsMissingColors(ctx)
	if err != nil {
		return fmt.Errorf("count pending: %w", err)
	}
	slog.Info("colors backfill starting",
		"pending_vis", pending,
		"batch_size", batchSize,
		"interval", interval,
		"limit_arg", limit,
		"dry_run", dryRun,
	)

	processed := 0
	afterID := ""
	for {
		if limit > 0 && processed >= limit {
			slog.Info("colors backfill: --limit reached", "processed", processed)
			return nil
		}

		batchLimit := batchSize
		if limit > 0 && processed+batchLimit > limit {
			batchLimit = limit - processed
		}

		batch, err := store.ListVIsMissingColorsBatch(ctx, afterID, batchLimit)
		if err != nil {
			return fmt.Errorf("list batch: %w", err)
		}
		if len(batch) == 0 {
			slog.Info("colors backfill complete", "processed", processed)
			return nil
		}

		for _, vi := range batch {
			if ctx.Err() != nil {
				slog.Info("colors backfill interrupted", "processed", processed)
				return nil
			}
			afterID = vi.ID

			imageBytes, mimeType, err := fetchBlobFromPDS(ctx, store, dir, vi.BlobDID, vi.BlobCID)
			if err != nil {
				slog.Warn("colors backfill: fetch blob", "vi_id", vi.ID, "blob_cid", vi.BlobCID, "err", err)
				continue
			}
			colors, err := inference.Palette(ctx, imageBytes, mimeType)
			if err != nil {
				slog.Warn("colors backfill: extract palette", "vi_id", vi.ID, "blob_cid", vi.BlobCID, "err", err)
				continue
			}

			if dryRun {
				slog.Info("colors backfill DRY", "vi_id", vi.ID, "blob_cid", vi.BlobCID, "palette", string(colors))
				processed++
				continue
			}

			if err := store.SetVIColors(ctx, vi.ID, colors); err != nil {
				slog.Warn("colors backfill: set vi colors", "vi_id", vi.ID, "err", err)
				continue
			}
			if err := store.UpdateSaveDominantColorsByCID(ctx, vi.BlobCID, colors); err != nil {
				slog.Warn("colors backfill: update save palettes", "blob_cid", vi.BlobCID, "err", err)
			}
			processed++
		}

		slog.Info("colors backfill batch done", "size", len(batch), "processed_total", processed, "remaining_in_pool", pending-int64(processed))

		if dryRun {
			slog.Info("colors backfill: --dry-run, stopping after first batch")
			return nil
		}

		// Throttle so live TAP enrichment and PDSes aren't hammered.
		select {
		case <-ctx.Done():
			slog.Info("colors backfill interrupted", "processed", processed)
			return nil
		case <-time.After(interval):
		}
	}
}
