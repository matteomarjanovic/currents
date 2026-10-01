package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClassify(t *testing.T) {
	put := payload{Operation: "create", Rev: "3abc", CID: "new"}
	del := payload{Operation: "delete", Rev: "3abc"}
	for _, tc := range []struct {
		name, state, rev, cid, want string
		event                       payload
	}{
		{"matching put", "active", "3abc", "new", "match", put},
		{"matching delete", "active", "3abc", "", "match", del},
		{"lagging repo", "active", "3abb", "", "tap-behind", put},
		{"wrong CID at same rev", "active", "3abc", "other", "mismatch", put},
		{"later revision is ambiguous", "active", "3abd", "other", "changed-later-or-missing", put},
		{"untracked repo", "", "", "", "untracked", put},
		{"pending repo", "pending", "", "", "tap-pending", put},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.event, tc.state, tc.rev, tc.cid); got != tc.want {
				t.Fatalf("classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStreamURL(t *testing.T) {
	raw, err := streamURL("wss://example.test/xrpc/network.bsky.jetstream.subscribeEvents", "did:plc:sample", 123)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if q := u.Query(); q.Get("collections") != "is.currents.*" || q.Get("dids") != "did:plc:sample" || q.Get("cursor") != "123" {
		t.Fatalf("unexpected subscription query: %s", u.RawQuery)
	}
}

func TestConsumeSkipsInclusiveReplay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for _, message := range []string{
			`{"$type":"message","payload":{"$type":"network.bsky.jetstream.subscribeEvents#commit","seq":10,"did":"did:plc:sample","witnessedAt":"2026-01-01T00:00:00Z","operation":"create","collection":"is.currents.feed.save","rkey":"r1","rev":"3abc","cid":"cid1"}}`,
			`{"$type":"message","payload":{"$type":"network.bsky.jetstream.subscribeEvents#commit","seq":10,"did":"did:plc:sample","witnessedAt":"2026-01-01T00:00:00Z","operation":"create","collection":"is.currents.feed.save","rkey":"r1","rev":"3abc","cid":"cid1"}}`,
			`{"$type":"message","payload":{"$type":"network.bsky.jetstream.subscribeEvents#account","seq":11,"did":"did:plc:sample","witnessedAt":"2026-01-01T00:00:01Z"}}`,
		} {
			if conn.WriteMessage(websocket.TextMessage, []byte(message)) != nil {
				return
			}
		}
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http", "ws", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := report{Kinds: make(map[string]int), TAPChecks: make(map[string]int)}
	if err := consume(ctx, conn, nil, &out, 2, 0); err != nil {
		t.Fatal(err)
	}
	if out.Events != 2 || out.Cursor != 11 || out.Kinds["commit"] != 1 || out.Kinds["account"] != 1 {
		t.Fatalf("unexpected report: %+v", out)
	}
}
