package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// FeedPrefs mirrors the JSON shape consumed by the web client.
type FeedPrefs struct {
	ExcludedCollections []string `json:"excludedCollections"`
	DefaultFeed         string   `json:"defaultFeed"`
}

const defaultFeed = "personal"

func (s *Server) APIGetFeedPreferences(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	prefs, err := s.Store.GetFeedPrefs(r.Context(), did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading feed preferences: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

func (s *Server) APIPutFeedPreferences(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var prefs FeedPrefs
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if prefs.ExcludedCollections == nil {
		prefs.ExcludedCollections = []string{}
	}
	if prefs.DefaultFeed == "" {
		prefs.DefaultFeed = defaultFeed
	}
	if prefs.DefaultFeed != "general" && prefs.DefaultFeed != "new-worlds" && prefs.DefaultFeed != "personal" {
		http.Error(w, "invalid default feed", http.StatusBadRequest)
		return
	}
	if err := s.Store.SetFeedPrefs(r.Context(), did.String(), prefs); err != nil {
		http.Error(w, fmt.Sprintf("saving feed preferences: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
