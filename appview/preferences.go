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
	GifAutoplay            bool   `json:"gifAutoplay"`
	OrganizeCollectionSort string `json:"organizeCollectionSort"`
}

// defaultUserPrefs is returned for users with no stored row. Kept in sync with
// the DB column defaults in migrations 042 and 048.
var defaultUserPrefs = UserPrefs{
	GifAutoplay:            true,
	OrganizeCollectionSort: "name",
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
	// Treat PUT as a field-wise update so older mobile clients that only know
	// gifAutoplay do not reset newer preferences.
	var patch struct {
		GifAutoplay            *bool   `json:"gifAutoplay"`
		OrganizeCollectionSort *string `json:"organizeCollectionSort"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	prefs, err := s.Store.GetUserPrefs(r.Context(), did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading preferences: %s", err), http.StatusInternalServerError)
		return
	}
	if patch.GifAutoplay != nil {
		prefs.GifAutoplay = *patch.GifAutoplay
	}
	if patch.OrganizeCollectionSort != nil {
		if *patch.OrganizeCollectionSort != "name" && *patch.OrganizeCollectionSort != "recent" {
			http.Error(w, "invalid organizeCollectionSort", http.StatusBadRequest)
			return
		}
		prefs.OrganizeCollectionSort = *patch.OrganizeCollectionSort
	}
	if err := s.Store.SetUserPrefs(r.Context(), did.String(), prefs); err != nil {
		http.Error(w, fmt.Sprintf("saving preferences: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
