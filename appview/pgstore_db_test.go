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
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
		HiddenDIDs:                []string{}, // nil would encode as SQL NULL in `<> ALL($n)` filters
	})
}

func truncateAll(t *testing.T, s *PgStore) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(), `
		TRUNCATE save, collection, "user", follow, favourite_collection,
			visual_identity, visual_identity_color, cluster, color_trial, seen_feature,
			label, blob_moderation_state, review_item, report, moderation_event,
			import_session
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

// TestGetActorCollectionsPage pins the profile listing: parent="root" returns
// only root collections (sections never surface as their own cards), each
// root's SectionCount reflects its children, and saveCount rolls in section
// saves. Without the parent filter, sections would consume the page budget and
// hide roots on section-heavy accounts.
func TestGetActorCollectionsPage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"

	alpha := "at://" + author + "/is.currents.feed.collection/alpha"
	alphaSec1 := "at://" + author + "/is.currents.feed.collection/alphasec1"
	alphaSec2 := "at://" + author + "/is.currents.feed.collection/alphasec2"
	beta := "at://" + author + "/is.currents.feed.collection/beta"
	seedCollection(t, s, alpha, author, "Alpha", "", testBase.Add(3*time.Hour))
	seedCollection(t, s, alphaSec1, author, "Alpha Detail 1", alpha, testBase.Add(2*time.Hour))
	seedCollection(t, s, alphaSec2, author, "Alpha Detail 2", alpha, testBase.Add(1*time.Hour))
	seedCollection(t, s, beta, author, "Beta", "", testBase)

	saveURI := func(rkey string) string { return "at://" + author + "/is.currents.feed.save/" + rkey }
	seedImageSave(t, s, saveURI("a1"), author, alpha, "blob-a1", 0.9, testBase)
	seedImageSave(t, s, saveURI("s1"), author, alphaSec1, "blob-s1", 0.8, testBase)

	// parent="root": only roots, with section counts and rolled-up save counts.
	roots, _, err := s.GetActorCollectionsPage(ctx, author, "", "root", 50, "")
	if err != nil {
		t.Fatalf("GetActorCollectionsPage root: %v", err)
	}
	byURI := map[string]CollectionRow{}
	for _, r := range roots {
		byURI[r.URI] = r
	}
	if len(roots) != 2 {
		t.Fatalf("root page returned %d collections, want 2 (roots only)", len(roots))
	}
	if a := byURI[alpha]; a.SectionCount != 2 || a.SaveCount != 2 {
		t.Fatalf("Alpha = {sections:%d, saves:%d}, want {2, 2}", a.SectionCount, a.SaveCount)
	}
	if b := byURI[beta]; b.SectionCount != 0 || b.SaveCount != 0 {
		t.Fatalf("Beta = {sections:%d, saves:%d}, want {0, 0}", b.SectionCount, b.SaveCount)
	}

	// No parent filter: roots and sections both appear.
	all, _, err := s.GetActorCollectionsPage(ctx, author, "", "", 50, "")
	if err != nil {
		t.Fatalf("GetActorCollectionsPage all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("unfiltered page returned %d collections, want 4 (roots + sections)", len(all))
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
	viID, err := s.CreateVI(ctx, author, "blob-hi", emb, nil, nil)
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

// TestGlobalFeedJunkFilter pins the feed junk gate: identities scoring at or
// above feedJunkScoreMax are excluded from the global feed, while unscored
// (NULL) and low-scored identities pass; SetVIJunkScoreIfNull fills gaps but
// never overwrites an existing score.
func TestGlobalFeedJunkFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"

	emb := make([]float32, 768)
	emb[0] = 1
	f32 := func(v float32) *float32 { return &v }

	mkImage := func(rkey string, junk *float32) (saveURI, viID string) {
		t.Helper()
		saveURI = "at://" + author + "/is.currents.feed.save/" + rkey
		viID, err := s.CreateVI(ctx, author, "blob-"+rkey, emb, nil, junk)
		if err != nil {
			t.Fatalf("CreateVI(%s): %v", rkey, err)
		}
		quality := float32(0.5)
		err = s.UpsertSave(ctx, UpsertSaveParams{
			URI: saveURI, AuthorDID: author, CollectionURI: "", PdsBlobCID: "blob-" + rkey,
			ContentNSID: "is.currents.content.image", CreatedAt: &testBase,
			VisualIdentityID: &viID, QualityScore: &quality,
		})
		if err != nil {
			t.Fatalf("seeding save %s: %v", rkey, err)
		}
		if err := s.SetVICanonicalSave(ctx, viID, saveURI); err != nil {
			t.Fatalf("SetVICanonicalSave(%s): %v", rkey, err)
		}
		return saveURI, viID
	}

	cleanURI, _ := mkImage("clean", f32(0.1))
	unscoredURI, unscoredVI := mkImage("unscored", nil)
	junkURI, junkVI := mkImage("junk", f32(0.9))

	feedURIs := func() map[string]bool {
		t.Helper()
		rows, err := s.GetGlobalFeedSaves(ctx, "", false, 10, 0)
		if err != nil {
			t.Fatalf("GetGlobalFeedSaves: %v", err)
		}
		out := map[string]bool{}
		for _, r := range rows {
			out[r.URI] = true
		}
		return out
	}

	got := feedURIs()
	if !got[cleanURI] || !got[unscoredURI] {
		t.Fatalf("feed %v must contain clean and unscored images", got)
	}
	if got[junkURI] {
		t.Fatalf("feed %v must exclude the junk-scored image", got)
	}

	// IfNull never overwrites: the junk image stays hidden.
	if err := s.SetVIJunkScoreIfNull(ctx, junkVI, 0.05); err != nil {
		t.Fatalf("SetVIJunkScoreIfNull(junk): %v", err)
	}
	if got := feedURIs(); got[junkURI] {
		t.Fatal("SetVIJunkScoreIfNull overwrote an existing score")
	}

	// IfNull fills gaps: scoring the unscored image as junk hides it.
	if err := s.SetVIJunkScoreIfNull(ctx, unscoredVI, 0.95); err != nil {
		t.Fatalf("SetVIJunkScoreIfNull(unscored): %v", err)
	}
	if got := feedURIs(); got[unscoredURI] {
		t.Fatal("junk-scored image still in the feed after SetVIJunkScoreIfNull")
	}

	// The unconditional setter (backfill path) can bring it back.
	if err := s.SetVIJunkScore(ctx, unscoredVI, 0.1); err != nil {
		t.Fatalf("SetVIJunkScore: %v", err)
	}
	if got := feedURIs(); !got[unscoredURI] {
		t.Fatal("re-scored image missing from the feed after SetVIJunkScore")
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

// TestEarliestSaveCreatedAt pins the timestamp a resave inherits: the earliest
// created_at among the *viewer's own* saves of that blob. Scoped by author so a
// resave of someone else's image doesn't backdate to their save time, and nil
// when the viewer has no copy (a genuinely new save stamps "now").
func TestEarliestSaveCreatedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	viewer := "did:plc:viewer"
	other := "did:plc:other"
	saveURI := func(did, rkey string) string { return "at://" + did + "/is.currents.feed.save/" + rkey }
	colA := "at://" + viewer + "/is.currents.feed.collection/a"
	colB := "at://" + viewer + "/is.currents.feed.collection/b"

	// Viewer has the same image in two collections, saved at different times.
	first := testBase.Add(1 * time.Hour)
	second := testBase.Add(5 * time.Hour)
	seedImageSave(t, s, saveURI(viewer, "v1"), viewer, colA, "blob-shared", 0.5, second)
	seedImageSave(t, s, saveURI(viewer, "v2"), viewer, colB, "blob-shared", 0.5, first)
	// Another user saved the same blob even earlier — must not affect the viewer's result.
	seedImageSave(t, s, saveURI(other, "o1"), other, colA, "blob-shared", 0.5, testBase)

	got, err := s.EarliestSaveCreatedAt(ctx, viewer, "blob-shared")
	if err != nil {
		t.Fatalf("EarliestSaveCreatedAt: %v", err)
	}
	if got == nil || !got.Equal(first) {
		t.Fatalf("got %v, want %v (viewer's earliest, ignoring the other user's)", got, first)
	}

	// A blob the viewer has never saved → nil, so the resave stamps a fresh time.
	none, err := s.EarliestSaveCreatedAt(ctx, viewer, "blob-unknown")
	if err != nil {
		t.Fatalf("EarliestSaveCreatedAt(unknown): %v", err)
	}
	if none != nil {
		t.Fatalf("got %v, want nil for a blob the viewer doesn't have", none)
	}
}

// TestSaveMimeTypeRoundTrip pins that a save's blob mime type survives the
// upsert → GetSavesByURIs path (the field the web client reads to freeze GIFs),
// and that a save whose record omits it reads back as "".
func TestSaveMimeTypeRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"
	createdAt := testBase.Add(time.Hour)

	if err := s.UpsertSave(ctx, UpsertSaveParams{
		URI:         "at://" + author + "/is.currents.feed.save/gif",
		AuthorDID:   author,
		PdsBlobCID:  "blob-gif",
		ContentNSID: "is.currents.content.image",
		CreatedAt:   &createdAt,
		MimeType:    "image/gif",
	}); err != nil {
		t.Fatalf("upserting gif save: %v", err)
	}
	if err := s.UpsertSave(ctx, UpsertSaveParams{
		URI:         "at://" + author + "/is.currents.feed.save/jpg",
		AuthorDID:   author,
		PdsBlobCID:  "blob-jpg",
		ContentNSID: "is.currents.content.image",
		CreatedAt:   &createdAt,
	}); err != nil {
		t.Fatalf("upserting jpg save: %v", err)
	}

	rows, err := s.GetSavesByURIs(ctx,
		[]string{"at://" + author + "/is.currents.feed.save/gif", "at://" + author + "/is.currents.feed.save/jpg"}, "")
	if err != nil {
		t.Fatalf("GetSavesByURIs: %v", err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.BlobCID] = r.MimeType
	}
	if got["blob-gif"] != "image/gif" {
		t.Fatalf("gif mime = %q, want image/gif", got["blob-gif"])
	}
	if got["blob-jpg"] != "" {
		t.Fatalf("jpg mime = %q, want empty (record omitted it)", got["blob-jpg"])
	}
}

// TestUserPrefs pins the general-preferences get/set semantics: a user with no
// row is on the defaults (gifAutoplay on), and a stored value round-trips.
func TestUserPrefs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	did := "did:plc:prefsuser"

	got, err := s.GetUserPrefs(ctx, did)
	if err != nil {
		t.Fatalf("GetUserPrefs(default): %v", err)
	}
	if !got.GifAutoplay {
		t.Fatalf("default gifAutoplay = %v, want true", got.GifAutoplay)
	}

	if err := s.SetUserPrefs(ctx, did, UserPrefs{GifAutoplay: false}); err != nil {
		t.Fatalf("SetUserPrefs: %v", err)
	}
	got, err = s.GetUserPrefs(ctx, did)
	if err != nil {
		t.Fatalf("GetUserPrefs(stored): %v", err)
	}
	if got.GifAutoplay {
		t.Fatalf("stored gifAutoplay = %v, want false", got.GifAutoplay)
	}

	// Upsert path: flipping back updates the existing row rather than erroring.
	if err := s.SetUserPrefs(ctx, did, UserPrefs{GifAutoplay: true}); err != nil {
		t.Fatalf("SetUserPrefs(update): %v", err)
	}
	got, _ = s.GetUserPrefs(ctx, did)
	if !got.GifAutoplay {
		t.Fatalf("updated gifAutoplay = %v, want true", got.GifAutoplay)
	}
}

// The color-search trial ledger: one row per color spent, scoped per viewer,
// and idempotent so a repeat of the same color can't drain the allowance.
func TestColorTrialLedger(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const did, other = "did:plc:trial", "did:plc:othertrial"

	colors, err := s.ColorTrialColors(ctx, did)
	if err != nil {
		t.Fatalf("ColorTrialColors(empty): %v", err)
	}
	if len(colors) != 0 {
		t.Fatalf("ColorTrialColors(empty) = %v, want none", colors)
	}

	for _, hex := range []string{"#e63946", "#2a9d8f", "#e63946"} {
		if err := s.RecordColorTrial(ctx, did, hex); err != nil {
			t.Fatalf("RecordColorTrial(%s): %v", hex, err)
		}
	}
	if err := s.RecordColorTrial(ctx, other, "#457b9d"); err != nil {
		t.Fatalf("RecordColorTrial(other): %v", err)
	}

	colors, err = s.ColorTrialColors(ctx, did)
	if err != nil {
		t.Fatalf("ColorTrialColors: %v", err)
	}
	// The repeat conflicts away, and the other viewer's color stays theirs.
	want := map[string]bool{"#e63946": true, "#2a9d8f": true}
	if len(colors) != len(want) {
		t.Fatalf("ColorTrialColors = %v, want %v", colors, want)
	}
	for _, hex := range colors {
		if !want[hex] {
			t.Errorf("ColorTrialColors returned unexpected %q", hex)
		}
	}
}

// TestHybridColorGate pins the deliberate asymmetry between the two color
// paths. Hybrid orders by semantics alone, so its gate is its only color
// signal and runs stricter on both axes: a tiny accent of the query color is
// excluded even when it's the closest semantic match, and so is a
// well-covered but visibly different hue. Pure color search ranks by
// ΔE − colorCoverageWeight·fraction, which demotes both cases without dropping
// them, so it keeps all three and orders them best-match first.
func TestHybridColorGate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	author := "did:plc:author"
	const queryHex = "#e03131"

	// The accent image is the nearer semantic match, so without the coverage
	// floor it would take the top hybrid slot.
	queryEmb := make([]float32, 768)
	queryEmb[0] = 1
	accentEmb := make([]float32, 768)
	accentEmb[0] = 1
	dominantEmb := make([]float32, 768)
	dominantEmb[0], dominantEmb[1] = 1, 1

	collURI := "at://" + author + "/is.currents.feed.collection/palette"
	seedCollection(t, s, collURI, author, "palette", "", testBase)

	mkImage := func(rkey string, emb []float32, palette string) string {
		t.Helper()
		saveURI := "at://" + author + "/is.currents.feed.save/" + rkey
		viID, err := s.CreateVI(ctx, author, "blob-"+rkey, emb, nil, nil)
		if err != nil {
			t.Fatalf("CreateVI(%s): %v", rkey, err)
		}
		quality := float32(0.5)
		err = s.UpsertSave(ctx, UpsertSaveParams{
			URI: saveURI, AuthorDID: author, CollectionURI: collURI, PdsBlobCID: "blob-" + rkey,
			ContentNSID: "is.currents.content.image", CreatedAt: &testBase,
			VisualIdentityID: &viID, QualityScore: &quality,
		})
		if err != nil {
			t.Fatalf("seeding save %s: %v", rkey, err)
		}
		if err := s.SetVICanonicalSave(ctx, viID, saveURI); err != nil {
			t.Fatalf("SetVICanonicalSave(%s): %v", rkey, err)
		}
		if err := s.SetVIColors(ctx, viID, json.RawMessage(palette)); err != nil {
			t.Fatalf("SetVIColors(%s): %v", rkey, err)
		}
		return saveURI
	}

	// Same query color in both palettes at the same ΔE (0) — only coverage differs.
	accentURI := mkImage("accent", accentEmb,
		`[{"hex":"#101010","fraction":0.94},{"hex":"`+queryHex+`","fraction":0.02}]`)
	dominantURI := mkImage("dominant", dominantEmb,
		`[{"hex":"`+queryHex+`","fraction":0.5},{"hex":"#101010","fraction":0.4}]`)
	// Plenty of coverage, but a visibly different hue: burnt orange sits in the
	// band that pure color search still admits and hybrid no longer does.
	farURI := mkImage("far", dominantEmb,
		`[{"hex":"#be4614","fraction":0.6},{"hex":"#101010","fraction":0.3}]`)

	lab, err := hexToLab(queryHex)
	if err != nil {
		t.Fatalf("hexToLab: %v", err)
	}
	// State the premise rather than trusting the fixture: #be4614 has to land
	// between the two thresholds for this test to mean anything.
	farLab, err := hexToLab("#be4614")
	if err != nil {
		t.Fatalf("hexToLab(far): %v", err)
	}
	farDE := math.Sqrt(float64((farLab[0]-lab[0])*(farLab[0]-lab[0]) +
		(farLab[1]-lab[1])*(farLab[1]-lab[1]) + (farLab[2]-lab[2])*(farLab[2]-lab[2])))
	if farDE <= colorHybridMaxDeltaE || farDE > colorMaxDeltaE {
		t.Fatalf("fixture ΔE = %.1f, need it in (%.0f, %.0f] to separate the two gates",
			farDE, colorHybridMaxDeltaE, colorMaxDeltaE)
	}

	uris := func(page annSavePage) []string {
		out := make([]string, len(page.Rows))
		for i, r := range page.Rows {
			out[i] = r.URI
		}
		return out
	}

	hybrid, err := s.SearchHybridSavesPage(ctx, queryEmb, lab, "", false, nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchHybridSavesPage: %v", err)
	}
	if got := uris(hybrid); !equalStrings(got, []string{dominantURI}) {
		t.Fatalf("hybrid = %v, want only %v (the accent match is below the coverage floor)", got, dominantURI)
	}

	// The scoped variants rebind the collection placeholder, so exercise both.
	lib, err := s.SearchHybridSavesPage(ctx, queryEmb, lab, author, true, nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchHybridSavesPage(library): %v", err)
	}
	if got := uris(lib); !equalStrings(got, []string{dominantURI}) {
		t.Fatalf("hybrid library = %v, want only %v", got, dominantURI)
	}

	scoped, err := s.SearchHybridSavesPage(ctx, queryEmb, lab, author, true, []string{collURI}, 10, 0)
	if err != nil {
		t.Fatalf("SearchHybridSavesPage(collections): %v", err)
	}
	if got := uris(scoped); !equalStrings(got, []string{dominantURI}) {
		t.Fatalf("hybrid collections = %v, want only %v", got, dominantURI)
	}

	color, err := s.SearchSavesByColorPage(ctx, lab, "", false, nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchSavesByColorPage: %v", err)
	}
	if got := uris(color); !equalStrings(got, []string{dominantURI, accentURI, farURI}) {
		t.Fatalf("color search = %v, want %v (both gates are hybrid-only; the ranking demotes rather than excludes)",
			got, []string{dominantURI, accentURI, farURI})
	}
}

// Sign-up pre-dismisses the announcements outside the onboarding set, so a
// fresh account isn't greeted by a wall of red dots.
func TestSeedSeenFeatures(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const did, other = "did:plc:newuser", "did:plc:veteran"

	// A veteran already dismissed one thing by hand; seeding another account
	// must not touch them.
	if err := s.MarkFeatureSeen(ctx, other, "organize-mode"); err != nil {
		t.Fatalf("MarkFeatureSeen: %v", err)
	}

	if err := s.SeedSeenFeatures(ctx, did, onboardingSeenFeatures); err != nil {
		t.Fatalf("SeedSeenFeatures: %v", err)
	}
	// Re-running (a re-login after a PDS profile wipe) is a no-op, not an error.
	if err := s.SeedSeenFeatures(ctx, did, onboardingSeenFeatures); err != nil {
		t.Fatalf("SeedSeenFeatures(repeat): %v", err)
	}

	seen, err := s.GetSeenFeatures(ctx, did)
	if err != nil {
		t.Fatalf("GetSeenFeatures: %v", err)
	}
	if len(seen) != len(onboardingSeenFeatures) {
		t.Fatalf("seeded %v, want %v", seen, onboardingSeenFeatures)
	}
	got := map[string]bool{}
	for _, k := range seen {
		got[k] = true
	}
	for _, k := range onboardingSeenFeatures {
		if !got[k] {
			t.Errorf("key %q not seeded", k)
		}
	}
	// The onboarding set is what stays lit: these must NOT be pre-dismissed.
	for _, k := range []string{"pinterest-import", "organize-mode"} {
		if got[k] {
			t.Errorf("key %q was pre-dismissed, want it kept as onboarding", k)
		}
	}

	if veteran, err := s.GetSeenFeatures(ctx, other); err != nil {
		t.Fatalf("GetSeenFeatures(other): %v", err)
	} else if len(veteran) != 1 || veteran[0] != "organize-mode" {
		t.Errorf("veteran flags = %v, want only organize-mode", veteran)
	}
}

// Pinterest's logged-out board feed omits pins that the board genuinely holds,
// so a job's listed item count can fall short of the board's pin_count. The
// job carries that count through to GetSessionStatus as Expected so the UI can
// report the shortfall instead of presenting a partial import as complete.
// Section jobs and section-filtered board jobs cover a subset of the board by
// design, so they record 0 ("unknown") rather than a count to compare against.
func TestImportJobExpectedCount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const did = "did:plc:importer"
	sessionID := "11111111-1111-1111-1111-111111111111"

	if err := s.UpsertImportSession(ctx, sessionID, did, "becksburg04"); err != nil {
		t.Fatalf("UpsertImportSession: %v", err)
	}

	newJob := func(name string, expected int) string {
		t.Helper()
		id, err := s.CreateImportJob(ctx, ImportJobRow{
			SessionID: sessionID, OwnerDID: did, OAuthSessionID: "sess", Source: "pinterest",
			SourceBoardID: "b1", SourceBoardName: name, SourceBoardURL: "/u/b/",
			ExpectedCount: expected, TargetCollectionURI: "at://" + did + "/is.currents.feed.collection/c1",
		})
		if err != nil {
			t.Fatalf("CreateImportJob(%s): %v", name, err)
		}
		return id
	}

	// A whole-board job: the board reports 40 pins, the feed yielded only 37.
	boardJob := newJob("Beach/Swimsuit", 40)
	pins := make([]PinterestPin, 0, 37)
	for i := 0; i < 37; i++ {
		pins = append(pins, PinterestPin{ID: fmt.Sprintf("pin%d", i), ImageURL: "https://i.pinimg.com/x.jpg"})
	}
	if n, err := s.BulkInsertImportItems(ctx, boardJob, did, pins); err != nil || n != 37 {
		t.Fatalf("BulkInsertImportItems = %d, %v; want 37, nil", n, err)
	}
	// A section job covers part of the board, so it records no expectation.
	sectionJob := newJob("Beach/Swimsuit › Bikinis", 0)

	rows, err := s.GetSessionStatus(ctx, sessionID, did)
	if err != nil {
		t.Fatalf("GetSessionStatus: %v", err)
	}
	byID := map[string]SessionJobStatus{}
	for _, r := range rows {
		byID[r.JobID] = r
	}
	if len(byID) != 2 {
		t.Fatalf("got %d jobs, want 2", len(byID))
	}
	if got := byID[boardJob]; got.Expected != 40 || got.Queued != 37 {
		t.Errorf("board job: expected=%d queued=%d, want 40 and 37", got.Expected, got.Queued)
	}
	if got := byID[sectionJob]; got.Expected != 0 {
		t.Errorf("section job: expected=%d, want 0 (not comparable)", got.Expected)
	}
}
