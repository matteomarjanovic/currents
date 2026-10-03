package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/jetstream"
	"github.com/gorilla/websocket"
)

func jetstreamWireServer(t *testing.T, payloads ...map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/network.bsky.jetstream.subscribeEvents" || r.URL.Query().Get("collections") != "is.currents.*" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for _, payload := range payloads {
			frame, err := json.Marshal(map[string]any{"$type": "message", "payload": payload})
			if err != nil || conn.WriteMessage(websocket.TextMessage, frame) != nil {
				return
			}
		}
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestJetstreamWireAppliesAndCheckpointsEvents(t *testing.T) {
	store := newTestStore(t)
	const did = "did:plc:jetstream-wire"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	server := jetstreamWireServer(t,
		map[string]any{
			"$type": "network.bsky.jetstream.subscribeEvents#commit", "seq": 100,
			"time": now, "witnessedAt": now, "did": did, "rev": "rev-100",
			"operation": "create", "collection": currentsProfileNSID, "rkey": "self", "cid": "cid-profile",
			"record": map[string]any{"$type": currentsProfileNSID, "displayName": "Alice"},
		},
		map[string]any{
			"$type": "network.bsky.jetstream.subscribeEvents#commit", "seq": 101,
			"time": now, "witnessedAt": now, "did": did, "rev": "rev-101",
			"operation": "create", "collection": collectionNSID, "rkey": "one", "cid": "cid-collection",
			"record": map[string]any{"$type": collectionNSID, "name": "One"},
		},
	)
	client, err := jetstream.Subscribe(server.URL,
		jetstream.WithCollection("is.currents.*"),
		jetstream.WithLiveCursor(uint64(time.Now().Add(-time.Minute).UnixMicro())),
		jetstream.WithBatchSize(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{DID: syntax.DID(did), Handle: syntax.Handle("alice.example.test")})
	handler := &TapHandler{Store: store, Dir: dir, IngestSource: "jetstream"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- consumeJetstream(ctx, client, handler, 0) }()
	for {
		cursor, err := store.JetstreamCursor(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if cursor >= 101 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Jetstream events were not checkpointed")
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var name string
	if err := store.pool.QueryRow(context.Background(), `SELECT name FROM collection WHERE uri = $1`,
		"at://"+did+"/"+collectionNSID+"/one").Scan(&name); err != nil || name != "One" {
		t.Fatalf("indexed collection = %q, %v", name, err)
	}
}

type failingPurgeDirectory struct{ identity.Directory }

func (failingPurgeDirectory) Purge(context.Context, syntax.AtIdentifier) error {
	return errors.New("temporary identity cache error")
}

func TestJetstreamWireDoesNotCheckpointHandlerFailure(t *testing.T) {
	store := newTestStore(t)
	const did = "did:plc:jetstream-wire"
	if _, err := store.RegisterJetstreamRepo(context.Background(), did); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	server := jetstreamWireServer(t, map[string]any{
		"$type": "network.bsky.jetstream.subscribeEvents#identity", "seq": 102,
		"time": now, "witnessedAt": now, "did": did,
		"identity": map[string]any{"did": did, "handle": "alice.example.test", "seq": 102, "time": now},
	})
	client, err := jetstream.Subscribe(server.URL,
		jetstream.WithCollection("is.currents.*"),
		jetstream.WithLiveCursor(uint64(time.Now().Add(-time.Minute).UnixMicro())),
		jetstream.WithBatchSize(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	handler := &TapHandler{Store: store, Dir: failingPurgeDirectory{identity.NewMockDirectory()}, IngestSource: "jetstream"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := consumeJetstream(ctx, client, handler, 0); err == nil {
		t.Fatal("handler failure was not returned")
	}
	if cursor, err := store.JetstreamCursor(context.Background()); err != nil || cursor != 0 {
		t.Fatalf("cursor advanced after handler failure: %d, %v", cursor, err)
	}
}
