package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/gorilla/sessions"
)

const inboxTestDID = "did:plc:owner"
const inboxTestSave = "at://" + inboxTestDID + "/is.currents.feed.save/current"

func testBlueskySubject(uri, cid string) blueskySubject {
	row := blueskySubject{Src: blueskyLabelerDID, Updated: time.Now().UTC()}
	row.Subject.URI, row.Subject.CID = uri, cid
	return row
}

func TestBlueskyInboxScopesAndOptIn(t *testing.T) {
	store := oauth.NewMemStore()
	cfg := oauth.NewPublicConfig("https://staging.example/oauth-client-metadata.json", "https://staging.example/oauth/callback", []string{"atproto"})
	s := Server{OAuth: oauth.NewClientApp(&cfg, store), CookieStore: sessions.NewCookieStore([]byte("test-secret"))}
	res := httptest.NewRecorder()
	s.ClientMetadata(res, httptest.NewRequest(http.MethodGet, cfg.ClientID, nil))
	var meta oauth.ClientMetadata
	if err := json.Unmarshal(res.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Scopes, []string{"atproto"}) || meta.Scope != "atproto "+blueskyListScope+" "+blueskyDetailScope {
		t.Fatalf("normal login scopes %v, declared scopes %s", cfg.Scopes, meta.Scope)
	}

	req := httptest.NewRequest(http.MethodGet, "https://staging.example/api/me/bluesky-moderation", nil)
	res = httptest.NewRecorder()
	s.APIMeBlueskyModeration(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("inbox without session = %d", res.Code)
	}
	token, err := s.encodeSessionToken(inboxTestDID, "missing-session", "test.example")
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	s.APIMeBlueskyModeration(res, req)
	var page blueskyInboxPage
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || page.Status != "connect" {
		t.Fatalf("missing OAuth session = %s, %v", res.Body.String(), err)
	}

	res = httptest.NewRecorder()
	s.completeBlueskyModerationConsent(res, req, nil)
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != "/explore/general?bluesky-moderation=session-changed" || len(res.Result().Cookies()) != 0 {
		t.Fatalf("unbound consent should leave the current session alone: %v", res.Result())
	}
}

func TestBlueskyInboxConsentRenewsExpiredSession(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()
	cfg := oauth.NewPublicConfig("https://staging.example/oauth-client-metadata.json", "https://staging.example/oauth/callback", []string{"atproto", "repo:is.currents.feed.save"})
	app := oauth.NewClientApp(&cfg, db)
	did := syntax.DID(inboxTestDID)
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID: did,
		Services: map[string]identity.ServiceEndpoint{
			"atproto_pds": {Type: "AtprotoPersonalDataServer", URL: "https://pds.example"},
		},
	})
	app.Dir = dir
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	s := Server{OAuth: app, CookieStore: sessions.NewCookieStore([]byte("test-secret"))}
	signedCookie, err := s.encodeSessionToken(inboxTestDID, "old-session", "test.example")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.SaveSession(ctx, oauth.ClientSessionData{AccountDID: did, SessionID: "old-session"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, "UPDATE oauth_sessions SET updated_at = now() - interval '2 hours' WHERE session_id = 'old-session' AND account_did = $1", did.String()); err != nil {
		t.Fatal(err)
	}
	scopes := withBlueskyInboxReadScopes(cfg.Scopes)
	for _, tc := range []struct {
		name       string
		granted    []string
		oauthError string
		outcome    string
		exchanges  int32
	}{
		{"approved", scopes, "", "connected", 1},
		{"partial", scopes[:len(scopes)-1], "", "incomplete", 1},
		{"declined", nil, "access_denied", "denied", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var exchanges atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				exchanges.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/token" || r.FormValue("code") != "approved-code" {
					t.Errorf("unexpected OAuth token request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(oauth.TokenResponse{Subject: did.String(), Scope: strings.Join(tc.granted, " ")})
			}))
			defer provider.Close()
			info := oauth.AuthRequestData{
				State: tc.name, AccountDID: &did, Scopes: scopes, AuthServerURL: provider.URL,
				AuthServerTokenEndpoint: provider.URL + "/token", DPoPPrivateKeyMultibase: key.Multibase(),
			}
			if err := db.SaveAuthRequestInfo(ctx, info); err != nil {
				t.Fatal(err)
			}
			params := url.Values{"state": {info.State}, "code": {"approved-code"}, "iss": {provider.URL}}
			if tc.oauthError != "" {
				params.Del("code")
				params.Set("error", tc.oauthError)
			}
			req := httptest.NewRequest(http.MethodGet, cfg.CallbackURL+"?"+params.Encode(), nil)
			req.AddCookie(&http.Cookie{Name: "currents-session", Value: signedCookie})
			res := httptest.NewRecorder()
			s.OAuthCallback(res, req)
			if res.Code != http.StatusSeeOther || !strings.HasSuffix(res.Header().Get("Location"), "bluesky-moderation="+tc.outcome) {
				t.Fatalf("consent result = %d %s", res.Code, res.Header().Get("Location"))
			}
			if got := exchanges.Load(); got != tc.exchanges {
				t.Fatalf("provider received %d token exchanges, want %d", got, tc.exchanges)
			}
			cookies := res.Result().Cookies()
			if tc.outcome != "connected" {
				if len(cookies) != 0 {
					t.Fatal("unsuccessful consent changed the browser session")
				}
				return
			}
			if len(cookies) != 1 {
				t.Fatalf("approved consent sent %d cookies, want one", len(cookies))
			}
			followup := httptest.NewRequest(http.MethodGet, cfg.CallbackURL, nil)
			followup.AddCookie(cookies[0])
			account, sessionID, _ := s.sessionDIDFromCookie(followup)
			if account == nil || *account != did || sessionID != info.State {
				t.Fatal("approved consent failed to replace the expired session")
			}
			grant, err := db.GetSession(ctx, did, sessionID)
			if err != nil || !slices.Equal(grant.Scopes, scopes) {
				t.Fatalf("fresh inbox grant was not stored: %v", err)
			}
		})
	}
}

