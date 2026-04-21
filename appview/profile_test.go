package main

import "testing"

func TestBuildCurrentsProfileRecordPreservesExistingCreatedAtAndBlobs(t *testing.T) {
	t.Parallel()

	existing := &currentsProfileRecord{
		DisplayName: "Before",
		Description: "Before description",
		Pronouns:    "they/them",
		Website:     "https://before.example",
		Avatar:      &repoBlobRef{Ref: map[string]string{"$link": "old-avatar"}},
		Banner:      &repoBlobRef{Ref: map[string]string{"$link": "old-banner"}},
		CreatedAt:   "2026-04-01T00:00:00Z",
	}

	got := buildCurrentsProfileRecord(existing, profileUpdate{
		DisplayName:  "After",
		Description:  "After description",
		Pronouns:     "he/him",
		Website:      "https://after.example",
		Avatar:       &repoBlobRef{Ref: map[string]string{"$link": "new-avatar"}},
		RemoveBanner: true,
	}, "2026-04-21T00:00:00Z")

	if got.CreatedAt != existing.CreatedAt {
		t.Fatalf("createdAt = %q, want %q", got.CreatedAt, existing.CreatedAt)
	}
	if got.Avatar == nil || got.Avatar.Ref["$link"] != "new-avatar" {
		t.Fatalf("avatar = %#v", got.Avatar)
	}
	if got.Banner != nil {
		t.Fatalf("banner should be removed, got %#v", got.Banner)
	}
	if got.DisplayName != "After" || got.Description != "After description" || got.Pronouns != "he/him" || got.Website != "https://after.example" {
		t.Fatalf("unexpected record fields: %#v", got)
	}
}

func TestBuildCurrentsProfileRecordUsesImportedBlobsWhenNoExistingRecord(t *testing.T) {
	t.Parallel()

	got := buildCurrentsProfileRecord(nil, profileUpdate{
		DisplayName: "New",
		Avatar:      &repoBlobRef{Ref: map[string]string{"$link": "imported-avatar"}},
		Banner:      &repoBlobRef{Ref: map[string]string{"$link": "imported-banner"}},
	}, "2026-04-21T12:00:00Z")

	if got.CreatedAt != "2026-04-21T12:00:00Z" {
		t.Fatalf("createdAt = %q", got.CreatedAt)
	}
	if got.Avatar == nil || got.Avatar.Ref["$link"] != "imported-avatar" {
		t.Fatalf("avatar = %#v", got.Avatar)
	}
	if got.Banner == nil || got.Banner.Ref["$link"] != "imported-banner" {
		t.Fatalf("banner = %#v", got.Banner)
	}
}
