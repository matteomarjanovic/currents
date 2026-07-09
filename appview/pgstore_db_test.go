package main

// DB-backed regression tests for PgStore queries. They run only when
// TEST_DATABASE_URL is set (e.g. against the dev compose Postgres:
//
//	TEST_DATABASE_URL=postgres://appview:appview@localhost:5432/appview_test go test ./...
//
// The database name must end in _test: the suite creates it if missing, drops
// and recreates its public schema, and runs every embedded migration — so each
// run also verifies the migrations apply cleanly from scratch.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	testStoreOnce sync.Once
	testStore     *PgStore
	testStoreErr  error
)

func newTestStore(t *testing.T) *PgStore {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB-backed regression tests")
	}
	testStoreOnce.Do(func() {
		testStore, testStoreErr = openTestStore(dsn)
	})
	if testStoreErr != nil {
		t.Fatalf("opening test store: %v", testStoreErr)
	}
	truncateAll(t, testStore)
	return testStore
}

func openTestStore(dsn string) (*PgStore, error) {
	ctx := context.Background()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(cfg.Database, "_test") {
		return nil, fmt.Errorf("TEST_DATABASE_URL database %q must end in _test (the suite wipes its schema)", cfg.Database)
	}

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "3D000" { // invalid_catalog_name
			return nil, err
		}
		admin := cfg.Copy()
		admin.Database = "postgres"
		adminConn, err := pgx.ConnectConfig(ctx, admin)
		if err != nil {
			return nil, err
		}
		_, err = adminConn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{cfg.Database}.Sanitize())
		adminConn.Close(ctx)
		if err != nil {
			return nil, err
		}
		if conn, err = pgx.ConnectConfig(ctx, cfg); err != nil {
			return nil, err
		}
	}
	_, err = conn.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`)
	conn.Close(ctx)
	if err != nil {
		return nil, err
	}

	return NewPgStore(ctx, &PgStoreConfig{
		DSN:                       dsn,
		SessionExpiryDuration:     time.Hour,
		SessionInactivityDuration: time.Hour,
		AuthRequestExpiryDuration: time.Hour,
	})
}

func truncateAll(t *testing.T, s *PgStore) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(), `
		TRUNCATE save, collection, "user", follow, favourite_collection,
			visual_identity, visual_identity_color, cluster,
			label, blob_moderation_state, review_item, report, moderation_event
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

var testBase = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func seedCollection(t *testing.T, s *PgStore, uri, author, name, parentURI string, createdAt time.Time) {
	t.Helper()
	if err := s.UpsertCollection(context.Background(), uri, "cid-"+name, author, name, "", parentURI, &createdAt); err != nil {
		t.Fatalf("seeding collection %s: %v", uri, err)
	}
}

func seedImageSave(t *testing.T, s *PgStore, uri, author, collectionURI, blobCID string, quality float32, createdAt time.Time) {
	t.Helper()
	err := s.UpsertSave(context.Background(), UpsertSaveParams{
		URI:           uri,
		AuthorDID:     author,
		CollectionURI: collectionURI,
		PdsBlobCID:    blobCID,
		ContentNSID:   "is.currents.content.image",
		CreatedAt:     &createdAt,
		QualityScore:  &quality,
	})
	if err != nil {
		t.Fatalf("seeding save %s: %v", uri, err)
	}
}

func previewsOf(row CollectionRow) []string {
	return row.PreviewBlobs
}

