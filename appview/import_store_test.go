package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

const importTestSession = "11111111-1111-1111-1111-111111111111"

func importTestJob(t *testing.T, s *PgStore, target string) string {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertImportSession(ctx, importTestSession, repairTestDID, "pinterest-user"); err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateImportJob(ctx, ImportJobRow{SessionID: importTestSession, OwnerDID: repairTestDID, Source: "pinterest", SourceBoardID: "board", SourceBoardURL: "/u/b/", TargetCollectionURI: target, ExpectedCount: 300, BoardPinCount: 300})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestImportPageCheckpointIsAtomicAndCanReplay(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := importTestJob(t, s, collectionTestURI("root"))
	page := PinterestPage{Count: 250, First: true, Next: "second"}
	for i := range 250 {
		page.Pins = append(page.Pins, PinterestPin{ID: fmt.Sprint(i), ImageURL: fmt.Sprintf("https://i.pinimg.com/%d.jpg", i)})
	}
	_, err := s.pool.Exec(ctx, `CREATE FUNCTION fail_import_page() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.source_pin_id='200' THEN RAISE EXCEPTION 'simulated page failure'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER fail_import_page BEFORE INSERT ON import_item FOR EACH ROW EXECUTE FUNCTION fail_import_page()`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.pool.Exec(ctx, `DROP TRIGGER IF EXISTS fail_import_page ON import_item; DROP FUNCTION IF EXISTS fail_import_page()`)
	})
	if _, err := s.storeImportPage(ctx, id, repairTestDID, page, true); err == nil {
		t.Fatal("insertion failure ignored")
	}
	var cursor string
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT list_cursor,(SELECT count(*) FROM import_item WHERE job_id=$1) FROM import_job WHERE id=$1`, id).Scan(&cursor, &count); err != nil {
		t.Fatal(err)
	}
	if cursor != "" || count != 0 {
		t.Fatalf("partial page persisted: cursor=%q count=%d", cursor, count)
	}
	if _, err := s.pool.Exec(ctx, `DROP TRIGGER fail_import_page ON import_item`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{250, 0} {
		if n, err := s.storeImportPage(ctx, id, repairTestDID, page, true); err != nil || n != want {
			t.Fatalf("replay inserted=%d err=%v want=%d", n, err, want)
		}
	}
	if _, err := s.storeImportPage(ctx, id, repairTestDID, PinterestPage{Count: 2, Unsupported: 1, Next: "-end-", Pins: []PinterestPin{{ID: "last", ImageURL: "https://i.pinimg.com/last.jpg"}}}, true); err != nil {
		t.Fatal(err)
	}
	if err := s.finishImportListing(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	rows, err := s.GetSessionStatus(ctx, importTestSession, repairTestDID)
	if err != nil || len(rows) != 1 || rows[0].Listed == nil || *rows[0].Listed != 252 || rows[0].Unsupported != 1 || rows[0].Queued != 251 {
		t.Fatalf("status=%+v err=%v", rows, err)
	}
}

func TestImportRepairDeduplicatesHistoryAndKeepsRemovedImagesRemoved(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	target := collectionTestURI("root")
	id := importTestJob(t, s, target)
	pins := []PinterestPin{{ID: "old", ImageURL: "https://i.pinimg.com/existing.jpg"}, {ID: "failed", ImageURL: "https://i.pinimg.com/failed.jpg"}}
	if _, err := s.BulkInsertImportItems(ctx, id, repairTestDID, pins); err != nil {
		t.Fatal(err)
	}
	// No save row exists: this successful item was subsequently deleted or
	// moved. The history still excludes it from repairs.
	if _, err := s.pool.Exec(ctx, `UPDATE import_item SET status=CASE WHEN source_pin_id='old' THEN 'done' ELSE 'failed' END WHERE job_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET status='done' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if n, err := s.retryImportJobs(ctx, importTestSession, "did:plc:other", []string{id}); err != nil || n != 0 {
		t.Fatalf("foreign retry=%d %v", n, err)
	}
	if n, err := s.retryImportJobs(ctx, importTestSession, repairTestDID, []string{id}); err != nil || n != 1 {
		t.Fatalf("retry=%d %v", n, err)
	}
	page := PinterestPage{First: true, Count: 4, Next: "-end-", Pins: []PinterestPin{
		{ID: "changed-id", ImageURL: pins[0].ImageURL}, {ID: "changed-failed-id", ImageURL: pins[1].ImageURL},
		{ID: "new", ImageURL: "https://i.pinimg.com/new.jpg"}, {ID: "duplicate-new", ImageURL: "https://i.pinimg.com/new.jpg"},
	}}
	if n, err := s.storeImportPage(ctx, id, repairTestDID, page, true); err != nil || n != 1 {
		t.Fatalf("repair inserted=%d %v", n, err)
	}
	rows, err := s.GetSessionStatus(ctx, importTestSession, repairTestDID)
	if err != nil || rows[0].Done != 1 || rows[0].Queued != 2 || rows[0].PreviouslyDone != 1 {
		t.Fatalf("status=%+v err=%v", rows, err)
	}
	// A separate job for the same destination also respects the ledger.
	same := importTestJob(t, s, target)
	if n, err := s.BulkInsertImportItems(ctx, same, repairTestDID, page.Pins); err != nil || n != 0 {
		t.Fatalf("cross-job dedup=%d %v", n, err)
	}
	// Saving the same image in a different collection remains valid.
	other := importTestJob(t, s, collectionTestURI("other"))
	if n, err := s.BulkInsertImportItems(ctx, other, repairTestDID, page.Pins); err != nil || n != 3 {
		t.Fatalf("other destination=%d %v", n, err)
	}
}

func TestImportCancellationBlocksPagesAndRepairs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	target := collectionTestURI("root")
	id := importTestJob(t, s, target)
	if err := s.queueCollectionDelete(ctx, repairTestDID, target); err != nil {
		t.Fatal(err)
	}
	if err := s.cancelCollectionImports(ctx, repairTestDID, []string{target}); err != nil {
		t.Fatal(err)
	}
	_, err := s.storeImportPage(ctx, id, repairTestDID, PinterestPage{First: true, Count: 1, Pins: []PinterestPin{{ID: "new", ImageURL: "https://i.pinimg.com/new.jpg"}}}, true)
	if !errors.Is(err, errImportCancelled) {
		t.Fatalf("cancelled insert=%v", err)
	}
	if n, err := s.retryImportJobs(ctx, importTestSession, repairTestDID, []string{id}); err != nil || n != 0 {
		t.Fatalf("deleted destination retry=%d %v", n, err)
	}
}

func TestImportHistoryIsOwnedAndIncludesFinishedSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := importTestJob(t, s, collectionTestURI("root"))
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET status='done' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertImportSession(ctx, importTestSession, "did:plc:other", "attacker"); err == nil {
		t.Fatal("another user could alter this session")
	}
	if rows, err := s.listImportSessions(ctx, repairTestDID); err != nil || len(rows) != 1 || rows[0].Active || rows[0].Username != "pinterest-user" {
		t.Fatalf("history=%+v err=%v", rows, err)
	}
	if rows, err := s.listImportSessions(ctx, "did:plc:other"); err != nil || len(rows) != 0 {
		t.Fatalf("foreign history=%+v err=%v", rows, err)
	}
	if rows, err := s.GetSessionStatus(ctx, importTestSession, "did:plc:other"); err != nil || len(rows) != 0 {
		t.Fatalf("foreign status=%+v err=%v", rows, err)
	}
}
