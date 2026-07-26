package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"
)

// rateLimitCooldown is how long the backfill pauses after a PDS answers 429, to
// give a throttled host room to recover before we move on.
const rateLimitCooldown = 30 * time.Second

func isBlobRateLimited(err error) bool {
	return err != nil && strings.Contains(err.Error(), "returned 429")
}

// runBackfillMimeTypes fills save.mime_type for existing image saves that
// predate the column, so the web client's GIF-freeze setting applies to them.
// For each distinct blob it fetches the bytes and sniffs the real format with
// http.DetectContentType — what the browser actually animates on, so a GIF that
// was transcoded to a static image at upload correctly reads as non-GIF.
//
// Resumable: stamping mime_type drops a blob from the pool, so a rerun retries
// only blobs that failed to fetch. Use --dry-run to preview the first batch.
func runBackfillMimeTypes(cctx *cli.Context) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	batchSize := cctx.Int("batch-size")
	interval := cctx.Duration("interval")
	limit := cctx.Int("limit")
	dryRun := cctx.Bool("dry-run")
	rps := cctx.Float64("rate")

	// Pace blob fetches to a gentle, steady rate so PDSes see a well-behaved
	// crawler with a token-bucket cap rather than bursts. Combined with the
	// sequential loop and the spread across many hosts, per-PDS load stays low.
	// A non-positive --rate disables the limiter.
	lim := rate.Limit(rps)
	if rps <= 0 {
		lim = rate.Inf
	}
	limiter := rate.NewLimiter(lim, 1)

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

	dir := identity.DefaultDirectory()

	pending, err := store.CountSaveBlobsMissingMimeType(ctx)
	if err != nil {
		return fmt.Errorf("count pending: %w", err)
	}
	slog.Info("mime-type backfill starting",
		"pending_blobs", pending,
		"batch_size", batchSize,
		"interval", interval,
		"rate_per_sec", rps,
		"limit_arg", limit,
		"dry_run", dryRun,
	)

	processed := 0
	afterCID := ""
	for {
		if limit > 0 && processed >= limit {
			slog.Info("mime-type backfill: --limit reached", "processed", processed)
			return nil
		}

		batchLimit := batchSize
		if limit > 0 && processed+batchLimit > limit {
			batchLimit = limit - processed
		}

		batch, err := store.ListSaveBlobsMissingMimeTypeBatch(ctx, afterCID, batchLimit)
		if err != nil {
			return fmt.Errorf("list batch: %w", err)
		}
		if len(batch) == 0 {
			slog.Info("mime-type backfill complete", "processed", processed)
			return nil
		}

		for _, b := range batch {
			if ctx.Err() != nil {
				slog.Info("mime-type backfill interrupted", "processed", processed)
				return nil
			}
			afterCID = b.BlobCID

			// Block until the limiter grants a token (or the run is cancelled),
			// so fetches leave at a steady rate rather than in bursts.
			if err := limiter.Wait(ctx); err != nil {
				slog.Info("mime-type backfill interrupted", "processed", processed)
				return nil
			}

			imageBytes, _, err := fetchBlobFromPDS(ctx, store, dir, b.BlobDID, b.BlobCID)
			if err != nil {
				if isBlobRateLimited(err) {
					slog.Warn("mime-type backfill: PDS answered 429, cooling down", "blob_cid", b.BlobCID, "cooldown", rateLimitCooldown)
					select {
					case <-ctx.Done():
						slog.Info("mime-type backfill interrupted", "processed", processed)
						return nil
					case <-time.After(rateLimitCooldown):
					}
				} else {
					slog.Warn("mime-type backfill: fetch blob", "blob_cid", b.BlobCID, "err", err)
				}
				continue
			}
			// Sniff the actual bytes rather than trusting the PDS Content-Type
			// header (some PDSes return application/octet-stream).
			mime := http.DetectContentType(imageBytes)

			if dryRun {
				slog.Info("mime-type backfill DRY", "blob_cid", b.BlobCID, "mime", mime)
				processed++
				continue
			}

			if err := store.UpdateSaveMimeTypeByCID(ctx, b.BlobCID, mime); err != nil {
				slog.Warn("mime-type backfill: update saves", "blob_cid", b.BlobCID, "err", err)
				continue
			}
			processed++
		}

		slog.Info("mime-type backfill batch done", "size", len(batch), "processed_total", processed, "remaining_in_pool", pending-int64(processed))

		if dryRun {
			slog.Info("mime-type backfill: --dry-run, stopping after first batch")
			return nil
		}

		// Throttle so live TAP enrichment and PDSes aren't hammered.
		select {
		case <-ctx.Done():
			slog.Info("mime-type backfill interrupted", "processed", processed)
			return nil
		case <-time.After(interval):
		}
	}
}
