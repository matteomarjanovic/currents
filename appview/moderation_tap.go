package main

import (
	"context"
	"fmt"
	"log/slog"
)

// applyModerationAfterSaveUpsert runs after a save row is persisted. It (1)
// materializes any pre-existing blob-level labels onto the new save URI so a
// resave or re-upload of known-labeled bytes inherits them, and (2) issues the
// creator's self-labels blob-keyed so the warning propagates to every copy of
// those bytes — past copies via retroactive fan-out, future copies via the
// materialization in (1). A content warning is a fact about the bytes, not about
// one record; we never forge a self-label into another user's repo, the labeler
// issues the propagated copies.
//
// Errors are logged and swallowed — moderation is best-effort and must never
// block the save indexing pipeline.
func applyModerationAfterSaveUpsert(ctx context.Context, handler *TapHandler, saveURI, authorDID, blobCID string, self *selfLabels) {
	if handler.Labeler == nil {
		return
	}

	// Labels already active for this blob (canonical harm labels, self-labels
	// declared on other copies). Materialize each onto the new URI, and keep the
	// set so a self-label below only fans out the first time a val appears.
	activeBlobVals := map[string]struct{}{}
	if blobCID != "" {
		vals, err := handler.Store.GetActiveLabelsByBlobCID(ctx, blobCID)
		if err != nil {
			slog.Warn("get active blob labels", "blob_cid", blobCID, "err", err)
		}
		for _, v := range vals {
			activeBlobVals[v] = struct{}{}
			if _, err := handler.Labeler.IssueLabel(ctx, IssueLabelParams{
				Actor:   "auto",
				Action:  ActionLabelAdd,
				URI:     saveURI,
				BlobCID: blobCID,
				Val:     v,
			}); err != nil {
				slog.Warn("materialize label", "uri", saveURI, "val", v, "err", err)
			}
		}
	}

	if self == nil {
		return
	}
	for _, lv := range self.Values {
		if lv.Val == "" {
			continue
		}
		// The creator's voluntary declaration about their own copy, attributed to
		// the author and blob-keyed so it joins the blob's active-label set.
		if _, err := handler.Labeler.IssueLabel(ctx, IssueLabelParams{
			Actor:   authorDID,
			Action:  ActionSelfLabel,
			URI:     saveURI,
			BlobCID: blobCID,
			Val:     lv.Val,
		}); err != nil {
			slog.Warn("issue self-label", "uri", saveURI, "val", lv.Val, "err", err)
			continue
		}
		// Retroactively warn existing copies of the same bytes — once per (blob,
		// val), so repeat saves of the same blob don't re-fan-out.
		if blobCID == "" {
			continue
		}
		if _, already := activeBlobVals[lv.Val]; already {
			continue
		}
		activeBlobVals[lv.Val] = struct{}{}
		propagateLabelToBlobSiblings(ctx, handler, blobCID, saveURI, lv.Val)
	}
}

// propagateLabelToBlobSiblings issues val (labeler-attributed) on every existing
// save URI sharing blobCID except sourceURI, so a creator's content warning
// applies to copies already in the index. Future copies inherit via the
// materialization step in applyModerationAfterSaveUpsert. Best-effort.
func propagateLabelToBlobSiblings(ctx context.Context, handler *TapHandler, blobCID, sourceURI, val string) {
	uris, err := handler.Store.ListSaveURIsByBlobCID(ctx, blobCID)
	if err != nil {
		slog.Warn("list sibling uris for self-label fan-out", "blob_cid", blobCID, "val", val, "err", err)
		return
	}
	for _, u := range uris {
		if u == sourceURI {
			continue
		}
		if _, err := handler.Labeler.IssueLabel(ctx, IssueLabelParams{
			Actor:   "auto",
			Action:  ActionLabelAdd,
			URI:     u,
			BlobCID: blobCID,
			Val:     val,
		}); err != nil {
			slog.Warn("propagate self-label to sibling", "uri", u, "val", val, "err", err)
		}
	}
}

// classifierAxis pairs a classifier score with the canonical label to auto-apply
// and the review_item category name. All three axes share identical flow logic.
type classifierAxis struct {
	name    string  // review_item.category: "nsfw", "violence", "ai-generated"
	score   float32
	autoVal string  // canonical label value to auto-apply at ≥ThresholdAutoApply
}

// processSafetyScores persists per-blob safety scores and drives auto-classification.
// All three axes (NSFW, violence, AI-generated) follow the same ladder:
//   - ≥ThresholdAutoApply: canonical label auto-applied to all saves of blob,
//     label_applied review_item created for owner dispute notification.
//   - [ThresholdSuspected, ThresholdAutoApply): harm_state='suspected' set in DB,
//     ai review_item created for owner confirmation and moderator queue.
//   - <ThresholdSuspected: no action.
//
// Errors on individual axes are logged but don't abort the function — partial success
// is preferable to losing all moderation signal on this blob.
func processSafetyScores(ctx context.Context, handler *TapHandler, source BlobSourceCandidate, blobCID string, scores SafetyScores) error {
	if err := handler.Store.UpsertBlobModerationState(ctx, blobCID, scores); err != nil {
		return fmt.Errorf("persist safety scores: %w", err)
	}

	axes := []classifierAxis{
		{"nsfw", scores.NSFW, LabelNSFW},
		{"violence", scores.Violence, LabelViolence},
		{"ai-generated", scores.AIGenerated, LabelAIGenerated},
	}

	for _, a := range axes {
		score := a.score
		switch ClassifyHarmScore(score) {
		case HarmAutoApply:
			// Fan out canonical label to every save URI sharing this blob.
			if handler.Labeler != nil {
				uris, err := handler.Store.ListSaveURIsByBlobCID(ctx, blobCID)
				if err != nil {
					slog.Warn("list sibling uris for auto-label", "blob_cid", blobCID, "axis", a.name, "err", err)
				}
				for _, u := range uris {
					if _, err := handler.Labeler.IssueLabel(ctx, IssueLabelParams{
						Actor:   "auto",
						Action:  ActionAIFlag,
						URI:     u,
						BlobCID: blobCID,
						Val:     a.autoVal,
					}); err != nil {
						slog.Warn("issue auto label", "blob_cid", blobCID, "uri", u, "axis", a.name, "err", err)
					}
				}
			}
			// Notify all blob owners that a label was applied; they can dispute.
			if _, err := handler.Store.UpsertReviewItem(ctx, ReviewItemRow{
				Source:     "label_applied",
				SubjectURI: source.URI,
				BlobCID:    blobCID,
				Category:   a.name,
				LabelVal:   a.autoVal,
				Score:      &score,
				Priority:   PriorityNormal,
			}); err != nil {
				slog.Warn("upsert label_applied review item", "blob_cid", blobCID, "axis", a.name, "err", err)
			}

		case HarmSuspected:
			// Record suspected state in DB only — no label published.
			if err := handler.Store.SetHarmState(ctx, blobCID, HarmStateSuspected, "auto", ""); err != nil {
				slog.Warn("set harm state suspected", "blob_cid", blobCID, "axis", a.name, "err", err)
			}
			// Owner notification and moderator queue entry.
			if _, err := handler.Store.UpsertReviewItem(ctx, ReviewItemRow{
				Source:     "ai",
				SubjectURI: source.URI,
				BlobCID:    blobCID,
				Category:   a.name,
				Score:      &score,
				Priority:   PriorityNormal,
			}); err != nil {
				slog.Warn("upsert review item", "blob_cid", blobCID, "axis", a.name, "err", err)
			}
		}
	}

	return nil
}
