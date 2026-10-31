package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/sessions"
)

func TestGlobalFeedSnapshotLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := range 20 {
		_, vi, _ := seedFeedImage(t, s, i, testBase.Add(-time.Duration(i)*time.Hour))
		if _, err := s.pool.Exec(ctx, `UPDATE visual_identity SET saver_count=$2 WHERE id=$1`, vi, i%4+1); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.GetGlobalFeedPage(ctx, "", false, 0, 0, 100)
	if err != nil || page.SnapshotID != 0 {
		t.Fatalf("before first build = %#v, %v", page, err)
	}
	id, count, err := s.RefreshGlobalFeed(ctx, testBase)
	if err != nil || id == 0 || count != 20 {
		t.Fatalf("first build = %d, %d, %v", id, count, err)
	}
	// Compare against the multiplicative score, using the agreed three-hour seed.
	rows, err := s.pool.Query(ctx, `
		SELECT s.uri FROM visual_identity vi JOIN save s ON s.uri=vi.canonical_save_uri
		ORDER BY vi.saver_count * EXP(-0.1 * EXTRACT(EPOCH FROM ($1::timestamptz-s.created_at))/86400)
		* (0.8 + 0.4 * (hashtext(s.uri || $2) & 2147483647)/2147483648.0) DESC, s.uri, vi.id
	`, testBase, strconv.FormatInt(testBase.Unix()/10800, 10))
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
	page, err = s.GetGlobalFeedPage(ctx, "", false, id, 0, 100)
	if err != nil || !reflect.DeepEqual(snapshotURIs(page), want) {
		t.Fatalf("snapshot order = %v, %v; want %v", snapshotURIs(page), err, want)
	}
	if next, _, err := s.RefreshGlobalFeed(ctx, testBase.Add(3*time.Hour-time.Second)); err != nil || next != 0 {
		t.Fatalf("premature refresh = %d, %v", next, err)
	}
	second, _, err := s.RefreshGlobalFeed(ctx, testBase.Add(3*time.Hour))
	if err != nil || second <= id {
		t.Fatalf("second refresh = %d, %v", second, err)
	}
	latestPage, err := s.GetGlobalFeedPage(ctx, "", false, 0, 0, 100)
	if err != nil || latestPage.SnapshotID != second || reflect.DeepEqual(snapshotURIs(latestPage), want) {
		t.Fatalf("three-hour reshuffle did not change the latest order: %#v, %v", latestPage, err)
	}
	page, err = s.GetGlobalFeedPage(ctx, "", false, id, 0, 100)
	if err != nil || !reflect.DeepEqual(snapshotURIs(page), want) {
		t.Fatal("previous generation changed after refresh", err)
	}
	// Force a failure after the metadata insert. Both published versions survive.
	if _, err := s.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE global_feed_rank ADD CONSTRAINT fail_build CHECK (snapshot_id <= %d)`, second)); err != nil {
		t.Fatal(err)
	}
	_, _, buildErr := s.RefreshGlobalFeed(ctx, testBase.Add(6*time.Hour))
	if _, err := s.pool.Exec(ctx, `ALTER TABLE global_feed_rank DROP CONSTRAINT fail_build`); err != nil {
		t.Fatal(err)
	}
	if buildErr == nil {
		t.Fatal("expected the build to fail")
	}
	page, err = s.GetGlobalFeedPage(ctx, "", false, 0, 0, 100)
	if err != nil || page.SnapshotID != second {
		t.Fatal("failed refresh replaced the current generation", err)
	}
	if _, err := s.GetGlobalFeedPage(ctx, "", false, id, 0, 1); err != nil {
		t.Fatal("failed refresh pruned the previous generation", err)
	}
	// A second appview must not duplicate an in-flight build.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, globalFeedRefreshLock); err != nil {
		t.Fatal(err)
	}
	lockedID, _, lockedErr := s.RefreshGlobalFeed(ctx, testBase.Add(6*time.Hour))
	tx.Rollback(ctx)
	if lockedErr != nil || lockedID != 0 {
		t.Fatalf("concurrent refresh = %d, %v", lockedID, lockedErr)
	}
	if _, _, err := s.RefreshGlobalFeed(ctx, testBase.Add(6*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetGlobalFeedPage(ctx, "", false, id, 0, 1); !errors.Is(err, errFeedCursorExpired) {
		t.Fatalf("retired generation = %v; want expired cursor", err)
	}
	var generations int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM global_feed_snapshot`).Scan(&generations); err != nil || generations != 2 {
		t.Fatalf("retained generations = %d, %v", generations, err)
	}
}

