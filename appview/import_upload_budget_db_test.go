package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func readUploadBudget(t *testing.T, s *PgStore, did, pds string) importUploadBudget {
	t.Helper()
	var raw []byte
	if err := s.pool.QueryRow(context.Background(), `SELECT state FROM import_upload_budget WHERE owner_did=$1 AND pds_url=$2`, did, pds).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var b importUploadBudget
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func seedUploadBudget(t *testing.T, s *PgStore, remaining int, left time.Duration) {
	t.Helper()
	if err := s.changeImportUploadBudget(context.Background(), repairTestDID, budgetTestPDS, func(b *importUploadBudget) {
		*b = importUploadBudget{Window: &importBlobWindow{Remaining: remaining, Reset: time.Now().Add(left), PDSReset: time.Now().Add(left).Unix()}}
	}); err != nil {
		t.Fatal(err)
	}
}

func uploadBudgetClient(s *PgStore, pds string, importing bool, transport http.RoundTripper) *atclient.APIClient {
	c := atclient.NewAPIClient(pds)
	c.Client = &http.Client{Transport: transport}
	return withUploadBudget(c, s, repairTestDID, importing)
}

func budgetUpload(c *atclient.APIClient) error {
	var out map[string]any
	return c.LexDo(context.Background(), "POST", "image/png", "com.atproto.repo.uploadBlob", nil, strings.NewReader("image"), &out)
}

func budgetResponse(status int, h http.Header, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func TestImportUploadBudgetSurvivesRestartAndMigration(t *testing.T) {
	s := newTestStore(t)
	seedUploadBudget(t, s, 50, 4*time.Minute)
	// A fresh store/client sees the persisted reserve, not an empty allowance.
	restarted := &PgStore{pool: s.pool, cfg: s.cfg}
	if wait, err := restarted.importUploadWait(context.Background(), repairTestDID, budgetTestPDS); err != nil || wait < 3*time.Minute {
		t.Fatalf("restart lost reserve: wait=%s err=%v", wait, err)
	}
	for _, pds := range []string{"https://eurosky.social", "https://discina.us-west.host.bsky.network"} {
		if wait, err := restarted.importUploadWait(context.Background(), repairTestDID, pds); err != nil || wait != 0 {
			t.Fatalf("old PDS budget leaked to %s: wait=%s err=%v", pds, wait, err)
		}
	}
	if err := s.DeleteUserData(context.Background(), repairTestDID, ""); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.pool.QueryRow(context.Background(), `SELECT count(*) FROM import_upload_budget WHERE owner_did=$1`, repairTestDID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted account retained budget: %d %v", count, err)
	}
}

func TestImportUploadBudgetSerializesConcurrentAttempts(t *testing.T) {
	s := newTestStore(t)
	seedUploadBudget(t, s, 51, 4*time.Minute)
	var sent atomic.Int32
	transport := blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		sent.Add(1)
		return budgetResponse(200, budgetHeaders(time.Now(), 50, 4*time.Minute), `{"ok":true}`), nil
	})
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Separate clients model separate workers/processes sharing the DB.
			errs <- budgetUpload(uploadBudgetClient(s, budgetTestPDS, true, transport))
		}()
	}
	wg.Wait()
	close(errs)
	var successes int
	for err := range errs {
		if err == nil {
			successes++
		} else {
			var waiting *importUploadWaitError
			if !errors.As(err, &waiting) {
				t.Fatalf("unexpected error: %v", err)
			}
		}
	}
	if successes != 1 || sent.Load() != 1 || readUploadBudget(t, s, repairTestDID, budgetTestPDS).Window.Remaining != 50 {
		t.Fatalf("reserve overspent: successes=%d sent=%d", successes, sent.Load())
	}
}

