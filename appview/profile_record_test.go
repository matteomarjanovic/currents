package main

import (
	"testing"
	"time"
)

func TestCurrentsProfileFromBskyProfileCopiesSharedFields(t *testing.T) {
	t.Parallel()

	bsky := bskyActorProfile{
		DisplayName: "Matteo",
		Description: "From Bluesky",
		Avatar:      &repoBlobRef{Ref: map[string]string{"$link": "avatar-cid"}},
		Banner:      &repoBlobRef{Ref: map[string]string{"$link": "banner-cid"}},
	}

	got := currentsProfileFromBskyProfile(bsky, "2026-04-21T10:00:00Z")
	if got.DisplayName != bsky.DisplayName {
		t.Fatalf("displayName = %q, want %q", got.DisplayName, bsky.DisplayName)
	}
	if got.Description != bsky.Description {
		t.Fatalf("description = %q, want %q", got.Description, bsky.Description)
	}
	if got.Avatar != bsky.Avatar {
		t.Fatal("avatar pointer was not preserved")
	}
	if got.Banner != bsky.Banner {
		t.Fatal("banner pointer was not preserved")
	}
	if got.CreatedAt != "2026-04-21T10:00:00Z" {
		t.Fatalf("createdAt = %q, want %q", got.CreatedAt, "2026-04-21T10:00:00Z")
	}
	if got.Pronouns != "" || got.Website != "" {
		t.Fatalf("Currents-only fields should stay empty after Bluesky bootstrap: %#v", got)
	}
}

func TestUserRecordFromCurrentsProfileUsesAllFields(t *testing.T) {
	t.Parallel()

	fallback := time.Date(2026, time.April, 20, 9, 0, 0, 0, time.UTC)
	profile := currentsProfileRecord{
		DisplayName: "Currents Matteo",
		Description: "Currents bio",
		Pronouns:    "he/him",
		Website:     "https://currents.is",
		Avatar:      &repoBlobRef{Ref: map[string]string{"$link": "avatar-cid"}},
		Banner:      &repoBlobRef{Ref: map[string]string{"$link": "banner-cid"}},
		CreatedAt:   "2026-04-21T11:22:33Z",
	}

	got := userRecordFromCurrentsProfile(
		"did:plc:alice",
		"alice.test",
		"https://pds.example",
		"https://cdn.example",
		profile,
		fallback,
	)

	if got.DisplayName != profile.DisplayName || got.Description != profile.Description {
		t.Fatalf("unexpected text fields: %#v", got)
	}
	if got.Pronouns != profile.Pronouns || got.Website != profile.Website {
		t.Fatalf("unexpected Currents-only fields: %#v", got)
	}
	if got.Avatar != "https://cdn.example/img/did:plc:alice/avatar-cid" {
		t.Fatalf("avatar = %q", got.Avatar)
	}
	if got.Banner != "https://cdn.example/img/did:plc:alice/banner-cid" {
		t.Fatalf("banner = %q", got.Banner)
	}
	if !got.CreatedAt.Equal(time.Date(2026, time.April, 21, 11, 22, 33, 0, time.UTC)) {
		t.Fatalf("createdAt = %s", got.CreatedAt)
	}
	if got.Handle != "alice.test" || got.PDSEndpoint != "https://pds.example" {
		t.Fatalf("unexpected identity fields: %#v", got)
	}
}

func TestUserRecordFromCurrentsProfileFallsBackCreatedAt(t *testing.T) {
	t.Parallel()

	fallback := time.Date(2026, time.April, 19, 8, 30, 0, 0, time.UTC)
	got := userRecordFromCurrentsProfile(
		"did:plc:bob",
		"bob.test",
		"",
		"https://cdn.example",
		currentsProfileRecord{CreatedAt: "not-a-timestamp"},
		fallback,
	)

	if !got.CreatedAt.Equal(fallback) {
		t.Fatalf("createdAt = %s, want fallback %s", got.CreatedAt, fallback)
	}
	if got.Avatar != "" || got.Banner != "" {
		t.Fatalf("unexpected blob urls: %#v", got)
	}
}
