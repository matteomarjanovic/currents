package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
				Name:    "mode",
				Usage:   "process mode: all or repair",
				Value:   "all",
				EnvVars: []string{"APPVIEW_MODE"},
			},
			&cli.StringFlag{
				Name:    "session-secret",
				Usage:   "random string/token used for session cookie security",
				EnvVars: []string{"SESSION_SECRET"},
			},
			&cli.StringFlag{
				Name:     "database-url",
				Usage:    "PostgreSQL connection string",
				Required: true,
				EnvVars:  []string{"DATABASE_URL"},
			},
			&cli.IntFlag{
				Name:    "db-min-conns",
				Usage:   "minimum number of PostgreSQL connections to keep open",
				Value:   4,
				EnvVars: []string{"DB_MIN_CONNS"},
			},
			&cli.IntFlag{
				Name:    "db-max-conns",
				Usage:   "maximum number of PostgreSQL connections in the pool",
				Value:   12,
				EnvVars: []string{"DB_MAX_CONNS"},
			},
			&cli.DurationFlag{
				Name:    "db-max-conn-lifetime",
				Usage:   "maximum lifetime of a PostgreSQL connection",
				Value:   30 * time.Minute,
				EnvVars: []string{"DB_MAX_CONN_LIFETIME"},
			},
			&cli.DurationFlag{
				Name:    "db-max-conn-idle-time",
				Usage:   "maximum idle time of a PostgreSQL connection",
				Value:   5 * time.Minute,
				EnvVars: []string{"DB_MAX_CONN_IDLE_TIME"},
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
			&cli.StringFlag{
				Name:    "frontend-url",
				Usage:   "URL of the SvelteKit frontend; OAuth callback redirects here after login",
				EnvVars: []string{"FRONTEND_URL"},
			},
			&cli.StringFlag{
				Name:    "hidden-dids",
				Usage:   "comma-separated author DIDs to hide from feed/search results",
				EnvVars: []string{"HIDDEN_DIDS"},
			},
		},
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
	app.RunAndExitOnError()
}

