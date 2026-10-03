package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// A record that can never be parsed must be reported as errSkipRecord so
// handleTapConn acks it instead of head-of-line-blocking TAP's outbox. Every
// case here fails before any Store/Dir call, so a zero-value handler is enough.
func TestHandleTapRecordSkipsUnprocessable(t *testing.T) {
	cases := []struct {
		name       string
		collection string
		record     string
	}{
		// The real-world poison: a backfill profile event delivered with no body.
		{"profile empty body", "is.currents.actor.profile", ""},
		{"profile malformed", "is.currents.actor.profile", "not json"},
		{"collection malformed", collectionNSID, "{"},
		{"save malformed", saveNSID, "garbage"},
		{"follow malformed", followNSID, "[]"},
		{"favourite malformed", favouriteNSID, "{"},
		{"favourite empty subject uri", favouriteNSID, `{"subject":{"uri":""}}`},
	}
	h := &TapHandler{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := handleTapRecord(context.Background(), h, &TapRecordEvent{
				DID:        "did:plc:test",
				Collection: c.collection,
				Rkey:       "self",
				Action:     "create",
				Record:     json.RawMessage(c.record),
			})
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !errors.Is(err, errSkipRecord) {
				t.Fatalf("expected errSkipRecord, got %v", err)
			}
		})
	}
}

func TestTapIdentityUpdatesActorLinksAndPurgesCachedDID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:changedhandle"
	const viewer = "did:plc:viewer"
	base := identity.NewMockDirectory()
	base.Insert(identity.Identity{DID: syntax.DID(did), Handle: syntax.Handle("old.example.test")})
	dir := identity.NewCacheDirectory(base, 10, time.Hour, time.Minute, time.Minute)
	if _, err := dir.LookupDID(ctx, syntax.DID(did)); err != nil {
		t.Fatal(err) // Cache the old handle before the identity event.
	}
	for _, user := range []UserRecord{
		{DID: did, Handle: "old.example.test", DisplayName: "Alice", CreatedAt: time.Now()},
		{DID: viewer, Handle: "viewer.example.test", CreatedAt: time.Now()},
	} {
		if err := store.CreateUser(ctx, user); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertFollow(ctx, "at://"+viewer+"/is.currents.graph.follow/one", viewer, did); err != nil {
		t.Fatal(err)
	}

	base.Insert(identity.Identity{DID: syntax.DID(did), Handle: syntax.Handle("new.example.test")})
	handler := &TapHandler{Store: store, Dir: dir}
	if err := handleTapIdentity(ctx, handler, &TapIdentityEvent{DID: did, Handle: "NEW.example.test"}); err != nil {
		t.Fatal(err)
	}
	actor, err := store.GetActorByDID(ctx, did)
	if err != nil || actor == nil || actor.Handle != "new.example.test" || actor.DisplayName != "Alice" {
		t.Fatalf("actor after identity event = %+v, %v", actor, err)
	}
	follows, err := store.GetFollows(ctx, viewer, 10, 0)
	if err != nil || len(follows) != 1 || follows[0].Handle != "new.example.test" {
		t.Fatalf("follow link after identity event = %+v, %v", follows, err)
	}
	matches, err := store.SearchActors(ctx, "new.example.test", 10, 0)
	if err != nil || len(matches) != 1 || matches[0].DID != did {
		t.Fatalf("search after identity event = %+v, %v", matches, err)
	}

	if err := handleTapRecord(ctx, handler, &TapRecordEvent{
		DID: did, Collection: currentsProfileNSID, Rkey: "self", Action: "update",
		Record: json.RawMessage(`{"displayName":"Alice","createdAt":"2026-01-01T00:00:00Z"}`),
	}); err != nil {
		t.Fatal(err)
	}
	actor, err = store.GetActorByDID(ctx, did)
	if err != nil || actor == nil || actor.Handle != "new.example.test" {
		t.Fatalf("profile event restored a stale handle: %+v, %v", actor, err)
	}
	if err := handleTapIdentity(ctx, handler, &TapIdentityEvent{DID: did, Handle: "handle.invalid"}); err != nil {
		t.Fatal(err)
	}
	actor, err = store.GetActorByDID(ctx, did)
	if err != nil || actor == nil || actor.Handle != "new.example.test" {
		t.Fatalf("invalid handle replaced a valid handle: %+v, %v", actor, err)
	}
}
