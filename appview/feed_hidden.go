package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) APIHideFeedImage(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var body struct {
		URI string `json:"uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body.URI = strings.TrimSpace(body.URI)
	if body.URI == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	found, err := s.Store.HideFeedImage(r.Context(), did.String(), body.URI)
	if err != nil {
		http.Error(w, fmt.Sprintf("hiding feed image: %s", err), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
