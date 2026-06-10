package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// requireModerator gates a handler on the session DID having an active row in
// the moderator table. 401 if not authenticated, 403 if authenticated but not a
// moderator.
func (s *Server) requireModerator(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		did, _, _ := s.currentSessionDID(r)
		if did == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		role, err := s.Store.IsModerator(r.Context(), did.String())
		if err != nil {
			slog.Error("IsModerator", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		if role == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// APIMeRole reports whether the session DID has a moderator role assigned.
// Always 200; clients read role==null as "not a moderator" to gate the admin UI.
func (s *Server) APIMeRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		json.NewEncoder(w).Encode(map[string]any{"role": nil})
		return
	}
	role, err := s.Store.IsModerator(r.Context(), did.String())
	if err != nil {
		slog.Error("IsModerator", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	resp := map[string]any{"role": nil}
	if role != "" {
		resp["role"] = role
	}
	json.NewEncoder(w).Encode(resp)
}

// authorDIDFromURI extracts the author DID from an at:// URI of the form
// at://{did}/{collection}/{rkey}. Returns "" if the URI doesn't parse.
func authorDIDFromURI(uri string) string {
	const prefix = "at://"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	rest := uri[len(prefix):]
	if i := strings.Index(rest, "/"); i > 0 {
		return rest[:i]
	}
	return ""
}

type reviewItemView struct {
	ID         int64    `json:"id"`
	Source     string   `json:"source"`
	SubjectURI string   `json:"subjectUri"`
	BlobCID    string   `json:"blobCid,omitempty"`
	Category   string   `json:"category,omitempty"`
	Score      *float32 `json:"score,omitempty"`
	Priority   int      `json:"priority"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"createdAt"`
	PreviewURL string `json:"previewUrl,omitempty"`
	// LabelVal is the specific atproto label val for label_applied items
	// (e.g. "porn", "nudity", "sexual", "graphic-media", "currents-ai-generated").
	LabelVal string `json:"labelVal,omitempty"`

	// Populated when source='report' and the report row was found.
	// Surfaces the original report context so moderators see the actual
	// reasonType + free-text reason instead of just `category='other'`.
	ReportReasonType  string `json:"reportReasonType,omitempty"`
	ReportReasonText  string `json:"reportReasonText,omitempty"`
	ReportReporterDID string `json:"reportReporterDid,omitempty"`

	// Author self-attestation: when true, the save's author disputed the
	// auto-flag. The suspected label remains; the moderator's queue gives
	// these items higher priority (bumped on dispute) and a visible badge.
	Disputed   bool   `json:"disputed,omitempty"`
	DisputedAt string `json:"disputedAt,omitempty"`
}

func toReviewItemView(it ReviewItemRow, cdnBaseURL string) reviewItemView {
	v := reviewItemView{
		ID:                it.ID,
		Source:            it.Source,
		SubjectURI:        it.SubjectURI,
		BlobCID:           it.BlobCID,
		Category:          it.Category,
		LabelVal:          it.LabelVal,
		Score:             it.Score,
		Priority:          it.Priority,
		Status:            it.Status,
		CreatedAt:         it.CreatedAt.UTC().Format(time.RFC3339),
		ReportReasonType:  it.ReportReasonType,
		ReportReasonText:  it.ReportReasonText,
		ReportReporterDID: it.ReportReporterDID,
		Disputed:          it.Disputed,
	}
	if it.DisputedAt != nil {
		v.DisputedAt = it.DisputedAt.UTC().Format(time.RFC3339)
	}
	if did := authorDIDFromURI(it.SubjectURI); did != "" && it.BlobCID != "" {
		v.PreviewURL = cdnBaseURL + "/img/" + did + "/" + it.BlobCID
	}
	return v
}

// APIAdminQueue lists pending review items.
// Query params:
//   ?source=ai|report|label_applied      (optional; empty = all)
//   ?category=nsfw|violence|other        (optional; empty = all)
//   ?order=priority|oldest|newest        (optional; default = priority)
//   ?limit=50                            (default 50, max 200)
//   ?offset=0                            (default 0)
func (s *Server) APIAdminQueue(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")
	order := r.URL.Query().Get("order")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	items, err := s.Store.ListPendingReviewItems(r.Context(), source, category, order, limit, offset)
	if err != nil {
		slog.Error("ListPendingReviewItems", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	views := make([]reviewItemView, len(items))
	for i, it := range items {
		views[i] = toReviewItemView(it, s.CDNBaseURL)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"items": views})
}

// APIAdminQueueDetail returns a single review item with all the context the
// reviewer needs: scores, ai-gen flag, sibling save URIs (same blob), active
// labels, and the audit-log entries for this blob.
func (s *Server) APIAdminQueueDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	item, err := s.Store.GetReviewItem(r.Context(), id)
	if err != nil {
		slog.Error("GetReviewItem", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	resp := map[string]any{
		"item": toReviewItemView(*item, s.CDNBaseURL),
	}

	if item.BlobCID != "" {
		blobState, err := s.Store.GetBlobModerationState(r.Context(), item.BlobCID)
		if err != nil {
			slog.Error("GetBlobModerationState", "err", err)
		} else if blobState != nil {
			bs := map[string]any{
				"harmState":   blobState.HarmState,
				"aiGenerated": blobState.AIGenerated,
				"decidedBy":   blobState.DecidedBy,
			}
			if blobState.SafetyScores != nil {
				bs["safetyScores"] = blobState.SafetyScores
			}
			if blobState.DecidedAt != nil {
				bs["decidedAt"] = blobState.DecidedAt.UTC().Format(time.RFC3339)
			}
			if blobState.Notes != "" {
				bs["notes"] = blobState.Notes
			}
			resp["blobState"] = bs
		}

		uris, err := s.Store.ListSaveURIsByBlobCID(r.Context(), item.BlobCID)
		if err != nil {
			slog.Error("ListSaveURIsByBlobCID", "err", err)
		} else {
			saves := make([]map[string]any, 0, len(uris))
			for _, u := range uris {
				saves = append(saves, map[string]any{
					"uri":       u,
					"authorDid": authorDIDFromURI(u),
				})
			}
			resp["saves"] = saves
		}

		labels, err := s.Store.GetActiveLabelsByBlobCID(r.Context(), item.BlobCID)
		if err != nil {
			slog.Error("GetActiveLabelsByBlobCID", "err", err)
		} else {
			resp["activeLabels"] = labels
		}

		events, err := s.Store.ListModerationEventsByBlobCID(r.Context(), item.BlobCID, 100)
		if err != nil {
			slog.Error("ListModerationEventsByBlobCID", "err", err)
		} else {
			eventViews := make([]map[string]any, 0, len(events))
			for _, e := range events {
				eventViews = append(eventViews, toModerationEventView(e))
			}
			resp["events"] = eventViews
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// adminAction shared payload for all action endpoints.
type adminActionRequest struct {
	Val   string `json:"val"`             // for confirm: canonical val (porn|sexual|nudity|graphic-media)
	Notes string `json:"notes,omitempty"` // optional reviewer note
}

// issueLabelOnAllSiblings issues a single label (or negation) on every save URI
// sharing the blob. Errors are logged but don't abort — partial issuance is
// preferred over leaving the moderator in limbo with no DB record of their
// action. Used by confirm + takedown + apply-label.
func (s *Server) issueLabelOnAllSiblings(ctx context.Context, uris []string, blobCID, val, actorDID, action string, neg bool) {
	for _, u := range uris {
		if _, err := s.Labeler.IssueLabel(ctx, IssueLabelParams{
			Actor:   actorDID,
			Action:  action,
			URI:     u,
			BlobCID: blobCID,
			Val:     val,
			Neg:     neg,
		}); err != nil {
			slog.Warn("issue label", "uri", u, "val", val, "neg", neg, "err", err)
		}
	}
}

// createLabelAppliedItem creates a label_applied review_item so all blob owners
// can dispute a canonical label that was just applied. Idempotent — if a pending
// item for the same blob+category already exists, UpsertReviewItem is a no-op.
// val is the specific atproto label val applied (e.g. "porn", "nudity", "sexual").
func (s *Server) createLabelAppliedItem(ctx context.Context, item *ReviewItemRow, val string) {
	if item.BlobCID == "" {
		return
	}
	if _, err := s.Store.UpsertReviewItem(ctx, ReviewItemRow{
		Source:     "label_applied",
		SubjectURI: item.SubjectURI,
		BlobCID:    item.BlobCID,
		Category:   item.Category,
		LabelVal:   val,
		Score:      item.Score,
		Priority:   PriorityNormal,
	}); err != nil {
		slog.Warn("upsert label_applied review item", "blob_cid", item.BlobCID, "err", err)
	}
}

// toModerationEventView projects a ModerationEventRow into the shape returned
// by the queue-detail and /admin/me endpoints. Keeping one projector keeps the
// two surfaces consistent (the /admin/me page's Negate button reads the same
// `subjectUri` / `blobCid` / `payload.val` fields the queue-detail audit log uses).
func toModerationEventView(e ModerationEventRow) map[string]any {
	v := map[string]any{
		"id":        e.ID,
		"actor":     e.ActorDID,
		"action":    e.Action,
		"createdAt": e.CreatedAt.UTC().Format(time.RFC3339),
	}
	if e.SubjectURI != "" {
		v["subjectUri"] = e.SubjectURI
	}
	if e.BlobCID != "" {
		v["blobCid"] = e.BlobCID
	}
	if len(e.Payload) > 0 {
		var payload any
		if err := json.Unmarshal(e.Payload, &payload); err == nil {
			v["payload"] = payload
		}
	}
	return v
}

// APIAdminMyEvents returns the current moderator's recent moderation_event rows,
// newest first. Powers the /admin/me page (own action history + per-row Negate).
func (s *Server) APIAdminMyEvents(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	// requireModerator already ensured did != nil
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	events, err := s.Store.ListModerationEventsByActor(r.Context(), did.String(), limit, offset)
	if err != nil {
		slog.Error("ListModerationEventsByActor", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	views := make([]map[string]any, 0, len(events))
	for _, e := range events {
		views = append(views, toModerationEventView(e))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"events": views})
}

// applyLabelAllowedVals is the set of label values an admin may attach via the
// generic apply-label endpoint. Excludes `!hide` (use takedown for that).
var applyLabelAllowedVals = map[string]bool{
	"porn":          true,
	"sexual":        true,
	"nudity":        true,
	"graphic-media": true,
	LabelAIGenerated: true,
}

// APIAdminQueueApplyLabel attaches an arbitrary canonical label to a review
// item, without the category-bound restriction confirm enforces. The label is
// issued on every URI sharing the blob and the item is resolved.
//
// Use this when:
//   - the item came from a report (category='other'), so confirm can't run
//   - the moderator disagrees with the auto-flagged category and wants a
//     different canonical label
func (s *Server) APIAdminQueueApplyLabel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body adminActionRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	if !applyLabelAllowedVals[body.Val] {
		http.Error(w, fmt.Sprintf("val %q not allowed", body.Val), http.StatusBadRequest)
		return
	}

	item, err := s.Store.GetReviewItem(r.Context(), id)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if item.Status != "pending" {
		http.Error(w, "already resolved", http.StatusConflict)
		return
	}
	if item.BlobCID == "" {
		http.Error(w, "item has no blob to apply labels to", http.StatusBadRequest)
		return
	}
	if s.Labeler == nil {
		http.Error(w, "labeler not configured", http.StatusServiceUnavailable)
		return
	}

	did, _, _ := s.currentSessionDID(r)
	actorDID := did.String()

	uris, err := s.Store.ListSaveURIsByBlobCID(r.Context(), item.BlobCID)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	s.issueLabelOnAllSiblings(r.Context(), uris, item.BlobCID, body.Val, actorDID, ActionLabelAdd, false)

	// Clear suspected state if it was set.
	if err := s.Store.SetHarmState(r.Context(), item.BlobCID, HarmStateClean, actorDID, ""); err != nil {
		slog.Warn("clear harm state after apply-label", "blob_cid", item.BlobCID, "err", err)
	}

	if err := s.Store.ResolveReviewItem(r.Context(), id, "resolved", actorDID); err != nil {
		slog.Warn("resolve review item", "id", id, "err", err)
	}

	// Notify blob owners so they can dispute if they disagree.
	s.createLabelAppliedItem(r.Context(), item, body.Val)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "applied_to": len(uris)})
}

// APIAdminQueueTakedown: reviewer hides the content site-wide. Sets harm_state=blocked
// (read queries already filter on this) AND issues !hide labels for any client that
// honors our labeler. Negates the suspected label too.
func (s *Server) APIAdminQueueTakedown(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body adminActionRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	item, err := s.Store.GetReviewItem(r.Context(), id)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if item.Status != "pending" {
		http.Error(w, "already resolved", http.StatusConflict)
		return
	}
	if item.BlobCID == "" {
		http.Error(w, "item has no blob to take down", http.StatusBadRequest)
		return
	}
	if s.Labeler == nil {
		http.Error(w, "labeler not configured", http.StatusServiceUnavailable)
		return
	}

	did, _, _ := s.currentSessionDID(r)
	actorDID := did.String()

	if err := s.Store.SetHarmState(r.Context(), item.BlobCID, HarmStateBlocked, actorDID, body.Notes); err != nil {
		slog.Error("SetHarmState", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	uris, err := s.Store.ListSaveURIsByBlobCID(r.Context(), item.BlobCID)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	s.issueLabelOnAllSiblings(r.Context(), uris, item.BlobCID, "!hide", actorDID, ActionLabelAdd, false)

	if err := s.Store.ResolveReviewItem(r.Context(), id, "resolved", actorDID); err != nil {
		slog.Warn("resolve review item", "id", id, "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "applied_to": len(uris)})
}

// APIAdminQueueDismiss: reviewer judges the flag was a false positive.
// Clears harm_state to 'clean' and marks the item dismissed. Safety scores
// stay on record but no labels propagate.
func (s *Server) APIAdminQueueDismiss(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	item, err := s.Store.GetReviewItem(r.Context(), id)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if item.Status != "pending" {
		http.Error(w, "already resolved", http.StatusConflict)
		return
	}

	did, _, _ := s.currentSessionDID(r)
	actorDID := did.String()

	// Clear suspected state — the moderator judged this a false positive.
	if item.BlobCID != "" {
		if err := s.Store.SetHarmState(r.Context(), item.BlobCID, HarmStateClean, actorDID, ""); err != nil {
			slog.Warn("clear harm state after dismiss", "blob_cid", item.BlobCID, "err", err)
		}
	}

	if err := s.Store.ResolveReviewItem(r.Context(), id, "dismissed", actorDID); err != nil {
		slog.Warn("resolve review item", "id", id, "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// APIAdminNegateLabel: generic label-removal endpoint. Used by the ai-generated
// audit surface to retract a false positive that bypassed human review.
// Body: {uri, val, blobCid?, notes?}
//
// When blobCid is supplied the negation fans out to every save URI sharing the
// blob, mirroring how apply/confirm fan the label out in the first place — a
// single negate on one URI would otherwise leave resaves labeled.
func (s *Server) APIAdminNegateLabel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URI     string `json:"uri"`
		Val     string `json:"val"`
		BlobCID string `json:"blobCid,omitempty"`
		Notes   string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.URI == "" || body.Val == "" {
		http.Error(w, "uri and val required", http.StatusBadRequest)
		return
	}
	if s.Labeler == nil {
		http.Error(w, "labeler not configured", http.StatusServiceUnavailable)
		return
	}
	did, _, _ := s.currentSessionDID(r)

	uris := []string{body.URI}
	if body.BlobCID != "" {
		siblings, err := s.Store.ListSaveURIsByBlobCID(r.Context(), body.BlobCID)
		if err != nil {
			slog.Error("list siblings for negate", "blob_cid", body.BlobCID, "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		if len(siblings) > 0 {
			uris = siblings
		}
	}

	s.issueLabelOnAllSiblings(r.Context(), uris, body.BlobCID, body.Val, did.String(), ActionLabelNegate, true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
