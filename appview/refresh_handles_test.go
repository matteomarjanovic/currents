package main

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func TestRefreshUserHandles(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:changedhandle"
	if err := store.CreateUser(ctx, UserRecord{
		DID: did, Handle: "old.example.test", DisplayName: "Alice", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{DID: syntax.DID(did), Handle: syntax.Handle("new.example.test")})
	if err := refreshUserHandles(ctx, store, dir, did, true); err != nil {
		t.Fatal(err)
	}
	actor, err := store.GetActorByDID(ctx, did)
	if err != nil || actor == nil || actor.Handle != "old.example.test" {
		t.Fatalf("dry run changed actor: %+v, %v", actor, err)
	}
	if err := refreshUserHandles(ctx, store, dir, did, false); err != nil {
		t.Fatal(err)
	}
	actor, err = store.GetActorByDID(ctx, did)
	if err != nil || actor == nil || actor.Handle != "new.example.test" || actor.DisplayName != "Alice" {
		t.Fatalf("refreshed actor = %+v, %v", actor, err)
	}
}