// splitCSV splits a comma-separated string into a slice, trimming whitespace
// and dropping empty entries. Returns nil for an empty input.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runServer(cctx *cli.Context) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mode := strings.ToLower(cctx.String("mode"))
	switch mode {
	case "all", "repair":
	default:
		return fmt.Errorf("invalid mode %q", mode)
	}
	if mode == "all" && cctx.String("session-secret") == "" {
		return fmt.Errorf("missing session-secret")
	}

	scopes := []string{"atproto", "repo:is.currents.actor.profile", "repo:is.currents.feed.collection", "repo:is.currents.feed.save", "blob:image/*"}
	bind := ":8080"
	dir := identity.DefaultDirectory()

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

	store, err := NewPgStore(ctx, &PgStoreConfig{
		DSN:                       cctx.String("database-url"),
		SessionExpiryDuration:     time.Hour * 24 * 90,
		SessionInactivityDuration: time.Hour * 24 * 14,
		AuthRequestExpiryDuration: time.Minute * 30,
		MinConns:                  int32(cctx.Int("db-min-conns")),
		MaxConns:                  int32(cctx.Int("db-max-conns")),
		MaxConnLifetime:           cctx.Duration("db-max-conn-lifetime"),
		MaxConnIdleTime:           cctx.Duration("db-max-conn-idle-time"),
		HiddenDIDs:                splitCSV(cctx.String("hidden-dids")),
	})
	if err != nil {
		return err
	}
	oauthClient := oauth.NewClientApp(&config, store)

	inferenceClient := NewInferenceClient(cctx.String("inference-url"))
	tapHandler := &TapHandler{
		Context:   ctx,
		Store:     store,
		Dir:       dir,
		Inference: inferenceClient,
	}

	if mode == "repair" {
		report, err := runRepairPass(ctx, tapHandler)
		if err != nil {
			return err
		}
		slog.Info("repair pass completed",
			"blob_candidates", report.BlobCandidates,
			"blob_enriched", report.BlobEnriched,
			"collection_candidates", report.CollectionCandidates,
			"collections_recomputed", report.CollectionsRecomputed,
		)
		return nil
	}

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

	cookieStore := sessions.NewCookieStore([]byte(cctx.String("session-secret")))
	cookieStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 90,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}

	srv := Server{
		CookieStore: cookieStore,
		Dir:         dir,
		OAuth:       oauthClient,
		Store:       store,
		CDNBaseURL:  cdnURL,
		ServiceDID:  serviceDID,
		AuthValidator: &auth.ServiceAuthValidator{
			Audience: serviceDID,
			Dir:      dir,
		},
		Inference:   inferenceClient,
		FrontendURL: cctx.String("frontend-url"),
		ProcessMode: mode,
	}

	http.HandleFunc("GET /.well-known/did.json", srv.WellKnownDID)
	http.HandleFunc("GET /oauth-client-metadata.json", srv.ClientMetadata)
	http.HandleFunc("GET /oauth/jwks.json", srv.JWKS)
	http.HandleFunc("GET /oauth/callback", srv.OAuthCallback)

	http.HandleFunc("GET /api/me", srv.APIMe)
	http.HandleFunc("GET /api/profile/import-bluesky", srv.APIImportBlueskyProfile)
	http.HandleFunc("PUT /api/profile", srv.UpdateProfile)
	http.HandleFunc("GET /debug/background", srv.BackgroundStatus)

	http.HandleFunc("POST /oauth/login", srv.OAuthLogin)
	http.HandleFunc("GET /oauth/logout", srv.OAuthLogout)

	http.HandleFunc("POST /collection", srv.CreateCollection)
	http.HandleFunc("GET /collection/{id}", srv.GetCollection)
	http.HandleFunc("PUT /collection/{id}", srv.UpdateCollection)
	http.HandleFunc("DELETE /collection/{id}", srv.DeleteCollection)

	http.HandleFunc("GET /img/{did}/{cid}", srv.ImageProxy)

	http.HandleFunc("GET /xrpc/is.currents.feed.getActorCollections", srv.XRPCGetActorCollections)
	http.HandleFunc("GET /xrpc/is.currents.actor.getProfile", srv.XRPCGetActorProfile)
	http.HandleFunc("GET /xrpc/is.currents.feed.getCollectionSaves", srv.XRPCGetCollectionSaves)
	http.HandleFunc("GET /xrpc/is.currents.feed.getSaves", srv.XRPCGetSaves)
	http.HandleFunc("GET /xrpc/is.currents.feed.searchSaves", srv.XRPCSearchSaves)
	http.HandleFunc("GET /xrpc/is.currents.feed.getRelatedSaves", srv.XRPCGetRelatedSaves)
	http.HandleFunc("GET /xrpc/is.currents.feed.getFeed", srv.XRPCGetFeed)

	http.HandleFunc("POST /save", srv.CreateSave)
	http.HandleFunc("PUT /save/attribution", srv.UpdateSaveAttribution)
	http.HandleFunc("GET /save/{id}", srv.GetSave)
	http.HandleFunc("PUT /save/{id}", srv.UpdateSave)
	http.HandleFunc("DELETE /save/{id}", srv.DeleteSave)
	http.HandleFunc("POST /resave", srv.CreateResave)

	tapHandler.CDNBaseURL = cdnURL
	go runTapListener(ctx, cctx.String("tap-url"), tapHandler)
	slog.Info("TAP listener started", "url", cctx.String("tap-url"))

	var handler http.Handler = http.DefaultServeMux
	handler = noCacheMiddleware(handler)
	if srv.FrontendURL != "" {
		handler = srv.corsMiddleware(handler)
	}

	httpServer := &http.Server{
		Addr:              bind,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("starting http server",
		"mode", mode,
		"bind", bind,
		"read_header_timeout", httpServer.ReadHeaderTimeout,
		"read_timeout", httpServer.ReadTimeout,
		"write_timeout", httpServer.WriteTimeout,
		"idle_timeout", httpServer.IdleTimeout,
	)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("http shutdown", "err", err)
		return err
	}
	return nil
}
