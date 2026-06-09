package main

// Auto-classification thresholds used by the TAP enrichment pipeline.
// All three classifier axes (NSFW, violence, AI-generated) share these thresholds.
const (
	// ThresholdAutoApply: score at or above this auto-applies a canonical label
	// to all saves of the blob and creates a label_applied review_item for owners.
	ThresholdAutoApply float32 = 0.90

	// ThresholdSuspected: score between this and ThresholdAutoApply sets
	// harm_state='suspected' in the DB and creates an AI review_item. No label
	// is published; blob owners are notified and can confirm or ignore.
	ThresholdSuspected float32 = 0.70
)

// Review queue priorities — higher surfaces first.
const (
	PriorityNormal = 0
	PriorityHigh   = 100
)

// Canonical label values auto-applied by the classifier at ≥ThresholdAutoApply.
// Moderator-applied canonical labels (porn / sexual / nudity / graphic-media) are
// referenced as string literals at call sites since they're driven by reviewer choice.
const (
	LabelNSFW      = "porn"
	LabelViolence  = "graphic-media"
	LabelAIGenerated = "currents-ai-generated"
)

// Moderation event actions written to moderation_event.action.
const (
	ActionLabelAdd    = "label_add"
	ActionLabelNegate = "label_negate"
	ActionAIFlag      = "ai_flag"
	ActionSelfLabel   = "self_label"
	ActionSelfConfirm = "self_confirm" // author confirmed a suspected classification
	ActionSelfDispute = "self_dispute" // author disputed a label or suspected classification
	ActionOwnerIgnore = "owner_ignore" // author dismissed a suspected item (no action)
)

// HarmOutcome is the action implied by a harm-axis score crossing the threshold ladder.
type HarmOutcome int

const (
	HarmNone       HarmOutcome = iota // score below ThresholdSuspected — no action
	HarmSuspected                     // score in [ThresholdSuspected, ThresholdAutoApply) — suspected DB state, owner notified
	HarmAutoApply                     // score >= ThresholdAutoApply — canonical label auto-applied
)

// ClassifyHarmScore returns the outcome implied by a single harm-axis score.
// Shared by live TAP enrichment and the backfill CLI so the ladder is defined once.
func ClassifyHarmScore(score float32) HarmOutcome {
	switch {
	case score >= ThresholdAutoApply:
		return HarmAutoApply
	case score >= ThresholdSuspected:
		return HarmSuspected
	default:
		return HarmNone
	}
}
