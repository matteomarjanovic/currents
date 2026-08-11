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

func TestFeedCursorRoundTripWithSeed(t *testing.T) {
	original := feedCursor{
		Version:      1,
		Mode:         feedCursorModePositive,
		Initialized:  true,
		Collections:  []feedCursorCollection{{URI: "at://did:plc:alice/is.currents.feed.collection/one", Offset: 3}},
		GlobalOffset: 0,
		Seed:         -8567143274837239424,
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

func TestFeedCursorGlobalRejectsSeed(t *testing.T) {
	// A global cursor must never carry a seed — the global feed keeps its own
	// daily jitter and stays reproducible within a day.
	if _, err := encodeFeedCursor(feedCursor{Version: 1, Mode: feedCursorModeGlobal, Seed: 42}); err == nil {
		t.Fatal("encodeFeedCursor accepted a global cursor with a seed")
	}
}

func TestSampleCollectionsDeterministicAndDistinct(t *testing.T) {
	cands := []CollectionImportance{
		{URI: "a", Score: 50},
		{URI: "b", Score: 30},
		{URI: "c", Score: 20},
		{URI: "d", Score: 10},
		{URI: "e", Score: 5},
	}

	first := sampleCollections(cands, 3, rand.New(rand.NewSource(99)))
	again := sampleCollections(cands, 3, rand.New(rand.NewSource(99)))
	if !reflect.DeepEqual(first, again) {
		t.Fatalf("same seed gave different selections: %v vs %v", uris(first), uris(again))
	}
	if len(first) != 3 {
		t.Fatalf("picked %d, want 3", len(first))
	}
	seen := map[string]bool{}
	for _, c := range first {
		if seen[c.URI] {
			t.Fatalf("duplicate pick %q — sampling must be without replacement", c.URI)
		}
		seen[c.URI] = true
	}
}

func TestSampleCollectionsClampsToPool(t *testing.T) {
	cands := []CollectionImportance{{URI: "a", Score: 1}, {URI: "b", Score: 1}}
	got := sampleCollections(cands, 5, rand.New(rand.NewSource(1)))
	if len(got) != 2 {
		t.Fatalf("picked %d, want 2 (all available)", len(got))
	}
}

func TestSampleCollectionsFavoursHigherScores(t *testing.T) {
	// Over many seeds the highest-scored collection should be selected far more
	// often than the lowest — variety, not uniformity.
	cands := []CollectionImportance{
		{URI: "big", Score: 100},
		{URI: "mid", Score: 20},
		{URI: "small", Score: 2},
	}
	counts := map[string]int{}
	for seed := int64(0); seed < 2000; seed++ {
		for _, c := range sampleCollections(cands, 1, rand.New(rand.NewSource(seed))) {
			counts[c.URI]++
		}
	}
	if counts["big"] <= counts["small"] {
		t.Fatalf("expected 'big' picked more than 'small'; got big=%d small=%d", counts["big"], counts["small"])
	}
	// But a low-scored collection should still surface sometimes (this is the point).
	if counts["small"] == 0 {
		t.Fatal("expected 'small' to be sampled at least once across 2000 seeds")
	}
}

func TestSeededStartOffset(t *testing.T) {
	// In range, deterministic, and sensitive to both seed and key.
	for seed := int64(1); seed < 50; seed++ {
		o := seededStartOffset(seed, "at://x", 60)
		if o < 0 || o >= 60 {
			t.Fatalf("offset %d out of [0,60)", o)
		}
		if seededStartOffset(seed, "at://x", 60) != o {
			t.Fatal("seededStartOffset not deterministic")
		}
	}
	if seededStartOffset(7, "key", 0) != 0 {
		t.Fatal("zero window must yield offset 0")
	}
	// Different keys under one seed should not all collapse to the same offset.
	distinct := map[int]bool{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		distinct[seededStartOffset(123, k, 60)] = true
	}
	if len(distinct) < 2 {
		t.Fatal("expected varied offsets across keys for one seed")
	}
}

func uris(cs []CollectionImportance) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.URI
	}
	return out
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
