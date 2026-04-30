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
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	allowed := strings.TrimRight(s.FrontendURL, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if strings.TrimRight(origin, "/") == allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
