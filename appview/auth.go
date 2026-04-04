package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func (s *Server) currentSessionDID(r *http.Request) (*syntax.DID, string, string) {
	sess, _ := s.CookieStore.Get(r, "currents-session")
	accountDID, ok := sess.Values["account_did"].(string)
	if !ok || accountDID == "" {
		return nil, "", ""
	}
	did, err := syntax.ParseDID(accountDID)
	if err != nil {
		return nil, "", ""
	}
	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, "", ""
	}
	handle, ok := sess.Values["handle"].(string)
	if !ok || handle == "" {
		return nil, "", ""
	}
	return &did, sessionID, handle
}

func (s *Server) ClientMetadata(w http.ResponseWriter, r *http.Request) {
	slog.Info("client metadata request", "url", r.URL, "host", r.Host)

	meta := s.OAuth.Config.ClientMetadata()
	if s.OAuth.Config.IsConfidential() {
		meta.JWKSURI = strPtr(fmt.Sprintf("https://%s/oauth/jwks.json", r.Host))
	}
	meta.ClientName = strPtr("Currents appview")
	meta.ClientURI = strPtr(fmt.Sprintf("https://%s", r.Host))

	if err := meta.Validate(s.OAuth.Config.ClientID); err != nil {
		slog.Error("validating client metadata", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(meta); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) JWKS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body := s.OAuth.Config.PublicJWKS()
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) Homepage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	did, sessionID, handle := s.currentSessionDID(r)
	if did == nil {
		tmplHome.Execute(w, nil)
		return
	}

	_, err := s.OAuth.ResumeSession(ctx, *did, sessionID)
	if err != nil {
		tmplHome.Execute(w, nil)
		return
	}
	tmplHome.Execute(w, TmplData{DID: did, Handle: handle})
}

func (s *Server) FeedPage(w http.ResponseWriter, r *http.Request) {
	did, _, handle := s.currentSessionDID(r)
	tmplFeed.Execute(w, TmplData{DID: did, Handle: handle})
}

