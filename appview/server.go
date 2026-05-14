package main

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/gorilla/sessions"
)

type Server struct {
	CookieStore           *sessions.CookieStore
	Dir                   identity.Directory
	OAuth                 *oauth.ClientApp
	Store                 *PgStore
	CDNBaseURL            string
	ServiceDID            string // did:web:{hostname}, used for /.well-known/did.json and XRPC service auth
	AuthValidator         *auth.ServiceAuthValidator
	Inference             *InferenceClient
	FrontendURL           string
	ProcessMode           string
	MobileOrigins         []string // additional CORS-allowed origins (e.g. capacitor://localhost)
	MobileRedirectSchemes []string // allowed return_to URL prefixes for native deep links (e.g. currents://)
}

type TmplData struct {
	DID    *syntax.DID
	Handle string
	Error  string
}

//go:embed "base.html"
var tmplBaseText string

//go:embed "home.html"
var tmplHomeText string
var tmplHome = template.Must(template.Must(template.New("home.html").Parse(tmplBaseText)).Parse(tmplHomeText))

//go:embed "login.html"
var tmplLoginText string
var tmplLogin = template.Must(template.Must(template.New("login.html").Parse(tmplBaseText)).Parse(tmplLoginText))

//go:embed "error.html"
var tmplErrorText string
var tmplError = template.Must(template.Must(template.New("error.html").Parse(tmplBaseText)).Parse(tmplErrorText))

//go:embed "collections.html"
var tmplCollectionsText string
var tmplCollections = template.Must(template.Must(template.New("collections.html").Parse(tmplBaseText)).Parse(tmplCollectionsText))

//go:embed "saves.html"
var tmplSavesText string
var tmplSaves = template.Must(template.Must(template.New("saves.html").Parse(tmplBaseText)).Parse(tmplSavesText))

//go:embed "feed.html"
var tmplFeedText string
var tmplFeed = template.Must(template.Must(template.New("feed.html").Parse(tmplBaseText)).Parse(tmplFeedText))

//go:embed "ops.html"
var tmplOpsText string
var tmplOps = template.Must(template.Must(template.New("ops.html").Parse(tmplBaseText)).Parse(tmplOpsText))

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	frontend := strings.TrimRight(s.FrontendURL, "/")
	// Capacitor WebView origins are fixed: iOS uses capacitor://localhost, Android uses
	// https://localhost (per the androidScheme: 'https' Capacitor config).
	defaultMobile := []string{"capacitor://localhost", "https://localhost"}
	mobile := append(defaultMobile, s.MobileOrigins...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		normalized := strings.TrimRight(origin, "/")
		allowed := false
		if frontend != "" && normalized == frontend {
			allowed = true
		}
		if !allowed {
			for _, o := range mobile {
				if normalized == strings.TrimRight(o, "/") {
					allowed = true
					break
				}
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func strPtr(raw string) *string {
	return &raw
}
