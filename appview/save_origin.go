package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	comatproto "github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/ipfs/go-cid"
)

// Accept a complete strong ref rather than resolving the latest record version:
// the CID identifies the version from which the image was actually saved.
func parseOriginAtprotoRef(raw []byte) (*strongRef, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var ref strongRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil, fmt.Errorf("originAtprotoRef must be an object containing uri and cid")
	}
	ref.URI = strings.TrimSpace(ref.URI)
	ref.CID = strings.TrimSpace(ref.CID)
	uri, err := syntax.ParseATURI(ref.URI)
	if err != nil || uri.Collection() == "" || uri.RecordKey() == "" {
		return nil, fmt.Errorf("originAtprotoRef.uri must be a complete AT record URI")
	}
	if _, err := syntax.ParseDID(uri.Authority().String()); err != nil {
		return nil, fmt.Errorf("originAtprotoRef.uri must use a DID, not a handle")
	}
	if parsed, err := cid.Decode(ref.CID); err != nil || parsed.Version() != 1 {
		return nil, fmt.Errorf("originAtprotoRef.cid must be a valid record CID")
	}
	return &ref, nil
}

// This edits one record in the viewer's own repository, including resaves.
// A JSON null explicitly removes the reference; omission is a bad request.
func (s *Server) UpdateSaveOriginAtprotoRef(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var body struct {
		OriginAtprotoRef json.RawMessage `json:"originAtprotoRef"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.OriginAtprotoRef) == 0 {
		http.Error(w, "originAtprotoRef is required (use null to clear it)", http.StatusBadRequest)
		return
	}
	ref, err := parseOriginAtprotoRef(body.OriginAtprotoRef)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	unlock, ok := s.lockRepositoryRequest(w, r, did.String())
	if !ok {
		return
	}
	defer unlock()
	rkey := r.PathValue("id")
	if err := putOriginAtprotoRefForRkey(r.Context(), c, did, rkey, ref); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("updating origin reference: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"uri":              "at://" + did.String() + "/" + saveNSID + "/" + rkey,
		"originAtprotoRef": ref,
	})
}

func putOriginAtprotoRefForRkey(ctx context.Context, c *atclient.APIClient, did *syntax.DID, rkey string, ref *strongRef) error {
	existing, err := comatproto.RepoGetRecord(ctx, c, "", saveNSID, did.String(), rkey)
	if err != nil {
		return fmt.Errorf("get existing: %w", err)
	}
	if existing.Value == nil {
		return fmt.Errorf("save record has no value")
	}
	var record map[string]any
	if err := json.Unmarshal(*existing.Value, &record); err != nil {
		return fmt.Errorf("unmarshal existing: %w", err)
	}
	if ref == nil {
		delete(record, "originAtprotoRef")
	} else {
		record["originAtprotoRef"] = ref
	}
	_, err = comatproto.RepoPutRecord(ctx, c, &comatproto.RepoPutRecord_Input{
		Collection: saveNSID,
		Repo:       did.String(),
		Rkey:       rkey,
		Record:     record,
		SwapRecord: existing.Cid,
	})
	return err
}
