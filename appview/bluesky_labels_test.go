package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/labeling"
	"github.com/gorilla/websocket"
)

func TestBlueskyLabelLifecycle(t *testing.T) {
	now := time.Now()
	label := func(val string, age time.Duration) blueskyLabel {
		return blueskyLabel{Src: blueskyLabelerDID, Val: val, CTS: now.Add(-age)}
	}
	porn := label("porn", time.Minute)
	negated := label("porn", time.Second)
	negated.Neg = true
	sexual := label("sexual", time.Minute)
	expired := label("sexual", time.Second)
	expired.Exp = &now
	versioned := label("nudity", time.Minute)
	versioned.CID = "record-version"
	otherVersion := label("nudity", time.Second)
	otherVersion.CID, otherVersion.Neg = "older-version", true
	labels := []blueskyLabel{negated, expired, versioned, otherVersion, porn, sexual, versioned}
	if got := activeBlueskyLabels(labels, "record-version", now); len(got) != 1 || got[0].Val != "nudity" || got[0].Src != blueskyLabelerDID {
		t.Fatalf("active labels = %+v", got)
	}
	if got := activeBlueskyLabels(labels, "different-version", now); len(got) != 0 {
		t.Fatalf("labels on another record version = %+v", got)
	}
}

func TestBlueskyLabelStreamKeepsSubjectsLocal(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:stream"
	const save = "at://" + did + "/is.currents.feed.save/photo"
	const sibling = "at://" + did + "/is.currents.feed.save/resave"
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := priv.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO "user" (did, created_at) VALUES ($1, now())`, did); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sign := func(seq int64, src, uri, value string, cts time.Time, neg bool) LabelRow {
		t.Helper()
		l := labeling.Label{Version: 1, SourceDID: src, URI: uri, Val: value, CreatedAt: cts.Format(time.RFC3339Nano)}
		if neg {
			l.Negated = &neg
		}
		if err := l.Sign(priv); err != nil {
			t.Fatal(err)
		}
		return LabelRow{ID: seq, Src: src, URI: uri, Val: value, CTS: cts, Neg: neg, Ver: 1, Sig: l.Sig}
	}

	valid := sign(1, blueskyLabelerDID, save, "porn", now, false)
	account := sign(2, blueskyLabelerDID, did, "!warn", now, false)
	forged := sign(3, blueskyLabelerDID, sibling, "porn", now, false)
	forged.Val = "!hide"
	foreign := sign(4, "did:plc:other", sibling, "!hide", now, false)
	unrelated := sign(5, blueskyLabelerDID, "at://"+did+"/app.bsky.feed.post/bsky", "!hide", now, false)
	unindexed := sign(6, blueskyLabelerDID, "did:plc:unindexed", "!hide", now, false)
	duplicate := valid
	duplicate.ID = 7
	retracted := sign(8, blueskyLabelerDID, save, "porn", now.Add(time.Second), true)
	outOfOrder := valid
	outOfOrder.ID = 9

	for _, invalid := range []LabelRow{forged, foreign, unrelated} {
		if _, ok := parseBlueskyLabel(labelRowToLexicon(invalid), pub); ok {
			t.Fatalf("accepted invalid label from %q on %q", invalid.Src, invalid.URI)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query()) != 0 {
			t.Errorf("subject identifiers sent to the stream: %s", r.URL)
		}
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		for _, row := range []LabelRow{valid, account, forged, foreign, unrelated, unindexed, duplicate, retracted, outOfOrder} {
			if err := writeLabelFrame(conn, row); err != nil {
				t.Error(err)
				return
			}
		}
	}))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// The upstream disconnect is expected. The last complete batch is preserved.
	if err := consumeBlueskyLabels(ctx, conn, pub, db, 0); !websocket.IsUnexpectedCloseError(err) {
		t.Fatalf("expected a stream disconnect, got %v", err)
	}
	cursor, err := db.blueskyLabelCursor(ctx)
	if err != nil || cursor != 9 {
		t.Fatalf("saved cursor %d, err %v", cursor, err)
	}
	got, err := db.getBlueskyLabels(ctx, map[string]string{save: did, sibling: did})
	if err != nil || len(got[save]) != 1 || len(got[sibling]) != 1 || got[sibling][0].URI != did {
		t.Fatalf("visible labels = %+v, %v", got, err)
	}

	var count int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM external_label").Scan(&count); count != 2 || err != nil {
		t.Fatalf("persisted %d labels, err %v", count, err)
	}
}

func TestBlueskyLabelStreamCIDIsRecordVersion(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()
	const owner = "did:plc:version"
	const uri = "at://" + owner + "/is.currents.feed.save/image"
	const other = "at://" + owner + "/is.currents.feed.save/other"
	p := UpsertSaveParams{URI: uri, AuthorDID: owner, RecordCID: "one", PdsBlobCID: "same-blob", ContentNSID: saveContentImageNSID}
	if err := db.UpsertSave(ctx, p); err != nil {
		t.Fatal(err)
	}
	otherSave := p
	otherSave.URI = other
	if err := db.UpsertSave(ctx, otherSave); err != nil {
		t.Fatal(err)
	}
	if err := db.saveBlueskyLabelBatch(ctx, 1, []blueskyLabel{
		{Src: blueskyLabelerDID, URI: uri, CID: p.RecordCID, Val: "porn", CTS: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	get := func() map[string][]labelView {
		t.Helper()
		got, err := db.getBlueskyLabels(ctx, map[string]string{uri: owner, other: owner})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := get(); len(got[uri]) != 1 || len(got[other]) != 0 {
		t.Fatalf("version-specific labels = %+v", got)
	}
	p.RecordCID = "two"
	if err := db.UpsertSave(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got := get(); len(got) != 0 {
		t.Fatalf("edited save carried a label from a different version: %+v", got)
	}
}
