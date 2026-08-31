package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const maxSaveSuggestionBatch = 50

// APIGetSaveSuggestions recommends one of the viewer's collections or sections
// for each save. The stored image and collection embeddings make this a cheap
// comparison; no inference request is involved.
func (s *Server) APIGetSaveSuggestions(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var body struct {
		SaveURIs []string `json:"saveUris"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body.SaveURIs) == 0 || len(body.SaveURIs) > maxSaveSuggestionBatch {
		http.Error(w, fmt.Sprintf("saveUris must contain 1-%d items", maxSaveSuggestionBatch), http.StatusBadRequest)
		return
	}

	seen := make(map[string]bool, len(body.SaveURIs))
	uris := make([]string, 0, len(body.SaveURIs))
	for _, uri := range body.SaveURIs {
		if uri != "" && !seen[uri] {
			seen[uri] = true
			uris = append(uris, uri)
		}
	}
	if len(uris) == 0 {
		http.Error(w, "saveUris must contain a non-empty URI", http.StatusBadRequest)
		return
	}

	suggestions, err := s.Store.GetSuggestedCollections(r.Context(), did.String(), uris)
	if err != nil {
		http.Error(w, fmt.Sprintf("suggesting collections: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Suggestions map[string]string `json:"suggestions"`
	}{Suggestions: suggestions})
}
