package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Per-user "seen feature" flags drive one-time "new feature" indicators in the
// web client. Keys are arbitrary strings owned by the frontend, so shipping a
// new announcement needs no backend change.

// onboardingSeenFeatures are pre-dismissed at sign-up. The dots double as
// onboarding, but to someone signing up today nothing is actually "new" — all
// of them at once is noise, and it teaches people that a red dot means
// nothing. So a first login keeps only the few worth pointing a newcomer at
// (Pinterest import, organize mode) and marks the rest seen. Keys mirror
// frontend/src/lib/stores/features.svelte.ts; add one here when an
// announcement should stop greeting new sign-ups. Existing users are
// untouched — they keep whatever they haven't dismissed.
var onboardingSeenFeatures = []string{
	"bluesky-import",
	"become-supporter",
	"color-search",
}

func (s *Server) APIGetSeenFeatures(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	keys, err := s.Store.GetSeenFeatures(r.Context(), did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading seen features: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"seen": keys})
}

func (s *Server) APIMarkFeatureSeen(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	key := strings.TrimSpace(r.PathValue("key"))
	if key == "" || len(key) > 64 {
		http.Error(w, "invalid feature key", http.StatusBadRequest)
		return
	}
	if err := s.Store.MarkFeatureSeen(r.Context(), did.String(), key); err != nil {
		http.Error(w, fmt.Sprintf("saving seen feature: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
