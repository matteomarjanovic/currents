package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func TestPinterestRepairPlanOnlyIncludesLatestLegacyPreview(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	old := importTestJob(t, s, collectionTestURI("old"))
	latest := importTestJob(t, s, collectionTestURI("latest"))
	other := importTestJob(t, s, collectionTestURI("other"))
	checked := importTestJob(t, s, collectionTestURI("checked"))
	small := importTestJob(t, s, collectionTestURI("small"))
	const newestSession = "33333333-3333-3333-3333-333333333333"
	if err := s.UpsertImportSession(ctx, newestSession, repairTestDID, "pinterest-user"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{old, latest, other, checked, small} {
		if _, err := s.pool.Exec(ctx, `UPDATE import_job SET status='done' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET created_at=now()-interval '1 day' WHERE id=$1`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET session_id=$2, created_at=now() WHERE id=$1`, latest, newestSession); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET source_board_id='other' WHERE id=$1`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET source_board_id='checked', listed_count=24 WHERE id=$1`, checked); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET source_board_id='small', expected_count=25 WHERE id=$1`, small); err != nil {
		t.Fatal(err)
	}
	plan, err := s.pinterestRepairCandidates(ctx, repairTestDID)
	if err != nil || len(plan) != 2 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	for _, c := range plan {
		if c.ID != latest && c.ID != other {
			t.Fatalf("unexpected candidate %+v", c)
		}
	}
	// A newer sectioned import replaces the flat-board source too. Its unknown
	// subset count must not cause the old flat destination to be replayed.
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET filter_section_pins=true, expected_count=0 WHERE id=$1`, latest); err != nil {
		t.Fatal(err)
	}
	plan, err = s.pinterestRepairCandidates(ctx, repairTestDID)
	if err != nil || len(plan) != 1 || plan[0].ID != other {
		t.Fatalf("sectioned plan=%+v err=%v", plan, err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO import_item(job_id,owner_did,source_pin_id,image_url,rkey)
		SELECT $1,$2,n::text,'https://i.pinimg.com/'||n||'.jpg','key'||n FROM generate_series(1,26)n`, other, repairTestDID); err != nil {
		t.Fatal(err)
	}
	plan, err = s.pinterestRepairCandidates(ctx, repairTestDID)
	if err != nil || len(plan) != 0 {
		t.Fatalf("healthy first-page plan=%+v err=%v", plan, err)
	}
	plan, err = s.pinterestRepairCandidates(ctx, "did:plc:other")
	if err != nil || len(plan) != 0 {
		t.Fatalf("foreign plan=%+v err=%v", plan, err)
	}
}

func TestPinterestOperatorRepairDryRunAndApply(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	root := collectionTestURI("root")
	removed := collectionTestURI("removed")
	good := importTestJob(t, s, root)
	missing := importTestJob(t, s, removed)
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET status='done', source_board_id=id::text WHERE session_id=$1`, importTestSession); err != nil {
		t.Fatal(err)
	}
	pds, client := newMaintenancePDS(t, testRepositoryRecord(root, `{"name":"root"}`))
	stubBlobTransport(t, http.DefaultTransport)
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSession(ctx, oauth.ClientSessionData{AccountDID: syntax.DID(repairTestDID), SessionID: "test", HostURL: client.Host, AccessToken: "test-token", DPoPPrivateKeyMultibase: key.Multibase()}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM oauth_sessions WHERE account_did=$1`, repairTestDID) })
	config := oauth.NewPublicConfig("https://currents.test/oauth-client-metadata.json", "https://currents.test/oauth/callback", []string{"atproto"})
	w := &ImportWorker{Store: s, OAuth: oauth.NewClientApp(&config, s)}
	if _, err := w.repairPinterestImports(ctx, "", false); err == nil {
		t.Fatal("unscoped apply accepted")
	}
	plan, err := w.repairPinterestImports(ctx, "", true)
	if err != nil || len(plan.Candidates) != 2 || plan.Queued != 0 {
		t.Fatalf("global plan=%+v err=%v", plan, err)
	}
	preview, err := w.repairPinterestImports(ctx, repairTestDID, true)
	if err != nil || preview.Eligible != 1 || preview.Queued != 0 || preview.SkippedDestinations != 1 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT status FROM import_job WHERE id=$1`, good).Scan(&status); err != nil || status != "done" {
		t.Fatalf("dry run wrote status=%q err=%v", status, err)
	}
	report, err := w.repairPinterestImports(ctx, repairTestDID, false)
	if err != nil || report.Queued != 1 || report.SkippedDestinations != 1 {
		t.Fatalf("apply=%+v err=%v", report, err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT status FROM import_job WHERE id=$1`, missing).Scan(&status); err != nil || status != "done" {
		t.Fatalf("missing target status=%q err=%v", status, err)
	}
	report, err = w.repairPinterestImports(ctx, repairTestDID, false)
	if err != nil || !report.Busy || report.Queued != 0 {
		t.Fatalf("busy account=%+v err=%v", report, err)
	}
	for _, call := range pds.calls {
		if call != "getRecord" {
			t.Fatalf("operator command made a PDS write: %s", call)
		}
	}
	// A completed repair is not selected again, even if Pinterest still omits
	// pins. This bounds the incident repair independently of source omissions.
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET status='done',listed_count=24 WHERE id=ANY($1)`, []string{good, missing}); err != nil {
		t.Fatal(err)
	}
	plan, err = w.repairPinterestImports(ctx, repairTestDID, true)
	if err != nil || len(plan.Candidates) != 0 {
		t.Fatalf("repeat plan=%+v err=%v", plan, err)
	}
}

func TestPinterestOperatorRepairRequiresReconnect(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := importTestJob(t, s, collectionTestURI("root"))
	if _, err := s.pool.Exec(ctx, `UPDATE import_job SET status='done' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM oauth_sessions WHERE account_did=$1`, repairTestDID); err != nil {
		t.Fatal(err)
	}
	w := &ImportWorker{Store: s}
	report, err := w.repairPinterestImports(ctx, repairTestDID, false)
	if err != nil || !report.ReconnectRequired || report.Queued != 0 {
		t.Fatalf("reconnect=%+v err=%v", report, err)
	}
}
