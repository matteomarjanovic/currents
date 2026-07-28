package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"
)

// mean averages an accumulated total over a count, for the per-batch
// timing line that tells you which half of the work to tune.
func mean(total, n int64) int64 {
	if n == 0 {
		return 0
	}
	return total / n
}

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
	concurrency := cctx.Int("concurrency")
	rps := cctx.Float64("rate")

	// Each item is a PDS round trip plus a palette call, so the run is latency
	// bound: workers overlap those waits, the token bucket caps the aggregate
	// rate whatever the worker count. A non-positive --rate disables the cap.
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

	inference := NewInferenceClient(cctx.String("inference-url"))
	dir := identity.DefaultDirectory()

	pending, err := store.CountVIsMissingColors(ctx)
	if err != nil {
		return fmt.Errorf("count pending: %w", err)
	}
	slog.Info("colors backfill starting",
		"pending_vis", pending,
		"batch_size", batchSize,
		"concurrency", concurrency,
		"rate_per_sec", rps,
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

		// The batch is ordered by id, so the cursor can advance past the whole
		// page up front: items that fail stay in the pool for a rerun anyway
		// (the NOT EXISTS only drops the ones that got colors written).
		afterID = batch[len(batch)-1].ID

		var done, fetchMS, paletteMS, fetchedBytes atomic.Int64
		jobs := make(chan VIColorBackfill)
		var wg sync.WaitGroup
		for range concurrency {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for vi := range jobs {
					if limiter.Wait(ctx) != nil {
						return
					}

					start := time.Now()
					imageBytes, mimeType, err := fetchBlobFromPDS(ctx, store, dir, vi.BlobDID, vi.BlobCID)
					if err != nil {
						if isBlobRateLimited(err) {
							slog.Warn("colors backfill: PDS answered 429, cooling down", "blob_cid", vi.BlobCID, "cooldown", rateLimitCooldown)
							select {
							case <-ctx.Done():
								return
							case <-time.After(rateLimitCooldown):
							}
						} else {
							slog.Warn("colors backfill: fetch blob", "vi_id", vi.ID, "blob_cid", vi.BlobCID, "err", err)
						}
						continue
					}
					fetchMS.Add(time.Since(start).Milliseconds())
					fetchedBytes.Add(int64(len(imageBytes)))

					start = time.Now()
					colors, err := inference.Palette(ctx, imageBytes, mimeType)
					if err != nil {
						slog.Warn("colors backfill: extract palette", "vi_id", vi.ID, "blob_cid", vi.BlobCID, "err", err)
						continue
					}
					paletteMS.Add(time.Since(start).Milliseconds())

					if dryRun {
						slog.Info("colors backfill DRY", "vi_id", vi.ID, "blob_cid", vi.BlobCID, "palette", string(colors))
						done.Add(1)
						continue
					}

					if err := store.SetVIColors(ctx, vi.ID, colors); err != nil {
						slog.Warn("colors backfill: set vi colors", "vi_id", vi.ID, "err", err)
						continue
					}
					if err := store.UpdateSaveDominantColorsByCID(ctx, vi.BlobCID, colors); err != nil {
						slog.Warn("colors backfill: update save palettes", "blob_cid", vi.BlobCID, "err", err)
					}
					done.Add(1)
				}
			}()
		}

		batchStart := time.Now()
	feed:
		for _, vi := range batch {
			select {
			case jobs <- vi:
			case <-ctx.Done():
				break feed
			}
		}
		close(jobs)
		wg.Wait()

		processed += int(done.Load())
		if ctx.Err() != nil {
			slog.Info("colors backfill interrupted", "processed", processed)
			return nil
		}

		elapsed := time.Since(batchStart)
		slog.Info("colors backfill batch done",
			"size", len(batch),
			"ok", done.Load(),
			"processed_total", processed,
			"remaining_in_pool", pending-int64(processed),
			"images_per_sec", fmt.Sprintf("%.1f", float64(done.Load())/elapsed.Seconds()),
			"avg_fetch_ms", mean(fetchMS.Load(), done.Load()),
			"avg_palette_ms", mean(paletteMS.Load(), done.Load()),
			"avg_blob_kb", mean(fetchedBytes.Load()/1024, done.Load()),
			// Blob download throughput. If this plateaus while images_per_sec
			// stays flat as --concurrency rises, the downlink is the wall and
			// no amount of workers will help.
			"mbit_per_sec", fmt.Sprintf("%.0f", float64(fetchedBytes.Load())*8/elapsed.Seconds()/1e6),
		)

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
