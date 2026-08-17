package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/gorilla/sessions"
)

func TestRegisterLegacyAndAPI(t *testing.T) {
	mux := http.NewServeMux()
	registerLegacyAndAPI(mux, "DELETE /save/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.PathValue("id")))
	})

	for _, path := range []string{"/save/abc", "/api/save/abc"} {
		req := httptest.NewRequest(http.MethodDelete, path, nil)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusOK || res.Body.String() != "abc" {
			t.Errorf("DELETE %s: got status %d body %q", path, res.Code, res.Body.String())
		}
	}
}

func TestSplitCSVEmptyIsNonNil(t *testing.T) {
	got := splitCSV("")
	if got == nil || len(got) != 0 {
		t.Fatalf("splitCSV empty = %#v, want non-nil empty slice", got)
	}
}

func TestClientMetadataUsesOAuthIdentity(t *testing.T) {
	config := oauth.NewPublicConfig(
		"https://currents.is/oauth-client-metadata.json",
		"https://currents.is/oauth/callback",
		[]string{"atproto"},
	)
	srv := Server{
		OAuth:       &oauth.ClientApp{Config: &config},
		FrontendURL: "https://currents.is",
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://api.currents.is/oauth-client-metadata.json", nil)
	srv.ClientMetadata(res, req)

	var meta map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"client_id":   "https://currents.is/oauth-client-metadata.json",
		"client_name": "Currents",
		"client_uri":  "https://currents.is",
	} {
		if got := meta[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLegacyLogoutExpiresHostOnlyCookieAndContinuesOnFrontend(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret"))
	store.Options = &sessions.Options{
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	srv := Server{CookieStore: store, FrontendURL: "https://currents.is"}
	res := httptest.NewRecorder()
	srv.OAuthLegacyLogout(res, httptest.NewRequest(http.MethodGet, "https://api.currents.is/oauth/logout/legacy", nil))

	if location := res.Header().Get("Location"); location != "https://currents.is/oauth/logout" {
		t.Fatalf("location = %q", location)
	}
	setCookie := res.Header().Get("Set-Cookie")
	for _, want := range []string{"currents-session=", "Path=/", "Max-Age=0", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(setCookie, want) {
			t.Errorf("Set-Cookie %q does not contain %q", setCookie, want)
		}
	}
	if strings.Contains(setCookie, "Domain=") {
		t.Errorf("Set-Cookie must remain host-only: %q", setCookie)
	}
}

func TestWellKnownDIDUsesServiceURLNotCDN(t *testing.T) {
	srv := Server{
		ServiceDID: "did:web:api.currents.is",
		ServiceURL: "https://api.currents.is",
		CDNBaseURL: "https://cdn.currents.is",
	}
	res := httptest.NewRecorder()
	srv.WellKnownDID(res, httptest.NewRequest(http.MethodGet, "https://api.currents.is/.well-known/did.json", nil))

	var doc struct {
		Service []struct {
			Endpoint string `json:"serviceEndpoint"`
		} `json:"service"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Service) != 1 || doc.Service[0].Endpoint != srv.ServiceURL {
		t.Fatalf("service endpoint = %#v, want %q", doc.Service, srv.ServiceURL)
	}
}
