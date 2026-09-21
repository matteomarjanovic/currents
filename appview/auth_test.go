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
