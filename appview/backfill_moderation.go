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

// runBackfillModeration scores existing saves' visual-identity embeddings via the
// inference server's /classify/safety/embeddings endpoint, persists scores, and
// issues labels on every save URI sharing each blob CID. Resumable: each blob's
// blob_moderation_state row excludes it from subsequent batches.
//
// Use --dry-run to preview the first batch's decisions without writing anything.
func runBackfillModeration(cctx *cli.Context) error {
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

	signer, err := NewLabelerSigner(cctx.String("labeler-did"), cctx.String("labeler-signing-key"))
	if err != nil {
		return fmt.Errorf("labeler signer: %w", err)
	}
	labeler := NewLabelerIssuer(signer, store)
	if labeler == nil && !dryRun {
		return errors.New("labeler not configured: set LABELER_DID and LABELER_SIGNING_KEY (or use --dry-run)")
	}

	pending, err := store.CountUnclassifiedBlobs(ctx)
	if err != nil {
		return fmt.Errorf("count unclassified: %w", err)
	}
	slog.Info("backfill starting",
		"pending_blobs", pending,
		"batch_size", batchSize,
		"interval", interval,
		"limit_arg", limit,
		"dry_run", dryRun,
	)

	processed := 0
	for {
		if limit > 0 && processed >= limit {
			slog.Info("backfill: --limit reached", "processed", processed)
			return nil
		}

		batchLimit := batchSize
		if limit > 0 && processed+batchLimit > limit {
			batchLimit = limit - processed
		}

		batch, err := store.ListUnclassifiedBlobsBatch(ctx, batchLimit)
		if err != nil {
			return fmt.Errorf("list batch: %w", err)
		}
		if len(batch) == 0 {
			slog.Info("backfill complete", "processed", processed)
			return nil
		}

		embeddings := make([][]float32, len(batch))
		for i, b := range batch {
			embeddings[i] = b.Embedding
		}
		scoresBatch, err := inference.ClassifySafetyEmbeddings(ctx, embeddings)
		if err != nil {
			return fmt.Errorf("classify batch: %w", err)
		}
		if len(scoresBatch) != len(batch) {
			return fmt.Errorf("classify returned %d scores for %d embeddings", len(scoresBatch), len(batch))
		}

		for i, blob := range batch {
			if err := applyBackfillForBlob(ctx, store, labeler, blob, scoresBatch[i], dryRun); err != nil {
				slog.Warn("apply backfill", "blob_cid", blob.BlobCID, "err", err)
				continue
			}
			processed++
		}

		slog.Info("backfill batch done", "size", len(batch), "processed_total", processed, "remaining_in_pool", pending-int64(processed))

		if dryRun {
			slog.Info("backfill: --dry-run, stopping after first batch")
			return nil
		}

		// Throttle so live TAP enrichment isn't starved on the inference server.
		select {
		case <-ctx.Done():
			slog.Info("backfill interrupted", "processed", processed)
			return nil
		case <-time.After(interval):
		}
	}
}

// applyBackfillForBlob runs the same threshold ladder as TAP, but:
//   - issues labels on EVERY save URI sharing the blob (live TAP only labels
//     the source URI and relies on per-resave materialization)
//   - attributes review_items to the earliest (sample) URI for the blob.
//
// In dry-run mode, decisions are logged and no writes happen.
func applyBackfillForBlob(
	ctx context.Context,
	store *PgStore,
	labeler *LabelerIssuer,
	blob UnclassifiedBlob,
	scores SafetyScores,
	dryRun bool,
) error {
	if dryRun {
		slog.Info("backfill DRY",
			"blob_cid", blob.BlobCID,
			"sample_uri", blob.SampleURI,
			"nsfw", scores.NSFW, "violence", scores.Violence, "ai_gen", scores.AIGenerated,
			"nsfw_action", ClassifyHarmScore(scores.NSFW),
			"violence_action", ClassifyHarmScore(scores.Violence),
			"ai_action", ClassifyHarmScore(scores.AIGenerated),
		)
		return nil
	}

	if err := store.UpsertBlobModerationState(ctx, blob.BlobCID, scores); err != nil {
		return fmt.Errorf("persist scores: %w", err)
	}

	// Pre-fetch the set of save URIs once; reuse for every label issuance below.
	uris, err := store.ListSaveURIsByBlobCID(ctx, blob.BlobCID)
	if err != nil {
		return fmt.Errorf("list save uris: %w", err)
	}

	issueOnAll := func(val string) {
		for _, u := range uris {
			if _, err := labeler.IssueLabel(ctx, IssueLabelParams{
				Actor:   "auto",
				Action:  ActionAIFlag,
				URI:     u,
				BlobCID: blob.BlobCID,
				Val:     val,
			}); err != nil {
				slog.Warn("issue label", "uri", u, "val", val, "err", err)
			}
		}
	}

	for _, axis := range []struct {
		name    string
		score   float32
		autoVal string
	}{
		{"nsfw", scores.NSFW, LabelNSFW},
		{"violence", scores.Violence, LabelViolence},
		{"ai-generated", scores.AIGenerated, LabelAIGenerated},
	} {
		score := axis.score
		switch ClassifyHarmScore(score) {
		case HarmAutoApply:
			issueOnAll(axis.autoVal)
			if _, err := store.UpsertReviewItem(ctx, ReviewItemRow{
				Source:     "label_applied",
				SubjectURI: blob.SampleURI,
				BlobCID:    blob.BlobCID,
				Category:   axis.name,
				Score:      &score,
				Priority:   PriorityNormal,
			}); err != nil {
				slog.Warn("upsert label_applied review item", "blob_cid", blob.BlobCID, "axis", axis.name, "err", err)
			}
		case HarmSuspected:
			if err := store.SetHarmState(ctx, blob.BlobCID, HarmStateSuspected, "auto", ""); err != nil {
				slog.Warn("set harm state suspected", "blob_cid", blob.BlobCID, "err", err)
			}
			if _, err := store.UpsertReviewItem(ctx, ReviewItemRow{
				Source:     "ai",
				SubjectURI: blob.SampleURI,
				BlobCID:    blob.BlobCID,
				Category:   axis.name,
				Score:      &score,
				Priority:   PriorityNormal,
			}); err != nil {
				slog.Warn("upsert review item", "blob_cid", blob.BlobCID, "axis", axis.name, "err", err)
			}
		}
	}

	return nil
}

