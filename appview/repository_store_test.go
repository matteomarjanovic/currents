package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func TestDeletionResumesAfterRateLimitAndKeepsParentUntilLast(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	root, section, other := collectionTestURI("root"), collectionTestURI("section"), collectionTestURI("other")
	records := []repositoryRecord{
		testRepositoryRecord(root, `{"name":"Root"}`),
		testRepositoryRecord(section, fmt.Sprintf(`{"name":"Section","parent":{"uri":%q}}`, root)),
		testRepositoryRecord(other, `{"name":"Other"}`),
	}
	for i := 0; i < 201; i++ {
		records = append(records, testRepositoryRecord(saveTestURI(fmt.Sprint(i)), fmt.Sprintf(`{"collection":{"uri":%q}}`, section)))
	}
	p, c := newMaintenancePDS(t, records...)
	p.failBatch = 2
	// Deliberately don't index the section: deletion must discover it on PDS.
	seedCollection(t, store, root, repairTestDID, "Root", "", testBase)
	if err := store.queueCollectionDelete(ctx, repairTestDID, root); err != nil {
		t.Fatal(err)
	}
	session := "00000000-0000-0000-0000-000000000054"
	if err := store.UpsertImportSession(ctx, session, repairTestDID, ""); err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateImportJob(ctx, ImportJobRow{SessionID: session, OwnerDID: repairTestDID, TargetCollectionURI: section})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BulkInsertImportItems(ctx, job, repairTestDID, []PinterestPin{{ID: "1", ImageURL: "https://example.test/image.jpg"}}); err != nil {
		t.Fatal(err)
	}
	worker := &RepositoryMaintenance{Store: store}
	if err := worker.deleteCollectionRecords(ctx, c, repairTestDID, root); !isRateLimited(err) {
		t.Fatalf("expected rate limit: %v", err)
	}
	if _, ok := p.records[root]; !ok {
		t.Fatal("parent removed before its contents")
	}
	var pending bool
	if err := store.pool.QueryRow(ctx, `SELECT completed_at IS NULL FROM collection_delete_job WHERE collection_uri = $1`, root).Scan(&pending); err != nil || !pending {
		t.Fatalf("job lost: %v", err)
	}
	var status string
	if err := store.pool.QueryRow(ctx, `SELECT status FROM import_job WHERE id = $1`, job).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("import not cancelled: %s %v", status, err)
	}
	if n, err := store.BulkInsertImportItems(ctx, job, repairTestDID, []PinterestPin{{ID: "2"}}); err != nil || n != 0 {
		t.Fatalf("cancelled listing accepted more pins: %d %v", n, err)
	}
	if err := store.UpdateImportJobStatus(ctx, job, "running", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT status FROM import_job WHERE id = $1`, job).Scan(&status); err != nil || status != "failed" {
		t.Fatal("listing resurrected failed job")
	}
	// A new worker/process needs no in-memory progress to finish.
	worker = &RepositoryMaintenance{Store: store}
	if err := worker.deleteCollectionRecords(ctx, c, repairTestDID, root); err != nil {
		t.Fatal(err)
	}
	if len(p.records) != 1 || p.records[other].URI != other {
		t.Fatalf("remaining records: %v", p.records)
	}
	if p.calls[len(p.calls)-1] != root {
		t.Fatal("root wasn't last")
	}
	if err := store.pool.QueryRow(ctx, `SELECT completed_at IS NULL FROM collection_delete_job WHERE collection_uri = $1`, root).Scan(&pending); err != nil || pending {
		t.Fatal("completion not persisted")
	}
	cols, _, err := store.GetActorCollectionsPage(ctx, repairTestDID, "", "", 100, "")
	if err != nil || len(cols) != 0 {
		t.Fatalf("pending TAP deletion remains selectable: %v %v", cols, err)
	}
	if err := store.DeleteCollection(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := worker.deleteCollections(ctx); err != nil {
		t.Fatal(err)
	}
	if busy, err := store.repositoryBusy(ctx, repairTestDID); err != nil || busy {
		t.Fatalf("completed tombstone not released: %v %v", busy, err)
	}
}

func TestOrphanGraceDryRunAndPDSAuthority(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	missing, section := collectionTestURI("missing"), collectionTestURI("section")
	rec := testRepositoryRecord(section, fmt.Sprintf(`{"name":"Section","parent":{"uri":%q}}`, missing))
	p, c := newMaintenancePDS(t, rec)
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{DID: syntax.DID(repairTestDID), Services: map[string]identity.ServiceEndpoint{"atproto_pds": {URL: c.Host}}})
	worker := &RepositoryMaintenance{Store: store, Dir: dir}
	seedCollection(t, store, section, repairTestDID, "Section", missing, testBase)
	if _, err := store.pool.Exec(ctx, `INSERT INTO "user" (did,created_at) VALUES ($1,now())`, repairTestDID); err != nil {
		t.Fatal(err)
	}
	report, err := worker.repairOrphans(ctx, repairTestDID, true, true)
	if err != nil || report.Candidates != 1 || report.Eligible != 1 || report.Updated != 0 {
		t.Fatalf("dry run: %+v %v", report, err)
	}
	var observations int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM orphan_record`).Scan(&observations); err != nil || observations != 0 {
		t.Fatal("dry run persisted observations")
	}
	report, err = worker.repairOrphans(ctx, repairTestDID, false, false)
	if err != nil || report.Candidates != 1 || report.Eligible != 0 {
		t.Fatalf("grace: %+v %v", report, err)
	}
	first, err := store.observeOrphan(ctx, repairTestDID, section, missing, false)
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.observeOrphan(ctx, repairTestDID, section, missing, false)
	if err != nil || !first.Equal(next) {
		t.Fatal("repeat observation reset grace")
	}
	if _, err := store.pool.Exec(ctx, `UPDATE orphan_record SET first_seen_at = now() - interval '25 hours'`); err != nil {
		t.Fatal(err)
	}
	report, err = worker.repairOrphans(ctx, repairTestDID, false, false)
	if err == nil || report.Eligible != 1 || report.Updated != 0 {
		t.Fatalf("missing OAuth session must defer: %+v %v", report, err)
	}
	if len(p.calls) != 0 {
		t.Fatal("repair wrote without a session")
	}
	// Index says orphan, but the authoritative parent exists: clear observation.
	p.records[missing] = testRepositoryRecord(missing, `{"name":"Parent returned"}`)
	report, err = worker.repairOrphans(ctx, repairTestDID, false, false)
	if err != nil || report.Candidates != 0 {
		t.Fatalf("TAP lag treated as orphan: %+v %v", report, err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM orphan_record`).Scan(&observations); err != nil || observations != 0 {
		t.Fatal("resolved orphan observation remains")
	}
	// Deletions and imports must prevent even --now from rescuing their contents.
	if err := store.queueCollectionDelete(ctx, repairTestDID, missing); err != nil {
		t.Fatal(err)
	}
	report, err = worker.repairOrphans(ctx, repairTestDID, true, true)
	if err != nil || report.Accounts != 0 {
		t.Fatalf("deletion bypassed: %+v %v", report, err)
	}
}

func TestCollectionDestinationsExcludeOrphansAndPendingDeletes(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	root, section := collectionTestURI("root"), collectionTestURI("section")
	p, c := newMaintenancePDS(t, testRepositoryRecord(section, fmt.Sprintf(`{"name":"Section","parent":{"uri":%q}}`, root)))
	seedCollection(t, store, section, repairTestDID, "Section", root, testBase)
	if _, err := resolveCollectionRef(ctx, c, store, repairTestDID, section, false); err == nil {
		t.Fatal("accepted orphan section")
	}
	cols, _, err := store.GetActorCollectionsPage(ctx, repairTestDID, "", "", 100, "")
	if err != nil || len(cols) != 0 {
		t.Fatalf("orphan exposed to selector: %v %v", cols, err)
	}
	p.records[root] = testRepositoryRecord(root, `{"name":"Root"}`)
	// Newly created root is still absent in TAP: PDS validation must succeed.
	if _, err := resolveCollectionRef(ctx, c, store, repairTestDID, section, false); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveCollectionRef(ctx, c, store, repairTestDID, section, true); err == nil {
		t.Fatal("section accepted as root")
	}
	if _, err := resolveCollectionRef(ctx, c, store, "did:plc:other", section, false); err == nil {
		t.Fatal("foreign collection accepted")
	}
	if err := store.queueCollectionDelete(ctx, repairTestDID, root); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveCollectionRef(ctx, c, store, repairTestDID, section, false); err == nil {
		t.Fatal("accepted destination being deleted")
	}
}

func TestEmbeddingInvalidationOnMoveDeleteAndEmptyRecompute(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	a, b := collectionTestURI("a"), collectionTestURI("b")
	seedCollection(t, store, a, repairTestDID, "A", "", testBase)
	seedCollection(t, store, b, repairTestDID, "B", "", testBase)
	vec := make([]float32, 768)
	vec[0] = 1
	vi, err := store.CreateVI(ctx, repairTestDID, "blob", vec, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	params := UpsertSaveParams{URI: saveTestURI("moving"), AuthorDID: repairTestDID, CollectionURI: a, PdsBlobCID: "blob", ContentNSID: saveContentImageNSID, VisualIdentityID: &vi, CreatedAt: &testBase}
	if err := store.UpsertSave(ctx, params); err != nil {
		t.Fatal(err)
	}
	if err := recomputeCollectionEmbedding(ctx, store, a); err != nil {
		t.Fatal(err)
	}
	assertNull := func(uri string, want bool) {
		t.Helper()
		var got bool
		if err := store.pool.QueryRow(ctx, `SELECT canonical_embedding IS NULL FROM collection WHERE uri = $1`, uri).Scan(&got); err != nil || got != want {
			t.Fatalf("%s null=%v want %v: %v", uri, got, want, err)
		}
	}
	assertNull(a, false)
	params.CollectionURI = b
	if err := store.UpsertSave(ctx, params); err != nil {
		t.Fatal(err)
	}
	assertNull(a, true)
	if err := recomputeCollectionEmbedding(ctx, store, b); err != nil {
		t.Fatal(err)
	}
	assertNull(b, false)
	if err := store.DeleteSave(ctx, params.URI); err != nil {
		t.Fatal(err)
	}
	assertNull(b, true)
	// Historical stale state must also be cleared by an explicit recomputation.
	if err := store.UpdateCollectionEmbedding(ctx, a, vec); err != nil {
		t.Fatal(err)
	}
	if err := recomputeCollectionEmbedding(ctx, store, a); err != nil {
		t.Fatal(err)
	}
	assertNull(a, true)
	// Grace is measured from detection, never from the original save timestamp.
	first, err := store.observeOrphan(ctx, repairTestDID, params.URI, a, false)
	if err != nil || time.Since(first) > time.Minute {
		t.Fatalf("wrong observation age: %v %v", first, err)
	}
}

func TestSuggestionsRejectMissingParentEmptyAndDeletingCollections(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	valid, orphan, empty, deleting := collectionTestURI("valid"), collectionTestURI("orphan"), collectionTestURI("empty"), collectionTestURI("deleting")
	exact := make([]float32, 768)
	exact[0] = 1
	close := make([]float32, 768)
	close[0] = 0.9
	close[1] = 0.1
	vi, err := store.CreateVI(ctx, repairTestDID, "query-blob", exact, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSave(ctx, UpsertSaveParams{URI: saveTestURI("query"), AuthorDID: repairTestDID, PdsBlobCID: "query-blob", ContentNSID: saveContentImageNSID, VisualIdentityID: &vi, CreatedAt: &testBase}); err != nil {
		t.Fatal(err)
	}
	for _, uri := range []string{valid, orphan, empty, deleting} {
		parent := ""
		if uri == orphan {
			parent = collectionTestURI("gone")
		}
		seedCollection(t, store, uri, repairTestDID, rkeyFromURI(uri), parent, testBase)
		if uri != empty {
			seedImageSave(t, store, saveTestURI(rkeyFromURI(uri)), repairTestDID, uri, rkeyFromURI(uri), 1, testBase)
		}
		vec := exact
		if uri == valid {
			vec = close
		}
		if err := store.UpdateCollectionEmbedding(ctx, uri, vec); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.queueCollectionDelete(ctx, repairTestDID, deleting); err != nil {
		t.Fatal(err)
	}
	suggestions, err := store.GetSuggestedCollections(ctx, repairTestDID, []string{saveTestURI("query")})
	if err != nil || suggestions[saveTestURI("query")] != valid {
		t.Fatalf("invalid destination won recommendation: %v %v", suggestions, err)
	}
}
