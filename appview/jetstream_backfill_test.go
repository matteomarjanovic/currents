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
	if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID, true); err != nil {
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

	newSave := saveTestURI("new")
	p.records[save] = testRepositoryRecord(save, fmt.Sprintf(`{"collection":{"uri":%q},"content":{"$type":"is.currents.content.text"},"text":"Changed"}`, root))
	p.records[newSave] = testRepositoryRecord(newSave, fmt.Sprintf(`{"collection":{"uri":%q},"content":{"$type":"is.currents.content.text"},"text":"New"}`, root))
	p.revision++
	if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID, false); err != nil {
		t.Fatal(err)
	}
	var text string
	if err := store.pool.QueryRow(ctx, `SELECT text FROM save WHERE uri = $1`, save).Scan(&text); err != nil || text != "Hello" {
		t.Fatalf("membership audit rewrote existing save: %q, %v", text, err)
	}
	_, rev, err = store.JetstreamRepoState(ctx, repairTestDID)
	if err != nil || rev != "rev-0" {
		t.Fatalf("membership audit skipped pending event revisions: %q, %v", rev, err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT text FROM save WHERE uri = $1`, newSave).Scan(&text); err != nil || text != "New" {
		t.Fatalf("membership audit missed new save: %q, %v", text, err)
	}
	if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID, true); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT text FROM save WHERE uri = $1`, save).Scan(&text); err != nil || text != "Changed" {
		t.Fatalf("full audit did not update existing save: %q, %v", text, err)
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
			if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID, true); err == nil {
				t.Fatal("partial PDS read unexpectedly reconciled")
			}
			var count int
			if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE uri = $1`, stale).Scan(&count); err != nil || count != 1 {
				t.Fatalf("partial read deleted indexed collection: %d, %v", count, err)
			}
		})
	}
}

func TestJetstreamRepoNotFoundIsObservedWithoutDeletingRows(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	uri := collectionTestURI("retained")
	p, client := newMaintenancePDS(t, testRepositoryRecord(uri, `{"name":"Retained"}`))
	p.repoMissing = true
	if _, err := store.RegisterJetstreamRepo(ctx, repairTestDID); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCollection(ctx, uri, "old-cid", repairTestDID, "Retained", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueJetstreamBackfill(ctx, repairTestDID); err != nil {
		t.Fatal(err)
	}
	job, err := store.NextJetstreamBackfill(ctx)
	if err != nil || job.DID != repairTestDID {
		t.Fatalf("queued repo = %+v, %v", job, err)
	}
	err = reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID, job.Full)
	if !isPDSRepoNotFound(err) {
		t.Fatalf("expected explicit RepoNotFound, got %v", err)
	}
	if err := store.ObserveJetstreamRepoMissing(ctx, job.DID, job.Due); err != nil {
		t.Fatal(err)
	}
	var count int
	var observed bool
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE uri = $1`, uri).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing PDS deleted indexed collection: %d, %v", count, err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT missing_since IS NOT NULL FROM jetstream_backfill WHERE did = $1`, repairTestDID).Scan(&observed); err != nil || !observed {
		t.Fatalf("missing repo observation = %t, %v", observed, err)
	}
	p.repoMissing = false
	if err := reconcileJetstreamRepo(ctx, jetstreamTestHandler(store, client.Host), repairTestDID, job.Full); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteJetstreamBackfill(ctx, job.DID, job.Due); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM jetstream_backfill WHERE did = $1`, repairTestDID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("recovered repo left missing observation: %d, %v", count, err)
	}
}

func TestJetstreamBackfillQueueRetry(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = repairTestDID
	if err := store.QueueJetstreamBackfill(ctx, did); err != nil {
		t.Fatal(err)
	}
	job, err := store.NextJetstreamBackfill(ctx)
	if err != nil || job.DID != did || job.Attempts != 0 || job.Due.IsZero() || !job.Full {
		t.Fatalf("queue = %+v, %v", job, err)
	}
	if err := store.RetryJetstreamBackfill(ctx, did, job.Due, 30); err != nil {
		t.Fatal(err)
	}
	if got, err := store.NextJetstreamBackfill(ctx); err != nil || got.DID != "" {
		t.Fatalf("retry ran too early: %+v, %v", got, err)
	}
	if err := store.QueueJetstreamBackfill(ctx, did); err != nil {
		t.Fatal(err)
	}
	newer, err := store.NextJetstreamBackfill(ctx)
	if err != nil || newer.DID != did {
		t.Fatalf("requeued = %+v, %v", newer, err)
	}
	if err := store.CompleteJetstreamBackfill(ctx, did, job.Due); err != nil {
		t.Fatal(err)
	}
	got, err := store.NextJetstreamBackfill(ctx)
	if err != nil || got.DID != did || newer.Due.Equal(job.Due) {
		t.Fatalf("requeue lost by stale completion: %+v, %v", got, err)
	}
}

func TestJetstreamAuditScopeAndQueue(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const first = "did:plc:first"
	const second = "did:plc:second"
	const removed = "did:plc:removed"
	for _, did := range []string{first, second} {
		if _, err := store.RegisterJetstreamRepo(ctx, did); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.OptOutJetstreamRepo(ctx, removed); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if err := store.UpsertSave(ctx, UpsertSaveParams{
			URI:       fmt.Sprintf("at://%s/%s/%d", first, saveNSID, i),
			AuthorDID: first, ContentNSID: "is.currents.content.text", CreatedAt: &testBase,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if repos, saves, err := store.JetstreamAuditPlan(ctx, ""); err != nil || repos != 2 || saves != 2 {
		t.Fatalf("all audit scope = %d repos, %d saves, %v", repos, saves, err)
	}
	if repos, saves, err := store.JetstreamAuditPlan(ctx, first); err != nil || repos != 1 || saves != 2 {
		t.Fatalf("DID audit scope = %d repos, %d saves, %v", repos, saves, err)
	}
	if repos, _, err := store.JetstreamAuditPlan(ctx, removed); err != nil || repos != 0 {
		t.Fatalf("removed repo appeared in audit: %d, %v", repos, err)
	}
	if queued, err := store.QueueJetstreamAudit(ctx, first); err != nil || queued != 1 {
		t.Fatalf("queued one repo = %d, %v", queued, err)
	}
	if queued, err := store.QueueJetstreamAudit(ctx, ""); err != nil || queued != 2 {
		t.Fatalf("queued all active repos = %d, %v", queued, err)
	}
	var pending int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM jetstream_backfill`).Scan(&pending); err != nil || pending != 2 {
		t.Fatalf("pending audit repos = %d, %v", pending, err)
	}
	var firstFull, secondFull bool
	if err := store.pool.QueryRow(ctx, `SELECT full_scan FROM jetstream_backfill WHERE did = $1`, first).Scan(&firstFull); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT full_scan FROM jetstream_backfill WHERE did = $1`, second).Scan(&secondFull); err != nil {
		t.Fatal(err)
	}
	if !firstFull || secondFull {
		t.Fatalf("audit modes: targeted full=%t, all-only full=%t", firstFull, secondFull)
	}
}
