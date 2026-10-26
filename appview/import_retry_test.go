package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/gorilla/sessions"
)

func TestImportRetryChecksOwnedSessionAndAuthoritativeDestinations(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	root, removed := collectionTestURI("root"), collectionTestURI("removed")
	good := importTestJob(t, store, root)
	deleted := importTestJob(t, store, removed)
	if _, err := store.pool.Exec(ctx, `UPDATE import_job SET status='done' WHERE session_id=$1`, importTestSession); err != nil {
		t.Fatal(err)
	}
	// The live destination is deliberately absent from the index. The removed
	// one is still indexed, illustrating why repair must consult the PDS.
	if err := store.UpsertCollection(ctx, removed, "cid", repairTestDID, "stale", "", "", nil); err != nil {
		t.Fatal(err)
	}
	pds, client := newMaintenancePDS(t, testRepositoryRecord(root, `{"name":"root"}`))
	stubBlobTransport(t, http.DefaultTransport)
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	mem := oauth.NewMemStore()
	for _, did := range []string{repairTestDID, "did:plc:other"} {
		if err := mem.SaveSession(ctx, oauth.ClientSessionData{AccountDID: syntax.DID(did), SessionID: "test", HostURL: client.Host, AccessToken: "test-token", DPoPPrivateKeyMultibase: key.Multibase()}); err != nil {
			t.Fatal(err)
		}
	}
	config := oauth.NewPublicConfig("https://currents.test/oauth-client-metadata.json", "https://currents.test/oauth/callback", []string{"atproto"})
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	s := &Server{Store: store, CookieStore: sessions.NewCookieStore([]byte("test-secret")), OAuth: oauth.NewClientApp(&config, mem), ImportWorker: &ImportWorker{Context: stopped, drains: map[string]chan struct{}{}}}
	call := func(did string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/import/sessions/"+importTestSession+"/retry", strings.NewReader(`{}`))
		req.SetPathValue("id", importTestSession)
		if did != "" {
			token, err := s.encodeSessionToken(did, "test", "test.example")
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		s.APIRetryImportSession(res, req)
		return res
	}
	if res := call(""); res.Code != 401 {
		t.Fatalf("anonymous=%d %s", res.Code, res.Body)
	}
	if res := call("did:plc:other"); res.Code != 404 {
		t.Fatalf("foreign=%d %s", res.Code, res.Body)
	}
	res := call(repairTestDID)
	var result struct{ Retried, Skipped int }
	json.Unmarshal(res.Body.Bytes(), &result)
	if res.Code != 200 || result.Retried != 1 || result.Skipped != 1 {
		t.Fatalf("retry=%d %s", res.Code, res.Body)
	}
	var liveState, removedState string
	store.pool.QueryRow(ctx, `SELECT status FROM import_job WHERE id=$1`, good).Scan(&liveState)
	store.pool.QueryRow(ctx, `SELECT status FROM import_job WHERE id=$1`, deleted).Scan(&removedState)
	if liveState != "listing" || removedState != "done" {
		t.Fatalf("live=%q removed=%q", liveState, removedState)
	}
	for _, call := range pds.calls {
		if strings.Contains(call, "putRecord") || strings.Contains(call, "applyWrites") {
			t.Fatalf("repair handler wrote PDS data: %s", call)
		}
	}
}
