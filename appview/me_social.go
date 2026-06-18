package main

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// APIMeSocial returns the signed-in user's Activity feed: who followed them,
// newest first, with follow-back state and the count of followers since they
// last marked the tab seen. Offset-based cursor, like searchActors.
func (s *Server) APIMeSocial(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	w.Header().Set("Content-Type", "application/json")
	if did == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}
	offset := 0
	if c := r.URL.Query().Get("cursor"); c != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(c); err == nil {
			if n, err := strconv.Atoi(string(raw)); err == nil && n > 0 {
				offset = n
			}
		}
	}

	seenAt, err := s.Store.GetSocialSeenAt(r.Context(), did.String())
	if err != nil {
		slog.Error("GetSocialSeenAt", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	rows, err := s.Store.ListFollowerNotifications(r.Context(), did.String(), limit, offset)
	if err != nil {
		slog.Error("ListFollowerNotifications", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	unseenCount, err := s.Store.CountFollowersSince(r.Context(), did.String(), seenAt)
	if err != nil {
		slog.Error("CountFollowersSince", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	type followerView struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName,omitempty"`
		Avatar      string `json:"avatar,omitempty"`
		FollowedAt  string `json:"followedAt"`
		YouFollow   bool   `json:"youFollow"`
		FollowUri   string `json:"followUri,omitempty"`
		IsNew       bool   `json:"isNew"`
	}
	items := make([]followerView, 0, len(rows))
	for _, row := range rows {
		items = append(items, followerView{
			DID:         row.DID,
			Handle:      row.Handle,
			DisplayName: row.DisplayName,
			Avatar:      row.Avatar,
			FollowedAt:  row.FollowedAt.UTC().Format(time.RFC3339),
			YouFollow:   row.FollowBackURI != "",
			FollowUri:   row.FollowBackURI,
			IsNew:       row.FollowedAt.After(seenAt),
		})
	}

	resp := map[string]any{"items": items, "unseenCount": unseenCount}
	if len(rows) == limit {
		resp["cursor"] = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset + len(rows))))
	}
	json.NewEncoder(w).Encode(resp)
}

// APIMeBlueskyFollows returns the Currents users that the signed-in user already
// follows on Bluesky but does not yet follow on Currents — the candidates for the
// "import Bluesky follows" dialog. A user's Currents DID is their Bluesky DID, so
// their Bluesky follow graph lives in their own PDS as app.bsky.graph.follow records.
func (s *Server) APIMeBlueskyFollows(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	// Page through the user's Bluesky follow records, collecting subject DIDs.
	// Capped to bound latency for accounts that follow very many people.
	const maxPages = 50
	subjects := make([]string, 0, 256)
	cursor := ""
	for page := 0; page < maxPages; page++ {
		params := map[string]any{
			"repo":       did.String(),
			"collection": "app.bsky.graph.follow",
			"limit":      100,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var resp struct {
			Cursor  string `json:"cursor"`
			Records []struct {
				Value struct {
					Subject string `json:"subject"`
				} `json:"value"`
			} `json:"records"`
		}
		if err := c.Get(r.Context(), "com.atproto.repo.listRecords", params, &resp); err != nil {
			if s.handleSessionError(err, w, r) {
				return
			}
			slog.Error("listing bluesky follows", "err", err, "did", did.String())
			http.Error(w, "could not read Bluesky follows", http.StatusBadGateway)
			return
		}
		for _, rec := range resp.Records {
			if rec.Value.Subject != "" {
				subjects = append(subjects, rec.Value.Subject)
			}
		}
		if resp.Cursor == "" || len(resp.Records) == 0 {
			break
		}
		cursor = resp.Cursor
	}

	rows, err := s.Store.FindFollowableCurrentsUsers(r.Context(), did.String(), subjects)
	if err != nil {
		slog.Error("FindFollowableCurrentsUsers", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	type actorView struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName,omitempty"`
		Avatar      string `json:"avatar,omitempty"`
	}
	actors := make([]actorView, 0, len(rows))
	for _, row := range rows {
		actors = append(actors, actorView{
			DID:         row.DID,
			Handle:      row.Handle,
			DisplayName: row.DisplayName,
			Avatar:      row.Avatar,
		})
	}
	json.NewEncoder(w).Encode(map[string]any{"actors": actors})
}

// APIMeSocialSeen marks the Activity tab seen as of now, clearing the unread dot.
func (s *Server) APIMeSocialSeen(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.Store.MarkSocialSeen(r.Context(), did.String()); err != nil {
		slog.Error("MarkSocialSeen", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