func TestImportUploadBudgetManualActivityWins(t *testing.T) {
	s := newTestStore(t)
	seedUploadBudget(t, s, 900, time.Hour)
	ctx := context.Background()
	// The worker has already checked capacity while preparing its image.
	if wait, err := s.importUploadWait(ctx, repairTestDID, budgetTestPDS); wait != 0 || err != nil {
		t.Fatalf("unexpected initial wait: %s %v", wait, err)
	}
	var calls atomic.Int32
	transport := blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return budgetResponse(200, http.Header{}, `{"token":"test"}`), nil
	})
	manual := uploadBudgetClient(s, budgetTestPDS, false, transport)
	var token map[string]string
	if err := manual.Get(ctx, "com.atproto.server.getServiceAuth", map[string]any{"lxm": "com.atproto.repo.uploadBlob"}, &token); err != nil {
		t.Fatal(err)
	}
	err := budgetUpload(uploadBudgetClient(s, budgetTestPDS, true, transport))
	var waiting *importUploadWaitError
	if !errors.As(err, &waiting) || waiting.delay < time.Minute || calls.Load() != 1 {
		t.Fatalf("import raced manual token: err=%v calls=%d", err, calls.Load())
	}
	b := readUploadBudget(t, s, repairTestDID, budgetTestPDS)
	if b.Window.Remaining != 899 {
		t.Fatalf("direct upload was not reserved: %+v", b)
	}
	// Manual uploads may use the final reserve; the guard is import-only.
	seedUploadBudget(t, s, 50, 4*time.Minute)
	if err := budgetUpload(manual); err != nil {
		t.Fatalf("manual upload blocked: %v", err)
	}
	if got := readUploadBudget(t, s, repairTestDID, budgetTestPDS).Window.Remaining; got != 49 {
		t.Fatalf("manual usage not charged: %d", got)
	}
}

func TestImportUploadBudgetTokenAuthRetryReservesOnce(t *testing.T) {
	s := newTestStore(t)
	seedUploadBudget(t, s, 900, time.Hour)
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	transport := blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			h := make(http.Header)
			h.Set("WWW-Authenticate", `DPoP error="use_dpop_nonce"`)
			h.Set("DPoP-Nonce", "test-nonce")
			return budgetResponse(401, h, `{"error":"use_dpop_nonce"}`), nil
		}
		return budgetResponse(200, http.Header{}, `{"token":"test"}`), nil
	})
	config := oauth.NewPublicConfig("https://currents.test/client.json", "https://currents.test/callback", []string{"atproto"})
	sess := &oauth.ClientSession{Config: &config, DPoPPrivateKey: key,
		Data:   &oauth.ClientSessionData{HostURL: budgetTestPDS, AccountDID: syntax.DID(repairTestDID), AccessToken: "test"},
		Client: &http.Client{Transport: transport}}
	c := withUploadBudget(sess.APIClient(), s, repairTestDID, false)
	var token map[string]string
	if err := c.Get(context.Background(), "com.atproto.server.getServiceAuth", map[string]any{"lxm": "com.atproto.repo.uploadBlob"}, &token); err != nil {
		t.Fatal(err)
	}
	b := readUploadBudget(t, s, repairTestDID, budgetTestPDS)
	if calls != 2 || b.Window.Remaining != 899 || !b.ManualUntil.After(time.Now()) {
		t.Fatalf("token retry spent an extra slot: calls=%d budget=%+v", calls, b)
	}
	failed := uploadBudgetClient(s, budgetTestPDS, false, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		return budgetResponse(403, http.Header{}, `{"error":"AuthRequired"}`), nil
	}))
	if err := failed.Get(context.Background(), "com.atproto.server.getServiceAuth", map[string]any{"lxm": "com.atproto.repo.uploadBlob"}, &token); err == nil {
		t.Fatal("failed token request unexpectedly succeeded")
	}
	if got := readUploadBudget(t, s, repairTestDID, budgetTestPDS).Window.Remaining; got != 899 {
		t.Fatalf("failed token request spent a slot: %d", got)
	}
}

