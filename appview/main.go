package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"

	"github.com/gorilla/sessions"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name:   "appview",
		Usage:  "AT Protocol appview server",
		Action: runServer,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "session-secret",
				Usage:    "random string/token used for session cookie security",
				Required: true,
				EnvVars:  []string{"SESSION_SECRET"},
			},
			&cli.StringFlag{
				Name:     "database-url",
				Usage:    "PostgreSQL connection string",
				Required: true,
				EnvVars:  []string{"DATABASE_URL"},
			},
			&cli.StringFlag{
				Name:    "hostname",
				Usage:   "public host name for this server (if not localhost dev mode)",
				EnvVars: []string{"CLIENT_HOSTNAME"},
			},
			&cli.StringFlag{
				Name:    "client-secret-key",
				Usage:   "confidential client secret key; P-256 private key in multibase encoding",
				EnvVars: []string{"CLIENT_SECRET_KEY"},
			},
			&cli.StringFlag{
				Name:    "client-secret-key-id",
				Usage:   "key id for client-secret-key",
				Value:   "primary",
				EnvVars: []string{"CLIENT_SECRET_KEY_ID"},
			},
			&cli.StringFlag{
				Name:    "tap-url",
				Usage:   "WebSocket URL of the TAP event stream",
				Value:   "ws://localhost:2480/channel",
				EnvVars: []string{"TAP_URL"},
			},
			&cli.StringFlag{
				Name:    "inference-url",
				Usage:   "Base URL of the inference FastAPI server",
				Value:   "http://localhost:8000",
				EnvVars: []string{"INFERENCE_URL"},
			},
			&cli.StringFlag{
				Name:    "cdn-url",
				Usage:   "base URL for image CDN; defaults to appview base URL in localhost mode",
				EnvVars: []string{"CDN_URL"},
			},
		},
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
	app.RunAndExitOnError()
}

func runServer(cctx *cli.Context) error {
	scopes := []string{"atproto", "repo:is.currents.actor.profile", "repo:is.currents.feed.collection", "repo:is.currents.feed.save", "blob:image/*"}
	bind := ":8080"

	var config oauth.ClientConfig
	hostname := cctx.String("hostname")
	if hostname == "" {
		config = oauth.NewLocalhostConfig(
			fmt.Sprintf("http://127.0.0.1%s/oauth/callback", bind),
			scopes,
		)
		slog.Info("configuring localhost OAuth client", "CallbackURL", config.CallbackURL)
	} else {
		config = oauth.NewPublicConfig(
			fmt.Sprintf("https://%s/oauth-client-metadata.json", hostname),
			fmt.Sprintf("https://%s/oauth/callback", hostname),
			scopes,
		)
	}

	if cctx.String("client-secret-key") != "" && hostname != "" {
		priv, err := atcrypto.ParsePrivateMultibase(cctx.String("client-secret-key"))
		if err != nil {
			return err
		}
		if err := config.SetClientSecret(priv, cctx.String("client-secret-key-id")); err != nil {
			return err
		}
		slog.Info("configuring confidential OAuth client")
	}

	store, err := NewPgStore(context.Background(), &PgStoreConfig{
		DSN:                       cctx.String("database-url"),
		SessionExpiryDuration:     time.Hour * 24 * 90,
		SessionInactivityDuration: time.Hour * 24 * 14,
		AuthRequestExpiryDuration: time.Minute * 30,
	})
	if err != nil {
		return err
	}
	oauthClient := oauth.NewClientApp(&config, store)

	cdnURL := cctx.String("cdn-url")
	var serviceDID string
	if hostname == "" {
		// localhost dev: did:web:localhost%3A8080 (colon in port must be percent-encoded)
		port := strings.TrimPrefix(bind, ":")
		serviceDID = "did:web:localhost%3A" + port
		if cdnURL == "" {
			cdnURL = "http://127.0.0.1" + bind
		}
	} else {
		serviceDID = "did:web:" + hostname
		if cdnURL == "" {
			cdnURL = "https://" + hostname
		}
	}

	srv := Server{
		CookieStore: sessions.NewCookieStore([]byte(cctx.String("session-secret"))),
		Dir:         identity.DefaultDirectory(),
		OAuth:       oauthClient,
		Store:       store,
		CDNBaseURL:  cdnURL,
		ServiceDID:  serviceDID,
		AuthValidator: &auth.ServiceAuthValidator{
			Audience: serviceDID,
			Dir:      identity.DefaultDirectory(),
		},
		Inference: NewInferenceClient(cctx.String("inference-url")),
	}

	http.HandleFunc("GET /.well-known/did.json", srv.WellKnownDID)
	http.HandleFunc("GET /oauth-client-metadata.json", srv.ClientMetadata)
	http.HandleFunc("GET /oauth/jwks.json", srv.JWKS)
	http.HandleFunc("GET /oauth/callback", srv.OAuthCallback)

	http.HandleFunc("GET /api/me", srv.APIMe)

	http.HandleFunc("GET /oauth/login", srv.OAuthLogin)
	http.HandleFunc("POST /oauth/login", srv.OAuthLogin)
	http.HandleFunc("GET /oauth/logout", srv.OAuthLogout)

	http.HandleFunc("GET /", srv.Homepage)
	http.HandleFunc("GET /feed", srv.FeedPage)

	http.HandleFunc("GET /collections", srv.ListCollections)
	http.HandleFunc("POST /collections", srv.CreateCollection)
	http.HandleFunc("GET /collections/{id}", srv.GetCollection)
	http.HandleFunc("POST /collections/{id}", srv.UpdateCollection)
	http.HandleFunc("POST /collections/{id}/delete", srv.DeleteCollection)

	http.HandleFunc("GET /img/{did}/{cid}", srv.ImageProxy)

	http.HandleFunc("GET /xrpc/is.currents.feed.getCollections",
		srv.AuthValidator.Middleware(srv.XRPCGetCollections, true))
	http.HandleFunc("GET /xrpc/is.currents.feed.getActorCollections", srv.XRPCGetActorCollections)
	http.HandleFunc("GET /xrpc/is.currents.feed.getSaves", srv.XRPCGetSaves)
	http.HandleFunc("GET /xrpc/is.currents.feed.searchSaves", srv.XRPCSearchSaves)
	http.HandleFunc("GET /xrpc/is.currents.feed.getFeed", srv.XRPCGetFeed)

	http.HandleFunc("GET /saves", srv.ListSaves)
	http.HandleFunc("POST /saves", srv.CreateSave)
	http.HandleFunc("GET /saves/{id}", srv.GetSave)
	http.HandleFunc("POST /saves/{id}", srv.UpdateSave)
	http.HandleFunc("POST /saves/{id}/delete", srv.DeleteSave)

	tapHandler := &TapHandler{
		Store:      store,
		Dir:        srv.Dir,
		Inference:  NewInferenceClient(cctx.String("inference-url")),
		CDNBaseURL: cdnURL,
	}
	go runTapListener(context.Background(), cctx.String("tap-url"), tapHandler)
	slog.Info("TAP listener started", "url", cctx.String("tap-url"))

	slog.Info("starting http server", "bind", bind)
	if err := http.ListenAndServe(bind, nil); err != nil {
		slog.Error("http shutdown", "err", err)
	}
	return nil
}
