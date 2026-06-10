package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Per-user moderation rendering preferences. Server-backed so they follow the
// user across browsers and devices (web + mobile). The frontend reads these to
// decide whether a labeled save is shown, blurred, or hidden entirely.

// ModerationPrefs mirrors the JSON shape consumed by the web client. Adult axes
// accept show|blur|hide; AI accepts show|hide (blur is meaningless for the AI
// badge).
type ModerationPrefs struct {
	Porn         string `json:"porn"`
	Sexual       string `json:"sexual"`
	Nudity       string `json:"nudity"`
	GraphicMedia string `json:"graphicMedia"`
	AIGenerated  string `json:"aiGenerated"`
}

// defaultModerationPrefs is returned for users with no stored row. Kept in sync
// with the DB column defaults in migration 025.
var defaultModerationPrefs = ModerationPrefs{
	Porn:         "blur",
	Sexual:       "blur",
	Nudity:       "blur",
	GraphicMedia: "blur",
	AIGenerated:  "show",
}

func validAdult(v string) bool { return v == "show" || v == "blur" || v == "hide" }
func validAI(v string) bool    { return v == "show" || v == "hide" }

func (p ModerationPrefs) valid() bool {
	return validAdult(p.Porn) && validAdult(p.Sexual) && validAdult(p.Nudity) &&
		validAdult(p.GraphicMedia) && validAI(p.AIGenerated)
}

func (s *Server) APIGetModerationPrefs(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	prefs, err := s.Store.GetModerationPrefs(r.Context(), did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading moderation prefs: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

func (s *Server) APIPutModerationPrefs(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var prefs ModerationPrefs
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !prefs.valid() {
		http.Error(w, "invalid preference value", http.StatusBadRequest)
		return
	}
	if err := s.Store.SetModerationPrefs(r.Context(), did.String(), prefs); err != nil {
		http.Error(w, fmt.Sprintf("saving moderation prefs: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