func snapshotURIs(page globalFeedPage) []string {
	var uris []string
	for _, row := range page.Rows {
		uris = append(uris, row.URI)
	}
	return uris
}

func TestGlobalFeedSnapshotLiveFiltersAndPositions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const viewer = "did:plc:viewer"
	for i := range 16 {
		seedFeedImage(t, s, i, testBase)
	}
	id, _, err := s.RefreshGlobalFeed(ctx, testBase)
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.GetGlobalFeedPage(ctx, "", false, id, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	seedImageSave(t, s, "at://did:plc:viewer/is.currents.feed.save/copy", viewer, "", all.Rows[0].BlobCID, 0.5, testBase)
	if _, err := s.HideFeedImage(ctx, viewer, all.Rows[2].URI); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO blob_moderation_state (blob_cid,harm_state) VALUES ($1,'blocked')`, all.Rows[1].BlobCID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE visual_identity SET junk_score=0.9 WHERE canonical_save_uri=$1`, all.Rows[3].URI); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE save SET created_at=NULL WHERE uri=$1`, all.Rows[4].URI); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSave(ctx, all.Rows[5].URI); err != nil {
		t.Fatal(err)
	}
	page, err := s.GetGlobalFeedPage(ctx, viewer, true, id, 0, 3)
	if err != nil || !reflect.DeepEqual(snapshotURIs(page), snapshotURIs(globalFeedPage{Rows: all.Rows[6:9]})) {
		t.Fatalf("live filters = %v, %v", snapshotURIs(page), err)
	}
	if !reflect.DeepEqual(page.Positions, []int{7, 8, 9}) {
		t.Fatalf("positions = %v; want absolute positions 7,8,9", page.Positions)
	}
	if _, err := s.HideFeedImage(ctx, viewer, page.Rows[0].URI); err != nil {
		t.Fatal(err)
	}
	next, err := s.GetGlobalFeedPage(ctx, viewer, true, id, page.Positions[2], 3)
	if err != nil || !reflect.DeepEqual(snapshotURIs(next), snapshotURIs(globalFeedPage{Rows: all.Rows[9:12]})) {
		t.Fatalf("page after earlier removal = %v, %v", snapshotURIs(next), err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE blob_moderation_state SET harm_state='clean' WHERE blob_cid=$1`, all.Rows[1].BlobCID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE visual_identity SET junk_score=0.1 WHERE canonical_save_uri=$1`, all.Rows[3].URI); err != nil {
		t.Fatal(err)
	}
	page, err = s.GetGlobalFeedPage(ctx, viewer, false, id, 0, 3)
	if err != nil || !reflect.DeepEqual(snapshotURIs(page), []string{all.Rows[0].URI, all.Rows[1].URI, all.Rows[3].URI}) {
		t.Fatalf("live unblocking = %v, %v", snapshotURIs(page), err)
	}
	var saved []map[string]string
	if json.Unmarshal(page.Rows[0].ViewerSaves, &saved) != nil || len(saved) != 1 {
		t.Fatal("viewer state was not hydrated")
	}
	newURI, _, _ := seedFeedImage(t, s, 99, testBase.Add(time.Hour))
	page, err = s.GetGlobalFeedPage(ctx, "", false, id, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Rows {
		if row.URI == newURI {
			t.Fatal("post-build image appeared before refresh")
		}
	}
	// Re-election changes the public save reference, not the retained VI position.
	var vi string
	if err := s.pool.QueryRow(ctx, `SELECT visual_identity_id FROM save WHERE uri=$1`, all.Rows[10].URI).Scan(&vi); err != nil {
		t.Fatal(err)
	}
	replacement := "at://did:plc:author/is.currents.feed.save/replacement"
	if err := s.UpsertSave(ctx, UpsertSaveParams{
		URI: replacement, AuthorDID: "did:plc:author", PdsBlobCID: all.Rows[10].BlobCID,
		ContentNSID: "is.currents.content.image", CreatedAt: &testBase, VisualIdentityID: &vi,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVICanonicalSave(ctx, vi, replacement); err != nil {
		t.Fatal(err)
	}
	page, err = s.GetGlobalFeedPage(ctx, "", false, id, 10, 1)
	if err != nil || len(page.Rows) != 1 || page.Rows[0].URI != replacement || page.Positions[0] != 11 {
		t.Fatalf("canonical re-election = %#v, %v", page, err)
	}
	s.cfg.HiddenDIDs = []string{"did:plc:author"}
	defer func() { s.cfg.HiddenDIDs = []string{} }()
	page, err = s.GetGlobalFeedPage(ctx, "", false, id, 0, 100)
	if err != nil || len(page.Rows) != 0 {
		t.Fatalf("author exclusion = %#v, %v", page, err)
	}
}

func TestSnapshotFeedCursorsAndMixedPool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const viewer = "did:plc:viewer"
	var emb []float32
	for i := range 12 {
		_, _, emb = seedFeedImage(t, s, i, testBase)
	}
	srv := &Server{Store: s, CookieStore: sessions.NewCookieStore([]byte("snapshot-test"))}
	token, err := srv.encodeSessionToken(viewer, "session", "viewer.test")
	if err != nil {
		t.Fatal(err)
	}
	request := func(params url.Values) (int, []saveView, string, string) {
		t.Helper()
		r := httptest.NewRequest("GET", "/xrpc/is.currents.feed.getFeed?"+params.Encode(), nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		srv.XRPCGetFeed(w, r)
		var data struct {
			Feed   []saveView `json:"feed"`
			Cursor string     `json:"cursor"`
			Error  string     `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(w.Body.String(), err)
		}
		return w.Code, data.Feed, data.Cursor, data.Error
	}
	params := url.Values{"limit": {"3"}, "personalized": {"0"}}
	_, _, legacy, _ := request(params)
	id, _, err := s.RefreshGlobalFeed(ctx, testBase)
	if err != nil {
		t.Fatal(err)
	}
	params.Set("cursor", legacy)
	code, _, encoded, _ := request(params)
	legacyNext, err := decodeFeedCursor(encoded)
	if err != nil || code != 200 || legacyNext.SnapshotID != 0 {
		t.Fatal("legacy pagination changed format", err)
	}
	params.Del("cursor")
	code, _, firstCursor, _ := request(params)
	first, err := decodeFeedCursor(firstCursor)
	if err != nil || code != 200 || first.SnapshotID != id || first.Version != 2 || first.GlobalOffset != 3 {
		t.Fatalf("snapshot cursor = %#v, %v", first, err)
	}
	if _, _, err := s.RefreshGlobalFeed(ctx, testBase.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	params.Set("cursor", firstCursor)
	code, _, secondCursor, _ := request(params)
	second, err := decodeFeedCursor(secondCursor)
	if err != nil || code != 200 || second.SnapshotID != id || second.GlobalOffset != 6 {
		t.Fatalf("cursor switched generation: %#v, %v", second, err)
	}
	// A nearly-global blend exhausts its sole clean global item on this page,
	// but must keep that absolute position while the ANN pool continues.
	col := "at://did:plc:viewer/is.currents.collection/c"
	if err := s.UpsertCollection(ctx, col, "cid", viewer, "c", "", "", &testBase); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCollectionEmbedding(ctx, col, emb); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE visual_identity SET junk_score=0.9 WHERE id NOT IN (SELECT visual_identity_id FROM global_feed_rank WHERE snapshot_id=$1 AND position=12)`, id); err != nil {
		t.Fatal(err)
	}
	global, err := s.GetGlobalFeedPage(ctx, viewer, false, id, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	ann, err := s.SearchSavesByEmbedding(ctx, emb, viewer, false, feedPoolFetchLimit(3)+1, 0)
	if err != nil {
		t.Fatal(err)
	}
	pools := []*feedCandidatePool{
		{Key: col, Weight: 0.01, Items: ann},
		{Weight: 0.99, Items: global.Rows, Positions: global.Positions},
	}
	want := buildFeedPage(rand.New(rand.NewSource(42^feedMixSeedSalt)), pools, 3)
	mixed, err := encodeFeedCursor(feedCursor{Mode: feedCursorModePositive, Initialized: true, SnapshotID: id, Seed: 42, Collections: []feedCursorCollection{{URI: col}}})
	if err != nil {
		t.Fatal(err)
	}
	params.Set("cursor", mixed)
	params.Set("personalized", "0.01")
	code, got, cursor, _ := request(params)
	next, err := decodeFeedCursor(cursor)
	if err != nil || code != 200 || next.SnapshotID != id || next.GlobalOffset != 12 || len(next.Collections) != 1 || next.Collections[0].Offset != pools[0].nextOffset() {
		t.Fatalf("mixed continuation = %#v, %v", next, err)
	}
	for i := range want {
		if got[i].URI != want[i].URI {
			t.Fatalf("mixed item %d = %s; want %s", i, got[i].URI, want[i].URI)
		}
	}
	if _, _, err := s.RefreshGlobalFeed(ctx, testBase.Add(6*time.Hour)); err != nil {
		t.Fatal(err)
	}
	params.Set("cursor", firstCursor)
	params.Set("personalized", "0")
	code, _, _, apiError := request(params)
	if code != 400 || apiError != "ExpiredCursor" {
		t.Fatalf("expired cursor = %d %s", code, apiError)
	}
}