// TestSearchCollections pins the rollup semantics of the aggregate-once
// rewrite: save_count and last_saved_at include section saves (and count
// blocked blobs — only previews exclude them), previews prefer direct saves
// over section saves ordered by quality, ordering is by rollup save count,
// and viewer favourites hydrate FavouriteURI.
func TestSearchCollections(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"
	viewer := "did:plc:viewer"

	alpha := "at://" + author + "/is.currents.feed.collection/alpha"
	alphaSec := "at://" + author + "/is.currents.feed.collection/alphasec"
	beta := "at://" + author + "/is.currents.feed.collection/beta"
	zeta := "at://" + author + "/is.currents.feed.collection/zeta"
	seedCollection(t, s, alpha, author, "Alpha Art", "", testBase.Add(3*time.Hour))
	seedCollection(t, s, alphaSec, author, "Alpha Detail", alpha, testBase.Add(3*time.Hour))
	seedCollection(t, s, beta, author, "Beta Art", "", testBase.Add(2*time.Hour))
	seedCollection(t, s, zeta, author, "Zeta", "", testBase.Add(1*time.Hour))

	saveURI := func(rkey string) string { return "at://" + author + "/is.currents.feed.save/" + rkey }
	seedImageSave(t, s, saveURI("a1"), author, alpha, "blob-a1", 0.9, testBase.Add(10*time.Minute))
	seedImageSave(t, s, saveURI("a2"), author, alpha, "blob-a2", 0.5, testBase.Add(20*time.Minute))
	seedImageSave(t, s, saveURI("a3"), author, alpha, "blob-blocked", 0.99, testBase.Add(30*time.Minute))
	seedImageSave(t, s, saveURI("s1"), author, alphaSec, "blob-s1", 0.7, testBase.Add(40*time.Minute))
	seedImageSave(t, s, saveURI("s2"), author, alphaSec, "blob-s2", 0.1, testBase.Add(50*time.Minute))
	seedImageSave(t, s, saveURI("b1"), author, beta, "blob-b1", 0.8, testBase.Add(5*time.Minute))
	seedImageSave(t, s, saveURI("z1"), author, zeta, "blob-z1", 0.8, testBase.Add(5*time.Minute))

	if err := s.SetHarmState(ctx, "blob-blocked", HarmStateBlocked, "auto", ""); err != nil {
		t.Fatalf("SetHarmState: %v", err)
	}
	favURI := "at://" + viewer + "/is.currents.feed.favourite/f1"
	if err := s.UpsertFavourite(ctx, favURI, viewer, beta); err != nil {
		t.Fatalf("UpsertFavourite: %v", err)
	}

	rows, err := s.SearchCollections(ctx, "art", viewer, 10, 0)
	if err != nil {
		t.Fatalf("SearchCollections: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (%#v)", len(rows), rows)
	}

	a := rows[0]
	if a.URI != alpha {
		t.Fatalf("rows[0].URI = %s, want alpha (higher rollup save count first)", a.URI)
	}
	if a.SaveCount != 5 {
		t.Fatalf("alpha SaveCount = %d, want 5 (2 direct + 1 blocked + 2 section)", a.SaveCount)
	}
	if a.LastSavedAt == nil || !a.LastSavedAt.Equal(testBase.Add(50*time.Minute)) {
		t.Fatalf("alpha LastSavedAt = %v, want section save time", a.LastSavedAt)
	}
	if a.AuthorDID != author {
		t.Fatalf("alpha AuthorDID = %q, want %q", a.AuthorDID, author)
	}
	wantPreviews := []string{
		author + ",blob-a1", // direct saves first, by quality desc
		author + ",blob-a2",
		author + ",blob-s1", // then section saves; blocked blob excluded
		author + ",blob-s2",
	}
	if got := previewsOf(a); !equalStrings(got, wantPreviews) {
		t.Fatalf("alpha previews = %v, want %v", got, wantPreviews)
	}

	b := rows[1]
	if b.URI != beta || b.SaveCount != 1 {
		t.Fatalf("rows[1] = %s (count %d), want beta with 1 save", b.URI, b.SaveCount)
	}
	if b.FavouriteCount != 1 {
		t.Fatalf("beta FavouriteCount = %d, want 1", b.FavouriteCount)
	}
	if b.FavouriteURI == nil || *b.FavouriteURI != favURI {
		t.Fatalf("beta FavouriteURI = %v, want %s", b.FavouriteURI, favURI)
	}

	// Offset pagination continues the save-count ordering.
	page, err := s.SearchCollections(ctx, "art", "", 1, 1)
	if err != nil {
		t.Fatalf("SearchCollections offset: %v", err)
	}
	if len(page) != 1 || page[0].URI != beta {
		t.Fatalf("offset page = %#v, want [beta]", page)
	}
	if page[0].FavouriteURI != nil {
		t.Fatal("unauthenticated FavouriteURI should be nil")
	}

	none, err := s.SearchCollections(ctx, "nomatch", "", 10, 0)
	if err != nil {
		t.Fatalf("SearchCollections nomatch: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("nomatch returned %d rows", len(none))
	}
}

// TestGetImageCollectionsPage pins: collections are matched by shared blob CID
// across the network, unsorted saves (collection_uri = ”) never surface,
// ordering is favourite count then recency, and blocked target blobs return
// nothing.
func TestGetImageCollectionsPage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"
	other := "did:plc:other"
	viewer := "did:plc:viewer"

	alpha := "at://" + author + "/is.currents.feed.collection/alpha"
	beta := "at://" + author + "/is.currents.feed.collection/beta"
	gamma := "at://" + other + "/is.currents.feed.collection/gamma"
	seedCollection(t, s, alpha, author, "Alpha", "", testBase.Add(1*time.Hour))
	seedCollection(t, s, beta, author, "Beta", "", testBase.Add(2*time.Hour))
	seedCollection(t, s, gamma, other, "Gamma", "", testBase.Add(3*time.Hour))

	target := "at://" + author + "/is.currents.feed.save/x1"
	seedImageSave(t, s, target, author, alpha, "blob-x", 0.9, testBase)
	seedImageSave(t, s, "at://"+author+"/is.currents.feed.save/x2", author, beta, "blob-x", 0.9, testBase)
	seedImageSave(t, s, "at://"+author+"/is.currents.feed.save/x3", author, "", "blob-x", 0.9, testBase) // unsorted
	seedImageSave(t, s, "at://"+other+"/is.currents.feed.save/x4", other, gamma, "blob-x", 0.9, testBase)

	for i, fav := range []string{"did:plc:fan1", "did:plc:fan2"} {
		uri := fmt.Sprintf("at://%s/is.currents.feed.favourite/g%d", fav, i)
		if err := s.UpsertFavourite(ctx, uri, fav, gamma); err != nil {
			t.Fatalf("UpsertFavourite: %v", err)
		}
	}
	favURI := "at://" + viewer + "/is.currents.feed.favourite/b1"
	if err := s.UpsertFavourite(ctx, favURI, viewer, beta); err != nil {
		t.Fatalf("UpsertFavourite: %v", err)
	}

	rows, err := s.GetImageCollectionsPage(ctx, target, viewer, 10, 0)
	if err != nil {
		t.Fatalf("GetImageCollectionsPage: %v", err)
	}
	var uris []string
	for _, r := range rows {
		uris = append(uris, r.URI)
	}
	if !equalStrings(uris, []string{gamma, beta, alpha}) {
		t.Fatalf("collection order = %v, want [gamma beta alpha] (favourite count, then recency)", uris)
	}
	if rows[0].FavouriteCount != 2 || rows[0].AuthorDID != other {
		t.Fatalf("gamma row = %+v, want 2 favourites by %s", rows[0], other)
	}
	if rows[1].FavouriteURI == nil || *rows[1].FavouriteURI != favURI {
		t.Fatalf("beta FavouriteURI = %v, want %s", rows[1].FavouriteURI, favURI)
	}
	if rows[2].SaveCount != 1 || !equalStrings(previewsOf(rows[2]), []string{author + ",blob-x"}) {
		t.Fatalf("alpha row = %+v, want 1 save and its preview", rows[2])
	}

	// A blocked target blob surfaces no collections at all.
	blocked := "at://" + author + "/is.currents.feed.save/y1"
	seedImageSave(t, s, blocked, author, alpha, "blob-y", 0.9, testBase)
	if err := s.SetHarmState(ctx, "blob-y", HarmStateBlocked, "auto", ""); err != nil {
		t.Fatalf("SetHarmState: %v", err)
	}
	rows, err = s.GetImageCollectionsPage(ctx, blocked, "", 10, 0)
	if err != nil {
		t.Fatalf("GetImageCollectionsPage blocked: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("blocked blob returned %d collections, want 0", len(rows))
	}
}

// TestGetCollectionByURI pins the scope-join rewrite: save_count rolls up
// section saves, previews prefer direct saves, and unknown URIs return nil.
func TestGetCollectionByURI(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"
	viewer := "did:plc:viewer"

	alpha := "at://" + author + "/is.currents.feed.collection/alpha"
	alphaSec := "at://" + author + "/is.currents.feed.collection/alphasec"
	seedCollection(t, s, alpha, author, "Alpha", "", testBase)
	seedCollection(t, s, alphaSec, author, "Alpha Detail", alpha, testBase)

	seedImageSave(t, s, "at://"+author+"/is.currents.feed.save/a1", author, alpha, "blob-a1", 0.5, testBase)
	seedImageSave(t, s, "at://"+author+"/is.currents.feed.save/s1", author, alphaSec, "blob-s1", 0.9, testBase)

	favURI := "at://" + viewer + "/is.currents.feed.favourite/f1"
	if err := s.UpsertFavourite(ctx, favURI, viewer, alpha); err != nil {
		t.Fatalf("UpsertFavourite: %v", err)
	}

	row, err := s.GetCollectionByURI(ctx, alpha, viewer)
	if err != nil {
		t.Fatalf("GetCollectionByURI: %v", err)
	}
	if row == nil {
		t.Fatal("GetCollectionByURI returned nil for existing collection")
	}
	if row.SaveCount != 2 {
		t.Fatalf("SaveCount = %d, want 2 (direct + section)", row.SaveCount)
	}
	// Direct save first despite the section save's higher quality.
	want := []string{author + ",blob-a1", author + ",blob-s1"}
	if !equalStrings(previewsOf(*row), want) {
		t.Fatalf("previews = %v, want %v", row.PreviewBlobs, want)
	}
	if row.FavouriteCount != 1 || row.FavouriteURI == nil || *row.FavouriteURI != favURI {
		t.Fatalf("favourite hydration = (%d, %v), want (1, %s)", row.FavouriteCount, row.FavouriteURI, favURI)
	}

	section, err := s.GetCollectionByURI(ctx, alphaSec, "")
	if err != nil {
		t.Fatalf("GetCollectionByURI section: %v", err)
	}
	if section == nil || section.ParentURI != alpha || section.SaveCount != 1 {
		t.Fatalf("section row = %+v, want parent alpha with 1 save", section)
	}

	missing, err := s.GetCollectionByURI(ctx, "at://nope", "")
	if err != nil {
		t.Fatalf("GetCollectionByURI missing: %v", err)
	}
	if missing != nil {
		t.Fatalf("missing collection = %+v, want nil", missing)
	}
}

// TestActiveLabelSemantics pins what the label indexes serve: the latest row
// per (src, uri/blob, val) decides activeness, so a negation hides a label and
// a re-application after negation revives it.
func TestActiveLabelSemantics(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := "did:web:labeler.test"
	u1 := "at://did:plc:author/is.currents.feed.save/l1"
	u2 := "at://did:plc:author/is.currents.feed.save/l2"

	insert := func(uri, val string, neg bool, blobCID string) {
		t.Helper()
		if _, err := s.InsertLabel(ctx, LabelRow{Src: src, URI: uri, Val: val, Neg: neg, CTS: testBase, Sig: []byte{1}, Ver: 1, BlobCID: blobCID}); err != nil {
			t.Fatalf("InsertLabel(%s, %s, neg=%t): %v", uri, val, neg, err)
		}
	}
	insert(u1, "porn", false, "blob-1")
	insert(u1, "porn", true, "blob-1") // negated
	insert(u1, "nudity", false, "blob-1")
	insert(u2, "porn", false, "blob-2")
	insert(u2, "porn", true, "blob-2")
	insert(u2, "porn", false, "blob-2") // re-applied after negation

	byURI, err := s.GetLabelsByURIs(ctx, []string{u1, u2})
	if err != nil {
		t.Fatalf("GetLabelsByURIs: %v", err)
	}
	if len(byURI[u1]) != 1 || byURI[u1][0].Val != "nudity" {
		t.Fatalf("u1 labels = %+v, want [nudity] (porn negated)", byURI[u1])
	}
	if len(byURI[u2]) != 1 || byURI[u2][0].Val != "porn" {
		t.Fatalf("u2 labels = %+v, want [porn] (re-applied)", byURI[u2])
	}

	byBlob, err := s.GetActiveLabelsByBlobCIDs(ctx, []string{"blob-1", "blob-2"})
	if err != nil {
		t.Fatalf("GetActiveLabelsByBlobCIDs: %v", err)
	}
	if !equalStrings(byBlob["blob-1"], []string{"nudity"}) || !equalStrings(byBlob["blob-2"], []string{"porn"}) {
		t.Fatalf("labels by blob = %+v, want blob-1:[nudity] blob-2:[porn]", byBlob)
	}
}

// TestDeleteSaveReelection pins DeleteSave's canonical re-election: deleting
// the canonical save promotes the best remaining save of the visual identity,
// and deleting the last save leaves the identity with no canonical.
func TestDeleteSaveReelection(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"

	emb := make([]float32, 768)
	emb[0] = 1
	viID, err := s.CreateVI(ctx, author, "blob-hi", emb, nil)
	if err != nil {
		t.Fatalf("CreateVI: %v", err)
	}

	hiURI := "at://" + author + "/is.currents.feed.save/hi"
	loURI := "at://" + author + "/is.currents.feed.save/lo"
	seedVISave := func(uri, blobCID string, quality float32) {
		t.Helper()
		err := s.UpsertSave(ctx, UpsertSaveParams{
			URI: uri, AuthorDID: author, CollectionURI: "", PdsBlobCID: blobCID,
			ContentNSID: "is.currents.content.image", CreatedAt: &testBase,
			VisualIdentityID: &viID, QualityScore: &quality,
		})
		if err != nil {
			t.Fatalf("seeding save %s: %v", uri, err)
		}
	}
	seedVISave(hiURI, "blob-hi", 0.9)
	seedVISave(loURI, "blob-lo", 0.4)
	if err := s.SetVICanonicalSave(ctx, viID, hiURI); err != nil {
		t.Fatalf("SetVICanonicalSave: %v", err)
	}

	viState := func() (saveCount int, canonicalURI, canonicalCID *string) {
		t.Helper()
		err := s.pool.QueryRow(ctx,
			`SELECT save_count, canonical_save_uri, canonical_blob_cid FROM visual_identity WHERE id = $1`,
			viID).Scan(&saveCount, &canonicalURI, &canonicalCID)
		if err != nil {
			t.Fatalf("reading visual identity: %v", err)
		}
		return
	}

	if count, _, _ := viState(); count != 2 {
		t.Fatalf("save_count = %d, want 2 (trigger-maintained)", count)
	}

	if err := s.DeleteSave(ctx, hiURI); err != nil {
		t.Fatalf("DeleteSave canonical: %v", err)
	}
	count, canonicalURI, canonicalCID := viState()
	if count != 1 || canonicalURI == nil || *canonicalURI != loURI || canonicalCID == nil || *canonicalCID != "blob-lo" {
		t.Fatalf("after deleting canonical: count=%d canonical=%v/%v, want re-election to lo", count, canonicalURI, canonicalCID)
	}

	if err := s.DeleteSave(ctx, loURI); err != nil {
		t.Fatalf("DeleteSave last: %v", err)
	}
	count, canonicalURI, _ = viState()
	if count != 0 || canonicalURI != nil {
		t.Fatalf("after deleting last save: count=%d canonical=%v, want 0/nil", count, canonicalURI)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
