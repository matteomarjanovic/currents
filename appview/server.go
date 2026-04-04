package main

import (
	_ "embed"
	"html/template"

	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

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

func strPtr(raw string) *string {
	return &raw
}