func (s *Server) OAuthLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != "POST" {
		tmplLogin.Execute(w, nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Errorf("parsing form data: %w", err).Error(), http.StatusBadRequest)
		return
	}

	username, _ := strings.CutPrefix(r.PostFormValue("username"), "@")

	slog.Info("OAuthLogin", "client_id", s.OAuth.Config.ClientID, "callback_url", s.OAuth.Config.CallbackURL)

	redirectURL, err := s.OAuth.StartAuthFlow(ctx, username)
	if err != nil {
		oauthErr := fmt.Errorf("OAuth login failed: %w", err).Error()
		slog.Error(oauthErr)
		tmplLogin.Execute(w, TmplData{Error: oauthErr})
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := r.URL.Query()
	slog.Info("received callback", "params", params)

	sessData, err := s.OAuth.ProcessCallback(ctx, r.URL.Query())
	if err != nil {
		callbackErr := fmt.Errorf("failed processing oauth callback: %w", err).Error()
		slog.Error(callbackErr)
		tmplError.Execute(w, TmplData{Error: callbackErr})
		return
	}

	oauthSess, err := s.OAuth.ResumeSession(ctx, sessData.AccountDID, sessData.SessionID)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	c := oauthSess.APIClient()
	var resp struct {
		Handle string `json:"handle"`
	}
	if err := c.Get(ctx, "com.atproto.server.getSession", nil, &resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ensureUserProfile(ctx, c, s.Store, sessData.AccountDID.String(), resp.Handle, oauthSess.Data.HostURL, s.CDNBaseURL)

	sess, _ := s.CookieStore.Get(r, "currents-session")
	sess.Values["account_did"] = sessData.AccountDID.String()
	sess.Values["session_id"] = sessData.SessionID
	sess.Values["handle"] = resp.Handle
	if err := sess.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("login successful", "did", sessData.AccountDID.String())
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) OAuthLogout(w http.ResponseWriter, r *http.Request) {
	did, sessionID, _ := s.currentSessionDID(r)
	if did != nil {
		if err := s.OAuth.Logout(r.Context(), *did, sessionID); err != nil {
			slog.Error("failed to delete session", "did", did, "err", err)
		}
	}

	sess, _ := s.CookieStore.Get(r, "currents-session")
	sess.Values = make(map[any]any)
	if err := sess.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("logged out")
	http.Redirect(w, r, "/", http.StatusFound)
}

type blobRef struct {
	Type     string            `json:"$type"`
	Ref      map[string]string `json:"ref"`
	MimeType string            `json:"mimeType"`
	Size     int               `json:"size"`
}

type bskyActorProfile struct {
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Avatar      *blobRef `json:"avatar"`
	Banner      *blobRef `json:"banner"`
}

func ensureUserProfile(ctx context.Context, c *atclient.APIClient, store *PgStore, did, handle, pdsEndpoint, cdnBaseURL string) {
	// Fetch the user's Bluesky profile to copy fields from
	var getRecordResp struct {
		Value json.RawMessage `json:"value"`
	}
	err := c.Get(ctx, "com.atproto.repo.getRecord", map[string]any{
		"repo":       did,
		"collection": "app.bsky.actor.profile",
		"rkey":       "self",
	}, &getRecordResp)
	if err != nil {
		slog.Warn("fetching app.bsky.actor.profile", "did", did, "err", err)
	}
	var bskyProfile bskyActorProfile
	if getRecordResp.Value != nil {
		if err := json.Unmarshal(getRecordResp.Value, &bskyProfile); err != nil {
			slog.Warn("parsing app.bsky.actor.profile", "did", did, "err", err)
		}
	}

	// Check if a Currents profile record already exists.
	// Some PDSes return 404, others return 400 with error name "RecordNotFound".
	profileExists := true
	err = c.Get(ctx, "com.atproto.repo.getRecord", map[string]any{
		"repo":       did,
		"collection": "is.currents.actor.profile",
		"rkey":       "self",
	}, nil)
	if err != nil {
		var apiErr *atclient.APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == 404 || (apiErr.StatusCode == 400 && apiErr.Name == "RecordNotFound")) {
			profileExists = false
		} else {
			slog.Warn("checking is.currents.actor.profile", "did", did, "err", err)
		}
	}

	if !profileExists {
		record := map[string]any{
			"$type":       "is.currents.actor.profile",
			"displayName": bskyProfile.DisplayName,
			"description": bskyProfile.Description,
			"createdAt":   syntax.DatetimeNow(),
		}
		if bskyProfile.Avatar != nil {
			record["avatar"] = bskyProfile.Avatar
		}
		if bskyProfile.Banner != nil {
			record["banner"] = bskyProfile.Banner
		}
		if err := c.Post(ctx, "com.atproto.repo.putRecord", map[string]any{
			"repo":       did,
			"collection": "is.currents.actor.profile",
			"rkey":       "self",
			"record":     record,
		}, nil); err != nil {
			slog.Error("creating is.currents.actor.profile", "did", did, "err", err)
		}
	}

	// Build avatar/banner URLs routed through the CDN/image proxy
	blobURL := func(blob *blobRef) string {
		if blob == nil {
			return ""
		}
		cid := blob.Ref["$link"]
		if cid == "" {
			return ""
		}
		return cdnBaseURL + "/img/" + did + "/" + cid
	}

	if err := store.CreateUser(ctx, UserRecord{
		DID:         did,
		Handle:      handle,
		DisplayName: bskyProfile.DisplayName,
		Description: bskyProfile.Description,
		Avatar:      blobURL(bskyProfile.Avatar),
		Banner:      blobURL(bskyProfile.Banner),
		CreatedAt:   time.Now(),
		PDSEndpoint: pdsEndpoint,
	}); err != nil {
		slog.Error("creating user in db", "did", did, "err", err)
	}
}
