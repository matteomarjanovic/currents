package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

func (s *Server) APISetCollectionPinned(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var body struct {
		CollectionURI string `json:"collectionUri"`
		Pinned        bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	uri, err := syntax.ParseATURI(body.CollectionURI)
	if err != nil || uri.Authority().String() != did.String() || uri.Collection().String() != collectionNSID {
		http.Error(w, "collectionUri must be your own is.currents.feed.collection record", http.StatusBadRequest)
		return
	}
	found, err := s.Store.SetCollectionPinned(r.Context(), did.String(), body.CollectionURI, body.Pinned)
	if err != nil {
		http.Error(w, fmt.Sprintf("saving collection pin: %s", err), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "collection not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
