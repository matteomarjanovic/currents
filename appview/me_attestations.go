package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

// disputePriorityBump is added to review_item.priority when an author disputes.
// Keeps the queue ordering meaningful — disputed items surface above
// untouched normal-priority items but still below true high-priority items.
const disputePriorityBump = 50

// pendingCountCap caps the dialog payload to keep it manageable.
const pendingCountCap = 50

// APIMeListAttestations returns the session DID's pending review items for blobs
// they have saved. Drives the Notifications dialog and the bell-icon badge count.
func (s *Server) APIMeListAttestations(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	w.Header().Set("Content-Type", "application/json")
	if did == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := s.Store.ListPendingAttestationsByAuthor(r.Context(), did.String(), pendingCountCap)
	if err != nil {
		slog.Error("ListPendingAttestationsByAuthor", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	views := make([]reviewItemView, len(items))
	for i, it := range items {
		views[i] = toReviewItemView(it, s.CDNBaseURL)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"items": views})
}

// canonicalValForCategory returns the canonical label to apply when an owner
// confirms a suspected classification. All three axes map to a single value
// (no choice — the classifier doesn't distinguish within an axis yet).
func canonicalValForCategory(category string) (string, bool) {
	switch category {
	case "nsfw":
		return LabelNSFW, true
	case "violence":
		return LabelViolence, true
	case "ai-generated":
		return LabelAIGenerated, true
	}
	return "", false
}

// requireAttestationOwnership returns the validated session DID + the loaded
// review_item, or writes an error response and returns nil/nil. Centralizes the
// 401/403/404/409 ladder all three /api/me/attestations endpoints share.
func (s *Server) requireAttestationOwnership(w http.ResponseWriter, r *http.Request) (string, *ReviewItemRow) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", nil
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return "", nil
	}
	item, err := s.Store.GetReviewItem(r.Context(), id)
	if err != nil {
		slog.Error("GetReviewItem", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return "", nil
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return "", nil
	}
	if item.Status != "pending" {
		http.Error(w, "already resolved", http.StatusConflict)
		return "", nil
	}
	// Ownership check — the session DID must own a save of this blob.
	uris, err := s.Store.ListSaveURIsByBlobCID(r.Context(), item.BlobCID)
	if err != nil {
		slog.Error("ListSaveURIsByBlobCID", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return "", nil
	}
	didStr := did.String()
	ownsBlob := false
	for _, u := range uris {
		if authorDIDFromURI(u) == didStr {
			ownsBlob = true
			break
		}
	}
	if !ownsBlob {
		// Fallback: check subject_uri directly (for items without blob_cid).
		author, err := s.Store.GetSaveAuthor(r.Context(), item.SubjectURI)
		if err != nil {
			slog.Error("GetSaveAuthor", "err", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return "", nil
		}
		if author == "" || author != didStr {
			http.Error(w, "forbidden", http.StatusForbidden)
			return "", nil
		}
	}
	return didStr, item
}

// APIMeAttestationConfirm: owner ratifies a suspected (source='ai') auto-flag.
// Issues the canonical label on every URI sharing the blob and resolves the item.
// Only valid for source='ai' items (suspected state — no label yet).
func (s *Server) APIMeAttestationConfirm(w http.ResponseWriter, r *http.Request) {
	actorDID, item := s.requireAttestationOwnership(w, r)
	if item == nil {
		return
	}
	if item.Source != "ai" {
		http.Error(w, "confirm only valid for suspected (source=ai) items", http.StatusBadRequest)
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

	canonicalVal, ok := canonicalValForCategory(item.Category)
	if !ok {
		http.Error(w, fmt.Sprintf("confirm not supported for category %q", item.Category), http.StatusBadRequest)
		return
	}

	uris, err := s.Store.ListSaveURIsByBlobCID(r.Context(), item.BlobCID)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	// Apply canonical label on every URI sharing the blob.
	s.issueLabelOnAllSiblings(r.Context(), uris, item.BlobCID, canonicalVal, actorDID, ActionSelfConfirm, false)

	// Clear suspected state — label is now the authoritative record.
	if err := s.Store.SetHarmState(r.Context(), item.BlobCID, HarmStateClean, actorDID, ""); err != nil {
		slog.Warn("clear harm state after confirm", "blob_cid", item.BlobCID, "err", err)
	}

	if err := s.Store.ResolveReviewItem(r.Context(), item.ID, "resolved", actorDID); err != nil {
		slog.Warn("resolve review item", "id", item.ID, "err", err)
	}

	// Create a label_applied item so other blob owners can dispute the label.
	if _, err := s.Store.UpsertReviewItem(r.Context(), ReviewItemRow{
		Source:     "label_applied",
		SubjectURI: item.SubjectURI,
		BlobCID:    item.BlobCID,
		Category:   item.Category,
		LabelVal:   canonicalVal,
		Score:      item.Score,
		Priority:   PriorityNormal,
	}); err != nil {
		slog.Warn("upsert label_applied review item after confirm", "blob_cid", item.BlobCID, "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "applied_to": len(uris)})
}

// APIMeAttestationIgnore: owner dismisses a suspected (source='ai') item without
// action. If no pending items remain for this blob, harm_state is cleared to 'clean'.
func (s *Server) APIMeAttestationIgnore(w http.ResponseWriter, r *http.Request) {
	actorDID, item := s.requireAttestationOwnership(w, r)
	if item == nil {
		return
	}

	if err := s.Store.ResolveReviewItem(r.Context(), item.ID, "dismissed", actorDID); err != nil {
		slog.Error("ResolveReviewItem", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	payload, _ := json.Marshal(map[string]any{"category": item.Category})
	_ = s.Store.InsertModerationEvent(r.Context(), actorDID, ActionOwnerIgnore, item.SubjectURI, item.SubjectCID, item.BlobCID, payload)

	// If no other pending items remain for this blob, clear the suspected state.
	if item.BlobCID != "" {
		remaining, err := s.Store.CountPendingReviewItemsByBlobCID(r.Context(), item.BlobCID)
		if err != nil {
			slog.Warn("count pending items", "blob_cid", item.BlobCID, "err", err)
		} else if remaining == 0 {
			if err := s.Store.SetHarmState(r.Context(), item.BlobCID, HarmStateClean, actorDID, ""); err != nil {
				slog.Warn("clear harm state after ignore", "blob_cid", item.BlobCID, "err", err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// APIMeAttestationDispute: owner disputes a review item (either a suspected
// classification or a label that was applied). Bumps priority so a moderator
// sees it sooner.
func (s *Server) APIMeAttestationDispute(w http.ResponseWriter, r *http.Request) {
	actorDID, item := s.requireAttestationOwnership(w, r)
	if item == nil {
		return
	}

	if err := s.Store.MarkReviewItemDisputed(r.Context(), item.ID, disputePriorityBump); err != nil {
		slog.Error("MarkReviewItemDisputed", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	payload, _ := json.Marshal(map[string]any{"category": item.Category})
	_ = s.Store.InsertModerationEvent(r.Context(), actorDID, ActionSelfDispute, item.SubjectURI, item.SubjectCID, item.BlobCID, payload)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
