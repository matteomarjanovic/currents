package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func jetstreamTestHandler(store *PgStore, pdsURL string) *TapHandler {
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID: syntax.DID(repairTestDID), Handle: syntax.Handle("repair.example.test"),
		Services: map[string]identity.ServiceEndpoint{"atproto_pds": {URL: pdsURL}},
	})
	return &TapHandler{Context: context.Background(), Store: store, Dir: dir, IngestSource: "jetstream"}
}

func TestJetstreamBackfillReconcilesPDSRecords(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	root := collectionTestURI("root")
	stale := collectionTestURI("stale")
	save := saveTestURI("current")
	followURI := "at://" + repairTestDID + "/" + followNSID + "/one"
	favouriteURI := "at://" + repairTestDID + "/" + favouriteNSID + "/one"
	p, client := newMaintenancePDS(t,
		testRepositoryRecord("at://"+repairTestDID+"/"+currentsProfileNSID+"/self", `{"displayName":"Alice"}`),
		testRepositoryRecord(root, `{"name":"Root","createdAt":"2026-01-01T00:00:00Z"}`),
		testRepositoryRecord(followURI, `{"subject":"did:plc:other"}`),
		testRepositoryRecord(favouriteURI, fmt.Sprintf(`{"subject":{"uri":%q}}`, root)),
		testRepositoryRecord(save, fmt.Sprintf(`{"collection":{"uri":%q},"content":{"$type":"is.currents.content.text"},"text":"Hello","createdAt":"2026-01-01T00:00:00Z"}`, root)),
	)
	_ = p
	if _, err := store.RegisterJetstreamRepo(ctx, repairTestDID); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCollection(ctx, stale, "old-cid", repairTestDID, "Stale", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSave(ctx, UpsertSaveParams{
		URI: saveTestURI("stale"), AuthorDID: repairTestDID,
		ContentNSID: "is.currents.content.text", CreatedAt: &testBase,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID); err != nil {
		t.Fatal(err)
	}
	var names []string
	rows, err := store.pool.Query(ctx, `SELECT name FROM collection WHERE author_did = $1 ORDER BY name`, repairTestDID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	rows.Close()
	if len(names) != 1 || names[0] != "Root" {
		t.Fatalf("collections after reconcile = %v", names)
	}
	var saveCount int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM save WHERE author_did = $1 AND uri = $2`, repairTestDID, save).Scan(&saveCount); err != nil || saveCount != 1 {
		t.Fatalf("current save count = %d, %v", saveCount, err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM save WHERE uri = $1`, saveTestURI("stale")).Scan(&saveCount); err != nil || saveCount != 0 {
		t.Fatalf("stale save count = %d, %v", saveCount, err)
	}
	for _, tableURI := range []struct{ table, uri string }{{"follow", followURI}, {"favourite_collection", favouriteURI}} {
		var count int
		if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM `+tableURI.table+` WHERE uri = $1`, tableURI.uri).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count = %d, %v", tableURI.table, count, err)
		}
	}
	state, rev, err := store.JetstreamRepoState(ctx, repairTestDID)
	if err != nil || state != "active" || rev != "rev-0" {
		t.Fatalf("reconciled repo = %q %q, %v", state, rev, err)
	}
	actor, err := store.GetActorByDID(ctx, repairTestDID)
	if err != nil || actor == nil || actor.DisplayName != "Alice" {
		t.Fatalf("profile after reconcile = %+v, %v", actor, err)
	}
}

func TestJetstreamBackfillLeavesIndexUntouchedOnPartialPDSRead(t *testing.T) {
	for _, failure := range []string{"page", "changed"} {
		t.Run(failure, func(t *testing.T) {
			store := newTestStore(t)
			ctx := context.Background()
			records := []repositoryRecord{}
			for i := range 101 {
				records = append(records, testRepositoryRecord(collectionTestURI(fmt.Sprintf("%03d", i)), `{"name":"Current"}`))
			}
			p, client := newMaintenancePDS(t, records...)
			p.failPage = failure == "page"
			p.changeDuringRead = failure == "changed"
			if _, err := store.RegisterJetstreamRepo(ctx, repairTestDID); err != nil {
				t.Fatal(err)
			}
			stale := collectionTestURI("stale")
			if err := store.UpsertCollection(ctx, stale, "old-cid", repairTestDID, "Stale", "", "", nil); err != nil {
				t.Fatal(err)
			}
			if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID); err == nil {
				t.Fatal("partial PDS read unexpectedly reconciled")
			}
			var count int
			if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE uri = $1`, stale).Scan(&count); err != nil || count != 1 {
				t.Fatalf("partial read deleted indexed collection: %d, %v", count, err)
			}
		})
	}
}

func TestJetstreamBackfillQueueRetry(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = repairTestDID
	if err := store.QueueJetstreamBackfill(ctx, did); err != nil {
		t.Fatal(err)
	}
	got, due, attempts, err := store.NextJetstreamBackfill(ctx)
	if err != nil || got != did || attempts != 0 || due.IsZero() {
		t.Fatalf("queue = %q %v %d, %v", got, due, attempts, err)
	}
	if err := store.RetryJetstreamBackfill(ctx, did, due, 30); err != nil {
		t.Fatal(err)
	}
	if got, _, _, err := store.NextJetstreamBackfill(ctx); err != nil || got != "" {
		t.Fatalf("retry ran too early: %q, %v", got, err)
	}
	if err := store.QueueJetstreamBackfill(ctx, did); err != nil {
		t.Fatal(err)
	}
	got, newerDue, _, err := store.NextJetstreamBackfill(ctx)
	if err != nil || got != did {
		t.Fatalf("requeued = %q, %v", got, err)
	}
	if err := store.CompleteJetstreamBackfill(ctx, did, due); err != nil {
		t.Fatal(err)
	}
	got, _, _, err = store.NextJetstreamBackfill(ctx)
	if err != nil || got != did || newerDue.Equal(due) {
		t.Fatalf("requeue lost by stale completion: %q, %v", got, err)
	}
}