func TestImportUploadBudgetAllowsOAuthUploadRetry(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprint("expired=", expired), func(t *testing.T) {
			s := newTestStore(t)
			left := time.Hour
			if expired {
				left = -time.Minute
			}
			seedUploadBudget(t, s, 900, left)
			key, err := atcrypto.GeneratePrivateKeyP256()
			if err != nil {
				t.Fatal(err)
			}
			var calls int
			transport := blobRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					h := make(http.Header)
					h.Set("WWW-Authenticate", `DPoP error="use_dpop_nonce"`)
					h.Set("DPoP-Nonce", "upload-nonce")
					return budgetResponse(401, h, `{"error":"use_dpop_nonce"}`), nil
				}
				return budgetResponse(200, budgetHeaders(time.Now(), 899, time.Hour), `{"ok":true}`), nil
			})
			config := oauth.NewPublicConfig("https://currents.test/client.json", "https://currents.test/callback", []string{"atproto"})
			sess := &oauth.ClientSession{Config: &config, DPoPPrivateKey: key,
				Data:   &oauth.ClientSessionData{HostURL: budgetTestPDS, AccountDID: syntax.DID(repairTestDID), AccessToken: "test"},
				Client: &http.Client{Transport: transport}}
			if err := budgetUpload(withUploadBudget(sess.APIClient(), s, repairTestDID, true)); err != nil {
				t.Fatalf("OAuth retry was blocked: %v", err)
			}
			b := readUploadBudget(t, s, repairTestDID, budgetTestPDS)
			if calls != 2 || b.Window.Remaining != 899 || b.RetryAt.After(time.Now()) {
				t.Fatalf("auth challenge spent quota or triggered cooldown: calls=%d budget=%+v", calls, b)
			}
		})
	}
}

func TestImportUploadBudgetPreservesConcurrentManualReservation(t *testing.T) {
	s := newTestStore(t)
	seedUploadBudget(t, s, 900, time.Hour)
	started, finish := make(chan struct{}), make(chan struct{})
	transport := blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-finish
		// This response predates the browser upload reserved in the meantime.
		return budgetResponse(200, budgetHeaders(time.Now(), 899, time.Hour), `{"ok":true}`), nil
	})
	done := make(chan error, 1)
	go func() { done <- budgetUpload(uploadBudgetClient(s, budgetTestPDS, true, transport)) }()
	<-started
	manual := uploadBudgetClient(s, budgetTestPDS, false, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		return budgetResponse(200, http.Header{}, `{"token":"test"}`), nil
	}))
	var token map[string]string
	err := manual.Get(context.Background(), "com.atproto.server.getServiceAuth", map[string]any{"lxm": "com.atproto.repo.uploadBlob"}, &token)
	close(finish)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := readUploadBudget(t, s, repairTestDID, budgetTestPDS).Window.Remaining; got != 898 {
		t.Fatalf("late response refunded manual reservation: %d", got)
	}
}

func TestImportUploadBudgetChargesFailuresAndKeepsSDKError(t *testing.T) {
	s := newTestStore(t)
	seedUploadBudget(t, s, 900, time.Hour)
	c := uploadBudgetClient(s, budgetTestPDS, true, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		return budgetResponse(500, http.Header{}, `{"error":"InternalServerError","message":"failed after admission"}`), nil
	}))
	err := budgetUpload(c)
	var apiErr *atclient.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 || apiErr.Message != "failed after admission" {
		t.Fatalf("SDK error handling changed: %v", err)
	}
	if got := readUploadBudget(t, s, repairTestDID, budgetTestPDS).Window.Remaining; got != 899 {
		t.Fatalf("failed upload was refunded: %d", got)
	}
	if wait, err := s.importUploadWait(context.Background(), repairTestDID, budgetTestPDS); err != nil || wait <= 0 {
		t.Fatalf("failed upload bypassed pacing: %s %v", wait, err)
	}
}

func TestImportUploadBudgetOtherPDSIsObservationOnly(t *testing.T) {
	s := newTestStore(t)
	const pds = "https://eurosky.social"
	var sent int
	c := uploadBudgetClient(s, pds, true, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		sent++
		return budgetResponse(429, budgetHeaders(time.Now(), 0, 24*time.Hour), `{"error":"RateLimitExceeded"}`), nil
	}))
	for range 2 {
		if err := budgetUpload(c); !isRateLimited(err) {
			t.Fatalf("non-Bluesky response changed: %v", err)
		}
	}
	b := readUploadBudget(t, s, repairTestDID, pds)
	if sent != 2 || b.Window != nil || b.Status != 429 || b.Headers["RateLimit-Remaining"] != "0" {
		t.Fatalf("other provider was gated or unobserved: sent=%d %+v", sent, b)
	}
}

