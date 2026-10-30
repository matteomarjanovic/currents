package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/sessions"
)

func seedFeedImage(t *testing.T, s *PgStore, i int, created time.Time) (string, string, []float32) {
	t.Helper()
	emb := make([]float32, 768)
	emb[0], emb[1] = float32(math.Cos(float64(i)/10)), float32(math.Sin(float64(i)/10))
	blob := fmt.Sprintf("feed-blob-%d", i)
	uri := fmt.Sprintf("at://did:plc:author/is.currents.feed.save/%d", i)
	vi, err := s.CreateVI(context.Background(), "did:plc:author", blob, emb, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSave(context.Background(), UpsertSaveParams{
		URI: uri, AuthorDID: "did:plc:author", PdsBlobCID: blob,
		ContentNSID: "is.currents.content.image", CreatedAt: &created,
		VisualIdentityID: &vi, AltText: blob,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVICanonicalSave(context.Background(), vi, uri); err != nil {
		t.Fatal(err)
	}
	return uri, vi, emb
}

func TestGlobalFeedMatchesOriginalRankingAndPagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const viewer = "did:plc:viewer"
	for i := range 12 {
		created := testBase.AddDate(0, 0, -i*3)
		if i >= 9 {
			created = testBase.AddDate(-50*(i-8), 0, 0)
		}
		uri, vi, _ := seedFeedImage(t, s, i, created)
		if _, err := s.pool.Exec(ctx, `UPDATE visual_identity SET saver_count=$2 WHERE id=$1`, vi, i%5+1); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if _, err := s.HideFeedImage(ctx, viewer, uri); err != nil {
				t.Fatal(err)
			}
		}
	}
	seedImageSave(t, s, "at://did:plc:viewer/is.currents.feed.save/copy", viewer, "", "feed-blob-0", 0.5, testBase)
	if _, err := s.pool.Exec(ctx, `
		UPDATE visual_identity SET saver_count=0 WHERE canonical_blob_cid='feed-blob-11';
		UPDATE visual_identity SET junk_score=0.9 WHERE canonical_blob_cid='feed-blob-2';
		UPDATE save SET created_at=NULL WHERE pds_blob_cid='feed-blob-3';
		INSERT INTO blob_moderation_state (blob_cid, harm_state) VALUES ('feed-blob-4', 'blocked');
	`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		viewer  string
		exclude bool
	}{{"", false}, {viewer, false}, {viewer, true}} {
		// Keep the original multiplicative decimal score as the independent reference.
		rows, err := s.pool.Query(ctx, `
			SELECT s.uri FROM visual_identity vi JOIN save s ON s.uri=vi.canonical_save_uri
			WHERE s.created_at IS NOT NULL AND (vi.junk_score IS NULL OR vi.junk_score < $3)
			AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid=s.pds_blob_cid AND b.harm_state='blocked')
			AND NOT EXISTS (SELECT 1 FROM hidden_feed_image h WHERE h.viewer_did=$1 AND h.visual_identity_id=vi.id)
			AND (NOT $2 OR NOT EXISTS (SELECT 1 FROM save v WHERE v.author_did=$1 AND v.pds_blob_cid=s.pds_blob_cid))
			ORDER BY vi.saver_count * EXP(-0.1 * EXTRACT(EPOCH FROM (NOW()-s.created_at))/86400)
			* (0.8 + 0.4 * (hashtext(s.uri || current_date::text) & 2147483647)/2147483648.0) DESC
		`, tc.viewer, tc.exclude, feedJunkScoreMax)
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		for rows.Next() {
			var uri string
			if err := rows.Scan(&uri); err != nil {
				t.Fatal(err)
			}
			want = append(want, uri)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		var got []string
		for offset := 0; offset < len(want); offset += 3 {
			page, err := s.GetGlobalFeedSaves(ctx, tc.viewer, tc.exclude, 3, offset)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range page {
				got = append(got, row.URI)
				if row.AltText != row.BlobCID {
					t.Fatalf("metadata lost for %s", row.URI)
				}
				if tc.viewer != "" && row.BlobCID == "feed-blob-0" {
					var saved []struct {
						SaveURI string `json:"saveUri"`
					}
					if json.Unmarshal(row.ViewerSaves, &saved) != nil || len(saved) != 1 || saved[0].SaveURI != "at://did:plc:viewer/is.currents.feed.save/copy" {
						t.Fatalf("viewer save state lost: %s", row.ViewerSaves)
					}
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("viewer=%q exclude=%v: got %v, want original order %v", tc.viewer, tc.exclude, got, want)
		}
	}
}

func TestPersonalFeedMatchesSequentialPools(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const viewer = "did:plc:viewer"
	cursor := feedCursor{Version: 1, Mode: feedCursorModePositive, Initialized: true, Seed: 42}
	var uris []string
	for i := range 10 {
		_, _, emb := seedFeedImage(t, s, i, testBase)
		if i < 3 {
			uri := fmt.Sprintf("at://%s/is.currents.collection/%d", viewer, i)
			if err := s.UpsertCollection(ctx, uri, "cid", viewer, "collection", "", "", &testBase); err != nil {
				t.Fatal(err)
			}
			if err := s.UpdateCollectionEmbedding(ctx, uri, emb); err != nil {
				t.Fatal(err)
			}
			uris = append(uris, uri)
			cursor.Collections = append(cursor.Collections, feedCursorCollection{URI: uri, Offset: i})
		}
	}
	cols, err := s.GetCollectionsByURIs(ctx, uris)
	if err != nil {
		t.Fatal(err)
	}
	const limit = 4
	var pools []*feedCandidatePool
	for _, col := range cols {
		offset := 0
		for _, c := range cursor.Collections {
			if c.URI == col.URI {
				offset = c.Offset
			}
		}
		rows, err := s.SearchSavesByEmbedding(ctx, col.Embedding, viewer, true, feedPoolFetchLimit(limit)+1, offset)
		if err != nil {
			t.Fatal(err)
		}
		pools = append(pools, &feedCandidatePool{Key: col.URI, Weight: 1.0 / 3, Offset: offset, Items: rows})
	}
	want := buildFeedPage(rand.New(rand.NewSource(cursor.Seed^feedMixSeedSalt)), pools, limit)
	next := feedCursor{Version: 1, Mode: cursor.Mode, Initialized: true, Seed: cursor.Seed}
	for _, pool := range pools {
		if pool.hasMoreAfterPage() {
			next.Collections = append(next.Collections, feedCursorCollection{URI: pool.Key, Offset: pool.nextOffset()})
		}
	}
	wantCursor, err := encodeFeedCursor(next)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeFeedCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: s, CookieStore: sessions.NewCookieStore([]byte("feed-test-secret"))}
	token, err := srv.encodeSessionToken(viewer, "session", "viewer.test")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/xrpc/is.currents.feed.getFeed?personalized=1&excludeSaved=true&limit=4&cursor="+encoded, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	srv.XRPCGetFeed(res, req)
	var got struct {
		Feed   []saveView `json:"feed"`
		Cursor string     `json:"cursor"`
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("getFeed = %d %s", res.Code, res.Body.String())
	}
	var gotURIs, wantURIs []string
	for _, row := range got.Feed {
		gotURIs = append(gotURIs, row.URI)
	}
	for _, row := range want {
		wantURIs = append(wantURIs, row.URI)
	}
	if !reflect.DeepEqual(gotURIs, wantURIs) || got.Cursor != wantCursor {
		t.Fatalf("parallel feed = %v cursor %s; want sequential %v cursor %s", gotURIs, got.Cursor, wantURIs, wantCursor)
	}
}
