package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/jetstream"
)

func TestJetstreamEnrollmentOptOutAndReplay(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:jetstream-test"
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{DID: syntax.DID(did), Handle: syntax.Handle("new.example.test")})
	handler := &TapHandler{Store: store, Dir: dir, IngestSource: "jetstream"}
	profile := jetstream.Event{DID: did, Kind: jetstream.KindCommit, Commit: &jetstream.Commit{
		Operation: jetstream.OpCreate, Collection: currentsProfileNSID, Rkey: "self", Rev: "rev-1",
		Record: map[string]any{"displayName": "Alice", "createdAt": "2026-01-01T00:00:00Z"},
	}}
	if err := applyJetstreamEvent(ctx, handler, profile); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.JetstreamRepoState(ctx, did)
	if err != nil || state != "active" {
		t.Fatalf("profile signal state = %q, %v", state, err)
	}
	if pending, err := store.NextJetstreamBackfill(ctx); err != nil || pending.DID != did || !pending.Full {
		t.Fatalf("profile signal backfill = %+v, %v", pending, err)
	}
	actor, err := store.GetActorByDID(ctx, did)
	if err != nil || actor == nil || actor.Handle != "new.example.test" {
		t.Fatalf("profile = %+v, %v", actor, err)
	}

	collection := jetstream.Event{DID: did, Kind: jetstream.KindCommit, Commit: &jetstream.Commit{
		Operation: jetstream.OpCreate, Collection: collectionNSID, Rkey: "one", CID: "cid-one", Rev: "rev-2",
		Record: map[string]any{"name": "One", "createdAt": "2026-01-01T00:00:00Z"},
	}}
	if err := applyJetstreamEvent(ctx, handler, collection); err != nil {
		t.Fatal(err)
	}
	if err := store.SetJetstreamReconciledRev(ctx, did, "rev-2"); err != nil {
		t.Fatal(err)
	}
	collection.Commit.Record["name"] = "Stale"
	collection.Commit.Rev = "rev-1"
	if err := applyJetstreamEvent(ctx, handler, collection); err != nil {
		t.Fatal(err)
	}
	var name string
	uri := "at://" + did + "/" + collectionNSID + "/one"
	if err := store.pool.QueryRow(ctx, `SELECT name FROM collection WHERE uri = $1`, uri).Scan(&name); err != nil || name != "One" {
		t.Fatalf("older replay changed collection: %q, %v", name, err)
	}

	if err := store.OptOutJetstreamRepo(ctx, did); err != nil {
		t.Fatal(err)
	}
	collection.Commit.Rev = "rev-3"
	collection.Commit.Record["name"] = "Should not return"
	if err := applyJetstreamEvent(ctx, handler, collection); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT name FROM collection WHERE uri = $1`, uri).Scan(&name); err != nil || name != "One" {
		t.Fatalf("opted-out repo changed: %q, %v", name, err)
	}
	if err := store.EnableJetstreamRepo(ctx, did); err != nil {
		t.Fatal(err)
	}
	if err := applyJetstreamEvent(ctx, handler, collection); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT name FROM collection WHERE uri = $1`, uri).Scan(&name); err != nil || name != "Should not return" {
		t.Fatalf("re-enabled repo = %q, %v", name, err)
	}
	if err := store.AdvanceJetstreamCursor(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if err := store.AdvanceJetstreamCursor(ctx, 90); err != nil {
		t.Fatal(err)
	}
	if cursor, err := store.JetstreamCursor(ctx); err != nil || cursor != 100 {
		t.Fatalf("cursor = %d, %v", cursor, err)
	}
}

