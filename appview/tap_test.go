package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
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
