package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// APIGetBlobAlt returns an existing alt text for the exact blob CID, if any user
// has already saved that image with alt text. The uploader and the browser
// extension call it to pre-fill the alt field with a suggestion before a
// re-upload. Alt text is already public via XRPC; session auth just gates this to
// the authenticated clients that need it.
func (s *Server) APIGetBlobAlt(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	cid := strings.TrimSpace(r.URL.Query().Get("cid"))
	if cid == "" {
		http.Error(w, "cid is required", http.StatusBadRequest)
		return
	}
	alt, err := s.Store.GetAltByBlobCID(r.Context(), cid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"alt": alt})
}
