package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/atclient"
)

var errCollectionDeleting = errors.New("this collection is being deleted")

// A section is a writable destination only while its parent exists. Check
// the PDS, not TAP, so newly created collections work before indexing and an
// orphan cannot accept invisible saves while awaiting the daily repair.
func resolveCollectionRef(ctx context.Context, c *atclient.APIClient, store *PgStore, did, uri string, rootOnly bool) (map[string]any, error) {
	if !ownCollection(did, uri) {
		return nil, fmt.Errorf("collection must belong to you")
	}
	read := func(uri string) (repositoryRecord, error) {
		var record repositoryRecord
		err := c.Get(ctx, "com.atproto.repo.getRecord", map[string]any{"repo": did, "collection": collectionNSID, "rkey": rkeyFromURI(uri)}, &record)
		return record, err
	}
	record, err := read(uri)
	if err != nil {
		return nil, err
	}
	parent := recordRef(record, "parent")
	if parent != "" {
		if rootOnly || !ownCollection(did, parent) || parent == uri {
			return nil, fmt.Errorf("parent must be one of your root collections")
		}
		p, err := read(parent)
		if err != nil {
			return nil, fmt.Errorf("this section's parent is unavailable: %w", err)
		}
		if recordRef(p, "parent") != "" {
			return nil, fmt.Errorf("nested sections are not supported")
		}
	}
	deleting, err := store.collectionDeleting(ctx, uri, parent)
	if err != nil {
		return nil, err
	}
	if deleting {
		return nil, errCollectionDeleting
	}
	return map[string]any{"uri": uri, "cid": record.CID}, nil
}

func (s *Server) lockRepositoryRequest(w http.ResponseWriter, r *http.Request, did string) (func(), bool) {
	unlock, err := s.Store.lockRepository(r.Context(), did)
	if err != nil {
		http.Error(w, "could not access your repository; please retry", http.StatusServiceUnavailable)
		return nil, false
	}
	return unlock, true
}