func TestImportUploadBudgetWaitDoesNotFetchOrSpendRetry(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	job := importTestJob(t, s, collectionTestURI("root"))
	if _, err := s.BulkInsertImportItems(ctx, job, repairTestDID, []PinterestPin{{ID: "one", ImageURL: "https://example.test/image.png"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.finishImportListing(ctx, job, ""); err != nil {
		t.Fatal(err)
	}
	item, err := s.ClaimNextImportItem(ctx, repairTestDID)
	if err != nil || item == nil {
		t.Fatalf("claim: %v %v", item, err)
	}
	seedUploadBudget(t, s, 50, 4*time.Minute)
	config := oauth.NewPublicConfig("https://currents.test/client.json", "https://currents.test/callback", []string{"atproto"})
	sess := &oauth.ClientSession{Config: &config, Data: &oauth.ClientSessionData{HostURL: budgetTestPDS, AccountDID: syntax.DID(repairTestDID)}, Client: &http.Client{Transport: blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("waiting import contacted PDS: %s", r.URL)
		return nil, nil
	})}}
	worker := &ImportWorker{Store: s}
	if wait := worker.processItem(ctx, item, repairTestDID, sess); wait != importBudgetRecheck {
		t.Fatalf("wait = %s", wait)
	}
	var status string
	var attempts int
	if err := s.pool.QueryRow(ctx, `SELECT status,attempt_count FROM import_item WHERE id=$1`, item.ID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "queued" || attempts != 0 {
		t.Fatalf("waiting spent a retry: %s %d", status, attempts)
	}
}

func TestImportUploadBudgetSurvivesRecordWriteFailure(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	job := importTestJob(t, s, collectionTestURI("root"))
	if _, err := s.BulkInsertImportItems(ctx, job, repairTestDID, []PinterestPin{{ID: "one", ImageURL: "https://example.test/image.png"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.finishImportListing(ctx, job, ""); err != nil {
		t.Fatal(err)
	}
	item, err := s.ClaimNextImportItem(ctx, repairTestDID)
	if err != nil || item == nil {
		t.Fatalf("claim: %v %v", item, err)
	}
	seedUploadBudget(t, s, 900, time.Hour)
	stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
		return 200, http.Header{"Content-Type": {"image/png"}}, string(testBlobPNG)
	})
	var uploaded, wrote bool
	transport := blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.repo.getRecord":
			return budgetResponse(200, http.Header{}, `{"cid":"collection-cid","value":{"name":"root"}}`), nil
		case "/xrpc/com.atproto.repo.uploadBlob":
			if r.Header.Get("DPoP") == "" {
				t.Fatal("budget wrapper bypassed OAuth authentication")
			}
			uploaded = true
			return budgetResponse(200, budgetHeaders(time.Now(), 899, time.Hour), `{"blob":{"$type":"blob","ref":{"$link":"bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"},"mimeType":"image/png","size":68}}`), nil
		case "/xrpc/com.atproto.repo.putRecord":
			wrote = true
			return budgetResponse(500, http.Header{}, `{"error":"InternalServerError"}`), nil
		default:
			t.Fatalf("unexpected PDS call: %s", r.URL)
			return nil, nil
		}
	})
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	config := oauth.NewPublicConfig("https://currents.test/client.json", "https://currents.test/callback", []string{"atproto"})
	sess := &oauth.ClientSession{Config: &config, DPoPPrivateKey: key,
		Data:   &oauth.ClientSessionData{HostURL: budgetTestPDS, AccountDID: syntax.DID(repairTestDID), AccessToken: "test"},
		Client: &http.Client{Transport: transport}}
	worker := &ImportWorker{Store: s}
	worker.processItem(ctx, item, repairTestDID, sess)
	if !uploaded || !wrote {
		t.Fatalf("did not reach record failure: uploaded=%v wrote=%v", uploaded, wrote)
	}
	_, daily, err := s.CountRecentImportDone(ctx, repairTestDID)
	if err != nil || daily != 0 {
		t.Fatalf("failed record counted as saved: %d %v", daily, err)
	}
	if got := readUploadBudget(t, s, repairTestDID, budgetTestPDS).Window.Remaining; got != 899 {
		t.Fatalf("successful upload was lost with failed record: %d", got)
	}
}
