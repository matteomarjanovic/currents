package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

// runBackfillFeedJunk scores existing visual identities' stored embeddings via
// the inference server's /classify/junk/embeddings endpoint and fills
// visual_identity.junk_score — no blob fetches and no backbone passes, just
// the tiny CPU head over vectors already in the DB. Resumable: scored VIs
// drop out of the pool; the keyset cursor keeps a single run terminating even
// when some rows persistently fail.
//
// Use --dry-run to preview the first batch's scores without writing anything.
func runBackfillFeedJunk(cctx *cli.Context) error {
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

	pending, err := store.CountVIsMissingJunkScore(ctx)
	if err != nil {
		return fmt.Errorf("count unscored: %w", err)
	}
	slog.Info("feed-junk backfill starting",
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
			slog.Info("feed-junk backfill: --limit reached", "processed", processed)
			return nil
		}

		batchLimit := batchSize
		if limit > 0 && processed+batchLimit > limit {
			batchLimit = limit - processed
		}

		batch, err := store.ListVIsMissingJunkScoreBatch(ctx, afterID, batchLimit)
		if err != nil {
			return fmt.Errorf("list batch: %w", err)
		}
		if len(batch) == 0 {
			slog.Info("feed-junk backfill complete", "processed", processed)
			return nil
		}
		afterID = batch[len(batch)-1].ID

		embeddings := make([][]float32, len(batch))
		for i, vi := range batch {
			embeddings[i] = vi.Embedding
		}
		scores, err := inference.ClassifyJunkEmbeddings(ctx, embeddings)
		if err != nil {
			return fmt.Errorf("classify batch: %w", err)
		}
		if len(scores) != len(batch) {
			return fmt.Errorf("classify returned %d scores for %d embeddings", len(scores), len(batch))
		}

		for i, vi := range batch {
			if dryRun {
				slog.Info("feed-junk DRY",
					"vi_id", vi.ID,
					"junk_score", scores[i],
					"would_filter", scores[i] >= feedJunkScoreMax,
				)
				processed++
				continue
			}
			if err := store.SetVIJunkScore(ctx, vi.ID, scores[i]); err != nil {
				slog.Warn("set junk score", "vi_id", vi.ID, "err", err)
				continue
			}
			processed++
		}

		slog.Info("feed-junk batch done", "size", len(batch), "processed_total", processed, "remaining_in_pool", pending-int64(processed))

		if dryRun {
			slog.Info("feed-junk backfill: --dry-run, stopping after first batch")
			return nil
		}

		// Throttle so live TAP enrichment isn't starved on the inference server.
		select {
		case <-ctx.Done():
			slog.Info("feed-junk backfill interrupted", "processed", processed)
			return nil
		case <-time.After(interval):
		}
	}
}
