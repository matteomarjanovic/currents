package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func TestEnrichmentMetricsSeparateFailuresFromPending(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:enrichment"
	pendingAt := testBase.Add(time.Hour)
	old := testBase.Add(-129 * 24 * time.Hour)
	for _, item := range []struct{ key, cid string }{
		{"pending", "pending"}, {"invalid-one", "invalid"}, {"invalid-two", "invalid"},
		{"unsupported", "unsupported"}, {"unavailable", "unavailable"}, {"failed", "failed"}, {"resolved", "resolved"},
	} {
		created := testBase
		if item.key == "pending" {
			created = pendingAt
		} else if item.key == "unsupported" {
			created = old
		}
		seedImageSave(t, store, "at://"+did+"/is.currents.feed.save/"+item.key, did, "", item.cid, 1, created)
	}
	for cid, kind := range map[string]string{"invalid": "failed", "unsupported": "unsupported", "unavailable": "source_unavailable", "failed": "failed", "resolved": "failed"} {
		if err := store.RecordBlobEnrichmentFailure(ctx, cid, kind, "first failure"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordBlobEnrichmentFailure(ctx, "invalid", "invalid", "latest failure"); err != nil {
		t.Fatal(err)
	}
	embedding := make([]float32, 768)
	embedding[0] = 1
	vi, err := store.CreateVI(ctx, did, "resolved", embedding, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE save SET visual_identity_id = $1 WHERE pds_blob_cid = 'resolved'`, vi); err != nil {
		t.Fatal(err)
	}
	metrics, err := store.GetBackgroundMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := metrics.Saves
	if m.MissingVisualIdentityCount != 6 || m.DistinctMissingBlobCIDCount != 5 || m.PendingEnrichmentCount != 1 ||
		m.FailedEnrichmentCount != 1 || m.UnsupportedMediaCount != 1 || m.InvalidMediaCount != 2 || m.UnavailableSourceCount != 1 {
		t.Fatalf("unexpected classification counts: %+v", m)
	}
	if m.OldestMissingCreatedAt == nil || !m.OldestMissingCreatedAt.Equal(old) ||
		m.OldestPendingCreatedAt == nil || !m.OldestPendingCreatedAt.Equal(pendingAt) {
		t.Fatalf("pending age includes historical failures: %+v", m)
	}
	if err := store.ClearBlobEnrichmentFailure(ctx, "invalid"); err != nil {
		t.Fatal(err)
	}
	metrics, err = store.GetBackgroundMetrics(ctx)
	if err != nil || metrics.Saves.PendingEnrichmentCount != 3 || metrics.Saves.InvalidMediaCount != 0 {
		t.Fatalf("cleared failures not reflected: %+v, %v", metrics, err)
	}
}

func TestBlobEnrichmentRecordsFailureAndClearsItOnSuccess(t *testing.T) {
	mp4, err := hex.DecodeString("000000206674797069736f6d0000020069736f6d69736f32617663316d703431")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name            string
		blobStatus      int
		inferenceStatus int
		body            []byte
		want            string
	}{
		{"video", 200, 200, mp4, "unsupported"},
		{"invalid image", 200, 400, []byte("invalid image"), "invalid"},
		{"missing source", 404, 200, nil, "source_unavailable"},
		{"temporary inference failure", 200, 503, testBlobPNG, "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := context.Background()
			const did, cid = "did:plc:enrichment", "test-blob"
			var success atomic.Bool
			pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if success.Load() {
					w.Write(testBlobPNG)
					return
				}
				w.WriteHeader(test.blobStatus)
				w.Write(test.body)
			}))
			defer pds.Close()
			// These lifecycle tests use a local fixture PDS. Network blocking is
			// tested separately with the production transport in blob_security_test.
			stubBlobTransport(t, http.DefaultTransport)
			var inferenceCalls atomic.Int64
			inference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				inferenceCalls.Add(1)
				if !success.Load() && test.inferenceStatus != http.StatusOK {
					http.Error(w, "test inference failure", test.inferenceStatus)
					return
				}
				embedding := make([]float32, 768)
				embedding[0] = 1
				json.NewEncoder(w).Encode(ImageEmbedding{Embedding: embedding, Width: 64, Height: 32, DominantColors: json.RawMessage(`[]`)})
			}))
			defer inference.Close()
			dir := identity.NewMockDirectory()
			dir.Insert(identity.Identity{DID: syntax.DID(did), Services: map[string]identity.ServiceEndpoint{"atproto_pds": {URL: pds.URL}}})
			if _, err := store.pool.Exec(ctx, `INSERT INTO "user" (did, created_at, pds_endpoint) VALUES ($1, now(), $2)`, did, pds.URL); err != nil {
				t.Fatal(err)
			}
			seedImageSave(t, store, "at://"+did+"/is.currents.feed.save/one", did, "", cid, 1, testBase)
			handler := &TapHandler{Store: store, Dir: dir, Inference: NewInferenceClient(inference.URL)}
			if err := processBlobEnrichment(ctx, handler, cid); err == nil {
				t.Fatal("expected initial enrichment failure")
			}
			var kind string
			if err := store.pool.QueryRow(ctx, `SELECT kind FROM blob_enrichment_failure WHERE blob_cid = $1`, cid).Scan(&kind); err != nil || kind != test.want {
				t.Fatalf("recorded kind = %q, %v; want %q", kind, err, test.want)
			}
			if test.want == "unsupported" && inferenceCalls.Load() != 0 {
				t.Fatal("MP4 should not be sent to image inference")
			}
			success.Store(true)
			if err := processBlobEnrichment(ctx, handler, cid); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
			var failures, unresolved int
			if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blob_enrichment_failure), (SELECT count(*) FROM save WHERE visual_identity_id IS NULL)`).Scan(&failures, &unresolved); err != nil || failures != 0 || unresolved != 0 {
				t.Fatalf("successful retry left failure/pending state: %d/%d, %v", failures, unresolved, err)
			}
		})
	}
}

func TestCancelledEnrichmentIsNotRecordedAsFailure(t *testing.T) {
	store := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := processBlobEnrichment(ctx, &TapHandler{Store: store}, "cancelled"); err == nil {
		t.Fatal("expected cancellation")
	}
	var failures int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM blob_enrichment_failure`).Scan(&failures); err != nil || failures != 0 {
		t.Fatalf("shutdown recorded an enrichment failure: %d, %v", failures, err)
	}
}
