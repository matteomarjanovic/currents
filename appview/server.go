package main

import (
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"

	"github.com/gorilla/sessions"
)

type Server struct {
	CookieStore   *sessions.CookieStore
	Dir           identity.Directory
	OAuth         *oauth.ClientApp
	Store         *PgStore
	CDNBaseURL    string
	ServiceDID    string // did:web:{hostname}, used for /.well-known/did.json and XRPC service auth
	AuthValidator *auth.ServiceAuthValidator
	Inference     *InferenceClient
	FrontendURL   string
	ProcessMode   string
	ImportWorker  *ImportWorker
	Labeler       *LabelerIssuer
	LabelerHost   string // host portion of the labeler DID (e.g. "moderation.currents.is"); empty when labeler disabled
	// MobileOrigins are additional CORS-allowed origins (beyond capacitor://localhost and https://localhost) for native clients.
	MobileOrigins []string
	// MobileRedirectSchemes are allowed return_to URL prefixes for native deep links (e.g. currents://).
	MobileRedirectSchemes []string
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	frontend := strings.TrimRight(s.FrontendURL, "/")
	// Capacitor WebView origins are fixed: iOS uses capacitor://localhost, Android uses
	// https://localhost (per the androidScheme: 'https' Capacitor config).
	mobile := append([]string{"capacitor://localhost", "https://localhost"}, s.MobileOrigins...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		normalized := strings.TrimRight(origin, "/")
		allowed := frontend != "" && normalized == frontend
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