func TestBlueskyInboxProxyFiltersAndDeduplicates(t *testing.T) {
	record := testBlueskySubject(inboxTestSave, "record-cid")
	newer := record
	newer.Updated = record.Updated.Add(time.Second)
	newer.IsRead = true
	post := testBlueskySubject("at://"+inboxTestDID+"/app.bsky.feed.post/post", "another")
	foreign := testBlueskySubject("at://did:plc:other/is.currents.feed.save/record", "foreign")
	wrongIssuer := testBlueskySubject("at://"+inboxTestDID+"/is.currents.feed.save/foreign-labeler", "x")
	wrongIssuer.Src = "did:plc:other"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Atproto-Proxy") != blueskyLabelerService || r.URL.Query().Has("did") {
			t.Errorf("unexpected subject lookup: %s", r.URL)
		}
		switch r.URL.Path {
		case "/xrpc/" + blueskyListSubjects:
			_ = json.NewEncoder(w).Encode(map[string]any{"subjects": []blueskySubject{post, record, foreign, newer, wrongIssuer}, "cursor": "older"})
		case "/xrpc/" + blueskyGetSubject:
			if r.URL.Query().Get("subject") != inboxTestSave {
				t.Error("requested details for wrong subject")
			}
			_ = json.NewEncoder(w).Encode(struct {
				blueskySubject
				Actions []blueskyAction `json:"actions"`
			}{record, []blueskyAction{{Type: "labelApplied", Labels: []string{"porn"}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	client := atclient.NewAPIClient(upstream.URL)
	rows, cursor, err := getBlueskyInboxSubjects(context.Background(), client, inboxTestDID, "", "")
	if err != nil || len(rows) != 1 || !rows[0].IsRead || rows[0].Subject.URI != inboxTestSave || cursor != "older" {
		t.Fatalf("inbox rows=%+v cursor=%q err=%v", rows, cursor, err)
	}
	rows, _, err = getBlueskyInboxSubjects(context.Background(), client, inboxTestDID, "", inboxTestSave)
	if err != nil || len(rows) != 1 || rows[0].LatestAction == nil || rows[0].LatestAction.Type != "labelApplied" {
		t.Fatalf("subject details=%+v err=%v", rows, err)
	}
}

func TestBlueskyInboxErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		name   string
		want   string
	}{
		{http.StatusUnauthorized, "AuthMissing", "permission-denied"},
		{http.StatusForbidden, "ScopeMissing", "permission-denied"},
		{http.StatusBadRequest, "MethodNotImplemented", "unsupported"},
		{http.StatusServiceUnavailable, "", "unavailable"},
	} {
		if got := blueskyInboxError(&atclient.APIError{StatusCode: tc.status, Name: tc.name}); got != tc.want {
			t.Errorf("HTTP %d %s = %q, want %q", tc.status, tc.name, got, tc.want)
		}
	}
}

func TestBlueskyInboxMatchesExactSaveAndRespectsTakedown(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()
	s := Server{Store: db, CDNBaseURL: "https://staging.example"}
	p := UpsertSaveParams{URI: inboxTestSave, AuthorDID: inboxTestDID, PdsBlobCID: "image-cid", RecordCID: "save-version", ContentNSID: saveContentImageNSID}
	if err := db.UpsertSave(ctx, p); err != nil {
		t.Fatal(err)
	}

	row := testBlueskySubject(inboxTestSave, p.RecordCID)
	missing := testBlueskySubject("at://"+inboxTestDID+"/is.currents.feed.save/missing", "same-version")
	checkPreview := func(want string) {
		t.Helper()
		notices, err := s.matchBlueskyModerationNotices(ctx, inboxTestDID, []blueskySubject{row, missing})
		if err != nil || len(notices) != 2 || notices[0].PreviewURL != want || notices[1].PreviewURL != "" {
			t.Fatalf("notices=%+v err=%v, expected preview %q only for the matching subject", notices, err, want)
		}
	}
	checkPreview(s.CDNBaseURL + "/img/" + inboxTestDID + "/" + p.PdsBlobCID)
	row.Subject.CID = "historical-version"
	checkPreview("")
	row.Subject.CID = p.RecordCID
	if err := db.SetHarmState(ctx, p.PdsBlobCID, HarmStateBlocked, "moderator", "test"); err != nil {
		t.Fatal(err)
	}
	checkPreview("")
}
