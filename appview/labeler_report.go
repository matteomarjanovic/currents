package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/jackc/pgx/v5"
)

// XRPCCreateReport implements com.atproto.moderation.createReport.
// The labeler accepts reports about is.currents.feed.save records (strongRef subject only).
// account-level reports (admin.defs#repoRef) are out of scope for v1.
func (s *Server) XRPCCreateReport(w http.ResponseWriter, r *http.Request) {
	reporterDID, err := s.optionalAuth(r)
	if err != nil {
		http.Error(w, "auth error", http.StatusUnauthorized)
		return
	}
	if reporterDID == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	var input comatproto.ModerationCreateReport_Input
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if input.ReasonType == nil || *input.ReasonType == "" {
		http.Error(w, "reasonType required", http.StatusBadRequest)
		return
	}
	if input.Subject == nil || input.Subject.RepoStrongRef == nil {
		http.Error(w, "subject must be com.atproto.repo.strongRef (record-level reports only)", http.StatusBadRequest)
		return
	}
	uri := input.Subject.RepoStrongRef.Uri
	cidStr := input.Subject.RepoStrongRef.Cid
	if uri == "" {
		http.Error(w, "subject.uri required", http.StatusBadRequest)
		return
	}
	var reason string
	if input.Reason != nil {
		reason = *input.Reason
	}

	reportID, err := s.Store.InsertReport(r.Context(), reporterDID.String(), uri, cidStr, *input.ReasonType, reason)
	if err != nil {
		slog.Error("InsertReport", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	// Best-effort: enqueue a review item if we know the blob for this save.
	// If the save isn't in our DB (reported from a Bluesky client about a
	// not-yet-indexed save), still record the report — review_item entry can be
	// added later via a reconciliation pass.
	if blobCID, err := lookupSaveBlobCID(r.Context(), s.Store, uri); err == nil && blobCID != "" {
		category := reasonTypeToCategory(*input.ReasonType)
		if _, err := s.Store.UpsertReviewItem(r.Context(), ReviewItemRow{
			Source:     "report",
			SourceRef:  &reportID,
			SubjectURI: uri,
			SubjectCID: cidStr,
			BlobCID:    blobCID,
			Category:   category,
			Priority:   PriorityNormal,
		}); err != nil {
			slog.Warn("UpsertReviewItem from report", "report_id", reportID, "err", err)
		}
	}

	out := &comatproto.ModerationCreateReport_Output{
		Id:         reportID,
		ReasonType: input.ReasonType,
		Reason:     input.Reason,
		ReportedBy: reporterDID.String(),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		Subject: &comatproto.ModerationCreateReport_Output_Subject{
			RepoStrongRef: input.Subject.RepoStrongRef,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		slog.Warn("encode createReport output", "err", err)
	}
}

// reasonTypeToCategory maps report reason strings to review_item.category.
func reasonTypeToCategory(reasonType string) string {
	switch reasonType {
	case "sexual":
		return "nsfw"
	case "violence":
		return "violence"
	case "ai-generated":
		return "ai-generated"
	default:
		return "other"
	}
}

// lookupSaveBlobCID returns the pds_blob_cid for a save URI, or "" if the save
// isn't indexed yet (no error — that's an expected state for reports about
// records the appview hasn't seen).
func lookupSaveBlobCID(ctx context.Context, store *PgStore, uri string) (string, error) {
	var blobCID string
	err := store.pool.QueryRow(ctx, `SELECT pds_blob_cid FROM save WHERE uri = $1`, uri).Scan(&blobCID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return blobCID, nil
}