func TestJetstreamSyncAndAccountMarkers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:jetstream-test"
	const follower = "did:plc:follower"
	if err := store.CreateUser(ctx, UserRecord{DID: did, Handle: "user.example.test", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RegisterJetstreamRepo(ctx, did); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCollection(ctx, "at://"+did+"/"+collectionNSID+"/one", "cid-one", did, "One", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertFollow(ctx, "at://"+follower+"/"+followNSID+"/one", follower, did); err != nil {
		t.Fatal(err)
	}
	handler := &TapHandler{Store: store, IngestSource: "jetstream"}
	syncEvent := jetstream.Event{DID: did, Kind: jetstream.KindSync, Sync: &jetstream.Sync{Rev: "rev-2"}}
	if err := applyJetstreamEvent(ctx, handler, syncEvent); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE author_did = $1`, did).Scan(&count); err != nil || count != 0 {
		t.Fatalf("sync left %d collections: %v", count, err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM follow WHERE subject_did = $1`, did).Scan(&count); err != nil || count != 1 {
		t.Fatalf("sync lost follows of actor: %d, %v", count, err)
	}
	if pending, err := store.NextJetstreamBackfill(ctx); err != nil || pending.DID != did || !pending.Full {
		t.Fatalf("sync backfill = %+v, %v", pending, err)
	}
	if err := applyJetstreamEvent(ctx, handler, jetstream.Event{DID: did, Kind: jetstream.KindAccount, Account: &jetstream.Account{Active: false, Status: "deleted"}}); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.JetstreamRepoState(ctx, did)
	if err != nil || state != "inactive" {
		t.Fatalf("deleted account state = %q, %v", state, err)
	}
	if err := applyJetstreamEvent(ctx, handler, jetstream.Event{DID: did, Kind: jetstream.KindAccount, Account: &jetstream.Account{Active: true}}); err != nil {
		t.Fatal(err)
	}
	state, _, err = store.JetstreamRepoState(ctx, did)
	if err != nil || state != "active" {
		t.Fatalf("reactivated account state = %q, %v", state, err)
	}
}

func TestRepoTrackingKeepsDeletionOptOut(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:jetstream-test"
	s := &Server{Store: store, IngestSource: "jetstream"}
	if err := s.tapRepos(ctx, "remove", did); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.JetstreamRepoState(ctx, did)
	if err != nil || state != "opted_out" {
		t.Fatalf("deleted account state = %q, %v", state, err)
	}
	if inserted, err := store.RegisterJetstreamRepo(ctx, did); err != nil || inserted {
		t.Fatalf("signal revived opted-out account: %t, %v", inserted, err)
	}
	if err := s.tapRepos(ctx, "add", did); err != nil {
		t.Fatal(err)
	}
	state, _, err = store.JetstreamRepoState(ctx, did)
	if err != nil || state != "active" {
		t.Fatalf("login state = %q, %v", state, err)
	}
	if pending, err := store.NextJetstreamBackfill(ctx); err != nil || pending.DID != did || !pending.Full {
		t.Fatalf("login backfill = %+v, %v", pending, err)
	}

	var actions []string
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DIDs []string `json:"dids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.DIDs) != 1 || body.DIDs[0] != did {
			t.Errorf("TAP admin body = %+v, %v", body, err)
		}
		actions = append(actions, r.URL.Path)
	}))
	defer admin.Close()
	s.IngestSource = "tap"
	s.TapAdminURL = admin.URL
	if err := s.tapRepos(ctx, "remove", did); err != nil {
		t.Fatal(err)
	}
	if err := s.tapRepos(ctx, "add", did); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 || actions[0] != "/repos/remove" || actions[1] != "/repos/add" {
		t.Fatalf("TAP admin actions = %v", actions)
	}
}

func TestJetstreamAccountDeletionAndReloginRestoresPDS(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = repairTestDID
	collectionURI := collectionTestURI("root")
	saveURI := saveTestURI("one")
	_, client := newMaintenancePDS(t,
		testRepositoryRecord("at://"+did+"/"+currentsProfileNSID+"/self", `{"displayName":"Alice"}`),
		testRepositoryRecord(collectionURI, `{"name":"Root"}`),
		testRepositoryRecord(saveURI, `{"collection":{"uri":"`+collectionURI+`"},"content":{"$type":"is.currents.content.text"},"text":"Kept on PDS"}`),
	)
	handler := jetstreamTestHandler(store, client.Host)
	s := &Server{Store: store, IngestSource: "jetstream"}
	if err := s.tapRepos(ctx, "add", did); err != nil {
		t.Fatal(err)
	}
	if err := reconcileJetstreamRepo(ctx, handler, did, true); err != nil {
		t.Fatal(err)
	}
	if err := s.tapRepos(ctx, "remove", did); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteUserData(ctx, did, ""); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.JetstreamRepoState(ctx, did)
	if err != nil || state != "opted_out" {
		t.Fatalf("deleted account tracking = %q, %v", state, err)
	}
	if err := applyJetstreamEvent(ctx, handler, jetstream.Event{DID: did, Kind: jetstream.KindCommit, Commit: &jetstream.Commit{
		Operation: jetstream.OpCreate, Collection: collectionNSID, Rkey: "new", Rev: "rev-new",
		Record: map[string]any{"name": "Must stay hidden"},
	}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE author_did = $1`, did).Scan(&count); err != nil || count != 0 {
		t.Fatalf("opted-out repo was reindexed: %d, %v", count, err)
	}
	if err := s.tapRepos(ctx, "add", did); err != nil {
		t.Fatal(err)
	}
	job, err := store.NextJetstreamBackfill(ctx)
	if err != nil || job.DID != did || !job.Full {
		t.Fatalf("relogin backfill = %+v, %v", job, err)
	}
	if err := reconcileJetstreamRepo(ctx, handler, did, job.Full); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM save WHERE uri = $1`, saveURI).Scan(&count); err != nil || count != 1 {
		t.Fatalf("PDS save not restored on relogin: %d, %v", count, err)
	}
	if actor, err := store.GetActorByDID(ctx, did); err != nil || actor == nil || actor.DisplayName != "Alice" {
		t.Fatalf("PDS profile not restored on relogin: %+v, %v", actor, err)
	}
}

func TestJetstreamSyncDivergenceReplacesRepoAndSkipsOldCommits(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = repairTestDID
	oldCollection := collectionTestURI("old")
	newCollection := collectionTestURI("new")
	oldSave := saveTestURI("old")
	newSave := saveTestURI("new")
	p, client := newMaintenancePDS(t,
		testRepositoryRecord("at://"+did+"/"+currentsProfileNSID+"/self", `{"displayName":"Alice"}`),
		testRepositoryRecord(oldCollection, `{"name":"Old"}`),
		testRepositoryRecord(oldSave, fmt.Sprintf(`{"collection":{"uri":%q},"content":{"$type":"is.currents.content.text"}}`, oldCollection)),
	)
	handler := jetstreamTestHandler(store, client.Host)
	if _, err := store.RegisterJetstreamRepo(ctx, did); err != nil {
		t.Fatal(err)
	}
	if err := reconcileJetstreamRepo(ctx, handler, did, true); err != nil {
		t.Fatal(err)
	}
	delete(p.records, oldCollection)
	delete(p.records, oldSave)
	p.records[newCollection] = testRepositoryRecord(newCollection, `{"name":"New"}`)
	p.records[newSave] = testRepositoryRecord(newSave, fmt.Sprintf(`{"collection":{"uri":%q},"content":{"$type":"is.currents.content.text"}}`, newCollection))
	p.revision++
	if err := applyJetstreamEvent(ctx, handler, jetstream.Event{DID: did, Kind: jetstream.KindSync, Sync: &jetstream.Sync{Rev: "rev-1"}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM save WHERE author_did = $1`, did).Scan(&count); err != nil || count != 0 {
		t.Fatalf("sync did not clear old saves: %d, %v", count, err)
	}
	job, err := store.NextJetstreamBackfill(ctx)
	if err != nil || job.DID != did || !job.Full {
		t.Fatalf("sync backfill = %+v, %v", job, err)
	}
	if err := reconcileJetstreamRepo(ctx, handler, did, job.Full); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		uri  string
		want int
	}{{oldSave, 0}, {newSave, 1}} {
		if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM save WHERE uri = $1`, tc.uri).Scan(&count); err != nil || count != tc.want {
			t.Fatalf("save %s after sync = %d, %v", tc.uri, count, err)
		}
	}
	if err := applyJetstreamEvent(ctx, handler, jetstream.Event{DID: did, Kind: jetstream.KindCommit, Commit: &jetstream.Commit{
		Operation: jetstream.OpCreate, Collection: collectionNSID, Rkey: "old", Rev: "rev-0",
		Record: map[string]any{"name": "Obsolete"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE uri = $1`, oldCollection).Scan(&count); err != nil || count != 0 {
		t.Fatalf("old replay resurrected collection: %d, %v", count, err)
	}
	if err := applyJetstreamEvent(ctx, handler, jetstream.Event{DID: did, Kind: jetstream.KindCommit, Commit: &jetstream.Commit{
		Operation: jetstream.OpCreate, Collection: collectionNSID, Rkey: "later", Rev: "rev-2",
		Record: map[string]any{"name": "Later"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE uri = $1`, collectionTestURI("later")).Scan(&count); err != nil || count != 1 {
		t.Fatalf("newer commit was skipped: %d, %v", count, err)
	}
}
