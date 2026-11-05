package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/gorilla/websocket"
)

const deletedAccountDID = "did:plc:deletedaccount"

func deletedAccountDirectory() identity.Directory {
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID: syntax.DID(deletedAccountDID), Handle: syntax.HandleInvalid,
		Services: map[string]identity.ServiceEndpoint{"atproto_pds": {URL: "https://pds.example"}},
	})
	return dir
}

func stubAccountStatus(t *testing.T, relayBody, pdsBody string, pdsCode int) {
	t.Helper()
	stubBlobTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/xrpc/com.atproto.sync.getRepoStatus" || r.URL.Query().Get("did") != deletedAccountDID {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		code, body := http.StatusOK, relayBody
		if r.URL.Host == "pds.example" {
			code, body = pdsCode, pdsBody
		} else if r.URL.Host != "relay.example" {
			t.Fatalf("unexpected status host: %s", r.URL.Host)
		}
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}))
}

func TestConfirmDeletedAccountRequiresRelayAndCurrentPDS(t *testing.T) {
	deleted := `{"did":"` + deletedAccountDID + `","active":false,"status":"deleted"}`
	active := `{"did":"` + deletedAccountDID + `","active":true}`
	for _, tc := range []struct {
		name, relay, pds string
		code             int
		want, wantErr    bool
	}{
		{"missing on current PDS", deleted, `{"error":"RepoNotFound"}`, 400, true, false},
		{"explicitly deleted on PDS", deleted, deleted, 200, true, false},
		{"stale relay after migration", deleted, active, 200, false, false},
		{"temporarily deactivated", strings.ReplaceAll(deleted, "deleted\"", "deactivated\""), "", 500, false, false},
		{"active relay", active, "", 500, false, false},
		{"PDS outage", deleted, `{"error":"InternalServerError"}`, 503, false, true},
		{"unstructured 404", deleted, "not found", 404, false, true},
		{"wrong DID", strings.ReplaceAll(deleted, deletedAccountDID, "did:plc:other"), "", 500, false, true},
		{"missing active field", `{"did":"` + deletedAccountDID + `","status":"deleted"}`, "", 500, false, true},
		{"contradictory response", strings.ReplaceAll(deleted, "false", "true"), "", 500, false, true},
		{"malformed response", "{", "", 500, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubAccountStatus(t, tc.relay, tc.pds, tc.code)
			got, err := confirmDeletedAccount(context.Background(), deletedAccountDirectory(), "https://relay.example", deletedAccountDID)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("confirmed=%v err=%v; want %v error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestTapAccountDeletionKeepsOtherSaversAndBlocksReplay(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const other = "did:plc:survivor"
	uri := "at://" + deletedAccountDID + "/" + saveNSID + "/one"
	otherURI := "at://" + other + "/" + saveNSID + "/one"
	collection := "at://" + deletedAccountDID + "/" + collectionNSID + "/one"
	for _, did := range []string{deletedAccountDID, other} {
		if err := s.CreateUser(ctx, UserRecord{DID: did, Handle: "user.example.test", CreatedAt: testBase}); err != nil {
			t.Fatal(err)
		}
	}
	seedCollection(t, s, collection, deletedAccountDID, "Home", "", testBase)
	seedImageSave(t, s, uri, deletedAccountDID, collection, "same-blob", 1, testBase)
	seedImageSave(t, s, otherURI, other, "", "same-blob", 1, testBase)
	var vi string
	if err := s.pool.QueryRow(ctx, `INSERT INTO visual_identity (canonical_blob_did, canonical_blob_cid, canonical_save_uri)
		VALUES ($1, 'same-blob', $2) RETURNING id`, deletedAccountDID, uri).Scan(&vi); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE save SET visual_identity_id = $1`, vi); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertFollow(ctx, "at://"+other+"/"+followNSID+"/one", other, deletedAccountDID); err != nil {
		t.Fatal(err)
	}
	session := "00000000-0000-0000-0000-000000000063"
	if err := s.UpsertImportSession(ctx, session, deletedAccountDID, ""); err != nil {
		t.Fatal(err)
	}
	job, err := s.CreateImportJob(ctx, ImportJobRow{SessionID: session, OwnerDID: deletedAccountDID, TargetCollectionURI: collection})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BulkInsertImportItems(ctx, job, deletedAccountDID, []PinterestPin{{ID: "one", ImageURL: "https://example.test/one.jpg"}}); err != nil {
		t.Fatal(err)
	}
	h := &TapHandler{Store: s, Dir: deletedAccountDirectory()}
	for _, status := range []string{"deactivated", "suspended", "takendown", ""} {
		if err := handleTapIdentity(ctx, h, &TapIdentityEvent{DID: deletedAccountDID, Handle: "handle.invalid", Status: status}); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM save WHERE author_did = $1`, deletedAccountDID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("status %q deleted saves: %d %v", status, n, err)
		}
	}
	deletion := &TapIdentityEvent{DID: deletedAccountDID, Handle: "handle.invalid", Status: "deleted"}
	for i := 0; i < 2; i++ { // Redelivery after a lost ack is harmless.
		if err := handleTapIdentity(ctx, h, deletion); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`SELECT count(*) FROM save WHERE author_did = $1`,
		`SELECT count(*) FROM collection WHERE author_did = $1`,
		`SELECT count(*) FROM "user" WHERE did = $1`,
		`SELECT count(*) FROM import_item WHERE owner_did = $1 AND status IN ('queued', 'running')`,
		`SELECT count(*) FROM import_job WHERE owner_did = $1 AND status IN ('listing', 'running')`,
	} {
		var n int
		if err := s.pool.QueryRow(ctx, query, deletedAccountDID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("deleted account rows remain: %s count=%d err=%v", query, n, err)
		}
	}
	var canonical string
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT canonical_save_uri, save_count FROM visual_identity WHERE id = $1`, vi).Scan(&canonical, &count); err != nil || canonical != otherURI || count != 1 {
		t.Fatalf("surviving saver lost: canonical=%s count=%d err=%v", canonical, count, err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM follow WHERE subject_did = $1`, deletedAccountDID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("another user's follow was removed: %d %v", count, err)
	}
	replay := &TapRecordEvent{DID: deletedAccountDID, Collection: collectionNSID, Rkey: "late", Action: "create", Record: json.RawMessage(`{"name":"Must not return"}`)}
	if err := applyTapRecord(ctx, h, replay); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE author_did = $1`, deletedAccountDID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("buffered record restored account: %d %v", count, err)
	}
	// An old buffered active event must not undo a historical deletion audit.
	deleted := fmt.Sprintf(`{"did":%q,"active":false,"status":"deleted"}`, deletedAccountDID)
	stubAccountStatus(t, deleted, `{"error":"RepoNotFound"}`, 400)
	activeEvent := &TapIdentityEvent{DID: deletedAccountDID, IsActive: true, Status: "active", Handle: "handle.invalid"}
	if err := handleTapIdentity(ctx, h, activeEvent); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tap_deleted_repo`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("buffered active event removed deletion marker: %d %v", count, err)
	}
	// An active status confirmed by the current PDS permits fresh TAP data again.
	active := fmt.Sprintf(`{"did":%q,"active":true}`, deletedAccountDID)
	stubAccountStatus(t, active, active, 200)
	if err := handleTapIdentity(ctx, h, &TapIdentityEvent{DID: deletedAccountDID, IsActive: true, Status: "active", Handle: "handle.invalid"}); err != nil {
		t.Fatal(err)
	}
	if err := applyTapRecord(ctx, h, replay); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE author_did = $1`, deletedAccountDID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("active status did not permit new records: %d %v", count, err)
	}
}

func TestPurgeDeletedAccountsPreviewAndRetry(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// No user row: historical cleanup must also discover orphaned index owners.
	collection := "at://" + deletedAccountDID + "/" + collectionNSID + "/one"
	seedCollection(t, s, collection, deletedAccountDID, "Home", "", testBase)
	seedImageSave(t, s, "at://"+deletedAccountDID+"/"+saveNSID+"/one", deletedAccountDID, collection, "blob", 1, testBase)
	deleted := fmt.Sprintf(`{"did":%q,"active":false,"status":"deleted"}`, deletedAccountDID)
	stubAccountStatus(t, deleted, `{"error":"RepoNotFound"}`, 400)
	audit := func(apply bool) error {
		return purgeDeletedAccounts(ctx, s, deletedAccountDirectory(), "https://relay.example", "", apply)
	}
	if err := audit(false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM save`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("preview changed saves: %d %v", count, err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tap_deleted_repo`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("preview wrote a marker: %d %v", count, err)
	}
	// Simulate cleanup failure after saving the marker. A rerun must finish.
	if _, err := s.pool.Exec(ctx, `CREATE FUNCTION fail_account_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'test cleanup failure'; END $$;
		CREATE TRIGGER fail_account_cleanup BEFORE DELETE ON collection FOR EACH ROW EXECUTE FUNCTION fail_account_cleanup()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DROP TRIGGER IF EXISTS fail_account_cleanup ON collection; DROP FUNCTION IF EXISTS fail_account_cleanup()`)
	})
	if err := audit(true); err == nil {
		t.Fatal("cleanup failure was swallowed")
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tap_deleted_repo`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("partial cleanup lost its marker: %d %v", count, err)
	}
	if _, err := s.pool.Exec(ctx, `DROP TRIGGER fail_account_cleanup ON collection; DROP FUNCTION fail_account_cleanup()`); err != nil {
		t.Fatal(err)
	}
	if err := audit(true); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM collection`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("retry did not finish: %d %v", count, err)
	}
}

func TestTapDeletionAcknowledgedOnlyAfterCleanup(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	seedCollection(t, s, "at://"+deletedAccountDID+"/"+collectionNSID+"/one", deletedAccountDID, "Home", "", testBase)
	if _, err := s.pool.Exec(ctx, `CREATE FUNCTION fail_tap_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'test cleanup failure'; END $$;
		CREATE TRIGGER fail_tap_cleanup BEFORE DELETE ON collection FOR EACH ROW EXECUTE FUNCTION fail_tap_cleanup()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS fail_tap_cleanup ON collection; DROP FUNCTION IF EXISTS fail_tap_cleanup()`)
	})
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connections <- conn
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	remote := <-connections
	defer remote.Close()
	done := make(chan struct{})
	go func() {
		handleTapConn(ctx, client, &TapHandler{Store: s})
		close(done)
	}()
	defer func() { cancel(); <-done }()
	deletion := TapEvent{ID: 1, Type: "identity", Identity: &TapIdentityEvent{DID: deletedAccountDID, Status: "deleted"}}
	if err := remote.WriteJSON(deletion); err != nil {
		t.Fatal(err)
	}
	// A following harmless event lets us assert which ack arrives without
	// timing out the WebSocket (which would make subsequent reads invalid).
	if err := remote.WriteJSON(TapEvent{ID: 2, Type: "identity", Identity: &TapIdentityEvent{Handle: "handle.invalid"}}); err != nil {
		t.Fatal(err)
	}
	_ = remote.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ack TapAck
	if err := remote.ReadJSON(&ack); err != nil || ack.Type != "ack" || ack.ID != 2 {
		t.Fatalf("failed deletion was acknowledged: %+v, %v", ack, err)
	}
	if _, err := s.pool.Exec(ctx, `DROP TRIGGER fail_tap_cleanup ON collection; DROP FUNCTION fail_tap_cleanup()`); err != nil {
		t.Fatal(err)
	}
	if err := remote.WriteJSON(deletion); err != nil {
		t.Fatal(err)
	}
	if err := remote.ReadJSON(&ack); err != nil || ack.Type != "ack" || ack.ID != 1 {
		t.Fatalf("successful retry was not acknowledged: %+v, %v", ack, err)
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM collection WHERE author_did = $1`, deletedAccountDID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("ack preceded cleanup: %d %v", count, err)
	}
}
