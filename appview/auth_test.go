package main

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/gorilla/sessions"
)

func TestOAuthLoginKeepsReturnCookieOnCallbackHost(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for _, returnTo := range []string{"currents://oauth-callback", "is.currents.app://oauth-callback"} {
			t.Run(method+"/"+returnTo, func(t *testing.T) {
				config := oauth.NewPublicConfig(
					"https://currents.is/oauth-client-metadata.json",
					"https://currents.is/oauth/callback",
					[]string{"atproto"},
				)
				store := sessions.NewCookieStore([]byte("test-secret"))
				store.Options = &sessions.Options{Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
				srv := Server{CookieStore: store, OAuth: oauth.NewClientApp(&config, oauth.NewMemStore())}
				// An invalid identifier stops before contacting a PDS, after the return
				// cookie has been written on the canonical login host.
				values := url.Values{"username": {"!"}, "return_to": {returnTo}}
				loginURL := "https://api.currents.is/oauth/login"
				body := ""
				if method == http.MethodGet {
					loginURL += "?" + values.Encode()
				} else {
					body = values.Encode()
				}
				req := httptest.NewRequest(method, loginURL, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				res := httptest.NewRecorder()
				srv.OAuthLogin(res, req)
				if res.Code != http.StatusTemporaryRedirect {
					t.Fatalf("API login status = %d, want 307", res.Code)
				}
				if len(res.Result().Cookies()) != 0 {
					t.Fatal("return cookie must not be set on the API host")
				}
				target, err := res.Result().Location()
				if err != nil {
					t.Fatal(err)
				}
				if target.Scheme != "https" || target.Host != "currents.is" || target.Path != "/oauth/login" || target.RawQuery != req.URL.RawQuery {
					t.Fatalf("unexpected canonical login URL: %s", target)
				}

				req = httptest.NewRequest(method, target.String(), strings.NewReader(body))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				res = httptest.NewRecorder()
				srv.OAuthLogin(res, req)
				if res.Code != http.StatusBadRequest {
					t.Fatalf("canonical login status = %d, want invalid-identifier 400", res.Code)
				}
				jar, _ := cookiejar.New(nil)
				jar.SetCookies(target, res.Result().Cookies())
				callback := httptest.NewRequest(http.MethodGet, config.CallbackURL, nil)
				for _, cookie := range jar.Cookies(callback.URL) {
					callback.AddCookie(cookie)
				}
				sess, err := store.Get(callback, "currents-session")
				if err != nil {
					t.Fatal(err)
				}
				if got := sess.Values["return_to"]; got != returnTo {
					t.Fatalf("callback return_to = %v, want %s", got, returnTo)
				}
			})
		}
	}
}

func TestMobileCallbackSessionToken(t *testing.T) {
	srv := Server{
		CookieStore:           sessions.NewCookieStore([]byte("test-secret")),
		MobileRedirectSchemes: splitCSV(defaultMobileRedirectSchemes),
	}
	for _, returnTo := range []string{"currents://oauth-callback", "is.currents.app://oauth-callback"} {
		t.Run(returnTo, func(t *testing.T) {
			if !srv.isMobileReturnTo(returnTo) {
				t.Fatalf("default server rejects %s", returnTo)
			}
			token, err := srv.encodeSessionToken("did:plc:test", "session-id", "test.bsky.social")
			if err != nil {
				t.Fatal(err)
			}
			callback, err := url.Parse(appendTokenParams(returnTo, token, "test.bsky.social"))
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "https://api.currents.is/api/me", nil)
			req.Header.Set("Authorization", "Bearer "+callback.Query().Get("token"))
			did, sid, handle := srv.currentSessionDID(req)
			if did == nil || did.String() != "did:plc:test" || sid != "session-id" || handle != "test.bsky.social" {
				t.Fatalf("native session = %v, %q, %q", did, sid, handle)
			}
			if callback.Query().Get("handle") != handle {
				t.Fatalf("callback handle = %q", callback.Query().Get("handle"))
			}
		})
	}
	for _, returnTo := range []string{"https://example.com/oauth-callback", "other://oauth-callback"} {
		if srv.isMobileReturnTo(returnTo) {
			t.Errorf("unexpectedly allowed %s", returnTo)
		}
	}
}

func TestExtensionSessionBridgeSetsAPIHostCookie(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret"))
	store.Options = &sessions.Options{Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
	srv := Server{
		CookieStore: store,
		FrontendURL: "https://currents.is",
		ServiceURL:  "https://api.currents.is",
	}
	rootURL, _ := url.Parse("https://currents.is/login/success")
	apiURL, _ := url.Parse("https://api.currents.is/api/me")
	jar, _ := cookiejar.New(nil)
	rootReq := httptest.NewRequest(http.MethodGet, rootURL.String(), nil)
	rootSession, _ := store.Get(rootReq, "currents-session")
	rootSession.Values = map[any]any{
		"account_did": "did:plc:test", "session_id": "session-id", "handle": "test.bsky.social",
	}
	rootRes := httptest.NewRecorder()
	if err := rootSession.Save(rootReq, rootRes); err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(rootURL, rootRes.Result().Cookies())
	if got := jar.Cookies(apiURL); len(got) != 0 {
		t.Fatalf("root-host cookie reached API host before bridge: %v", got)
	}

	token, err := srv.encodeSessionToken("did:plc:test", "session-id", "test.bsky.social")
	if err != nil {
		t.Fatal(err)
	}
	bridgePage := httptest.NewRecorder()
	srv.serveExtensionSessionBridge(bridgePage, token)
	if got := bridgePage.Header().Get("Referrer-Policy"); got != "origin" {
		t.Fatalf("bridge referrer policy = %q, want origin for the form POST", got)
	}
	if !strings.Contains(bridgePage.Body.String(), `action="https://api.currents.is/oauth/extension-session"`) ||
		!strings.Contains(bridgePage.Body.String(), `name="token" value="`+token+`"`) {
		t.Fatalf("extension bridge page does not submit the session to the API host: %s", bridgePage.Body.String())
	}

	form := url.Values{"token": {token}}
	req := httptest.NewRequest(http.MethodPost, "https://api.currents.is/oauth/extension-session", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://currents.is")
	res := httptest.NewRecorder()
	srv.OAuthExtensionSession(res, req)
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != rootURL.String() {
		t.Fatalf("bridge response = %d, %q", res.Code, res.Header().Get("Location"))
	}
	for _, cookie := range res.Result().Cookies() {
		if cookie.Domain != "" {
			t.Fatalf("API cookie must be host-only: %s", cookie)
		}
	}
	jar.SetCookies(apiURL, res.Result().Cookies())
	apiReq := httptest.NewRequest(http.MethodGet, apiURL.String(), nil)
	for _, cookie := range jar.Cookies(apiURL) {
		apiReq.AddCookie(cookie)
	}
	did, sid, handle := srv.currentSessionDID(apiReq)
	if did == nil || did.String() != "did:plc:test" || sid != "session-id" || handle != "test.bsky.social" {
		t.Fatalf("API session = %v, %q, %q", did, sid, handle)
	}
	if got := jar.Cookies(rootURL); len(got) != 1 || got[0].Name != "currents-session" {
		t.Fatalf("root-host session changed: %v", got)
	}

	req.Header.Set("Origin", "https://other.example")
	res = httptest.NewRecorder()
	srv.OAuthExtensionSession(res, req)
	if res.Code != http.StatusForbidden || len(res.Result().Cookies()) != 0 {
		t.Fatalf("cross-origin bridge = %d, cookies %v", res.Code, res.Result().Cookies())
	}
}
