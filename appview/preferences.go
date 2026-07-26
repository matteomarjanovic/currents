package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Per-user general preferences. Server-backed so they follow the user across
// browsers and devices (web + mobile). Distinct from moderation prefs, which
// gate content visibility; these are UI/rendering preferences.

// UserPrefs mirrors the JSON shape consumed by the web client.
type UserPrefs struct {
	GifAutoplay bool `json:"gifAutoplay"`
}

// defaultUserPrefs is returned for users with no stored row. Kept in sync with
// the DB column defaults in migration 042.
var defaultUserPrefs = UserPrefs{
	GifAutoplay: true,
}

func (s *Server) APIGetPreferences(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	prefs, err := s.Store.GetUserPrefs(r.Context(), did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading preferences: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

func (s *Server) APIPutPreferences(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var prefs UserPrefs
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.Store.SetUserPrefs(r.Context(), did.String(), prefs); err != nil {
		http.Error(w, fmt.Sprintf("saving preferences: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
