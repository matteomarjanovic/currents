package main

import (
	"encoding/base64"
	"math/rand"
	"reflect"
	"testing"
)

func TestFeedCursorRoundTrip(t *testing.T) {
	original := feedCursor{
		Version:     1,
		Mode:        feedCursorModePositive,
		Initialized: true,
		Collections: []feedCursorCollection{
			{URI: "at://did:plc:alice/is.currents.feed.collection/one", Offset: 17},
			{URI: "at://did:plc:alice/is.currents.feed.collection/two", Offset: 9},
		},
		GlobalOffset: 33,
	}

	encoded, err := encodeFeedCursor(original)
	if err != nil {
		t.Fatalf("encodeFeedCursor: %v", err)
	}

	decoded, err := decodeFeedCursor(encoded)
	if err != nil {
		t.Fatalf("decodeFeedCursor: %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("decoded cursor mismatch: got %#v want %#v", decoded, original)
	}
}

func TestFeedCursorRoundTripNegative(t *testing.T) {
	original := feedCursor{
		Version:     1,
		Mode:        feedCursorModeNegative,
		Initialized: true,
		Seeds: []feedCursorSeed{
			{VisualIdentityID: "28d1e31d-5142-42fe-9fd2-b433ef4d2e7d", Offset: 5},
			{VisualIdentityID: "8d1a4c79-7cab-45a2-b7a6-96be55b76f57", Offset: 11},
		},
		GlobalOffset: 4,
	}

	encoded, err := encodeFeedCursor(original)
	if err != nil {
		t.Fatalf("encodeFeedCursor: %v", err)
	}

	decoded, err := decodeFeedCursor(encoded)
	if err != nil {
		t.Fatalf("decodeFeedCursor: %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("decoded cursor mismatch: got %#v want %#v", decoded, original)
	}
}

func TestDecodeFeedCursorRejectsLegacyOffset(t *testing.T) {
	legacy := base64.RawURLEncoding.EncodeToString([]byte("50"))

	if _, err := decodeFeedCursor(legacy); err == nil {
		t.Fatal("decodeFeedCursor unexpectedly accepted a legacy cursor")
	}
}

func TestFeedCursorModeMismatch(t *testing.T) {
	cursor := feedCursor{
		Version:     1,
		Mode:        feedCursorModePositive,
		Initialized: true,
		Collections: []feedCursorCollection{{URI: "at://did:plc:alice/is.currents.feed.collection/one", Offset: 2}},
	}

	if err := cursor.validateForMode(feedCursorModeNegative); err == nil {
		t.Fatal("validateForMode unexpectedly accepted a mismatched cursor mode")
	}
}

func TestBuildFeedPageConsumesDuplicates(t *testing.T) {
	pools := []*feedCandidatePool{
		{
			Key:    "col-1",
			Items:  []SaveRow{{URI: "a"}, {URI: "b"}},
			Weight: 0,
		},
		{
			Key:    "col-2",
			Items:  []SaveRow{{URI: "a"}, {URI: "c"}},
			Weight: 0,
		},
	}

	rows := buildFeedPage(rand.New(rand.NewSource(1)), pools, 3)
	got := []string{rows[0].URI, rows[1].URI, rows[2].URI}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("row URIs = %v, want %v", got, want)
	}

	if pools[1].consumed != 2 {
		t.Fatalf("second pool consumed = %d, want 2", pools[1].consumed)
	}
	if pools[1].hasMoreAfterPage() {
		t.Fatalf("second pool should be exhausted after consuming the duplicate and the next unique item")
	}
	if pools[1].nextOffset() != 2 {
		t.Fatalf("second pool next offset = %d, want 2", pools[1].nextOffset())
	}
}

func TestBuildFeedPageKeepsPerPoolOffsets(t *testing.T) {
	pools := []*feedCandidatePool{
		{
			Key:    "col-1",
			Items:  []SaveRow{{URI: "a1"}, {URI: "a2"}},
			Weight: 0,
		},
		{
			Key:    "col-2",
			Items:  []SaveRow{{URI: "b1"}, {URI: "b2"}},
			Weight: 0,
		},
		{
			Key:    "col-3",
			Items:  []SaveRow{{URI: "c1"}, {URI: "c2"}},
			Weight: 0,
		},
	}

	rows := buildFeedPage(rand.New(rand.NewSource(7)), pools, 4)
	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}

	totalConsumed := pools[0].consumed + pools[1].consumed + pools[2].consumed
	if totalConsumed != 4 {
		t.Fatalf("total consumed = %d, want 4", totalConsumed)
	}
	if pools[0].nextOffset() != 2 || pools[1].nextOffset() != 2 || pools[2].nextOffset() != 0 {
		t.Fatalf("expected per-pool offsets 2,2,0; got %d,%d,%d", pools[0].nextOffset(), pools[1].nextOffset(), pools[2].nextOffset())
	}
	if !pools[2].hasMoreAfterPage() {
		t.Fatalf("third pool should still have remaining items")
	}
}
