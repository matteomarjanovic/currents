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

func (s *Server) APIMe(w http.ResponseWriter, r *http.Request) {
	did, _, handle := s.currentSessionDID(r)
	w.Header().Set("Content-Type", "application/json")
	if did == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "not authenticated"})
		return
	}
	resp := map[string]string{
		"did":    did.String(),
		"handle": handle,
	}
	var displayName, avatar string
	err := s.Store.pool.QueryRow(r.Context(),
		`SELECT COALESCE(display_name, ''), COALESCE(avatar, '') FROM "user" WHERE did = $1`, did.String(),
	).Scan(&displayName, &avatar)
	if err == nil {
		if displayName != "" {
			resp["displayName"] = displayName
		}
		if avatar != "" {
			resp["avatar"] = avatar
		}
	}
	json.NewEncoder(w).Encode(resp)
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

	if returnTo := r.PostFormValue("return_to"); returnTo != "" {
		sess, _ := s.CookieStore.Get(r, "currents-session")
		sess.Values["return_to"] = returnTo
		sess.Save(r, w)
	}

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
	redirectTarget := "/"
	if s.FrontendURL != "" {
		redirectTarget = s.FrontendURL
	}
	if returnTo, ok := sess.Values["return_to"].(string); ok && returnTo != "" {
		if s.FrontendURL != "" && strings.HasPrefix(returnTo, s.FrontendURL) {
			redirectTarget = returnTo
		}
		delete(sess.Values, "return_to")
		sess.Save(r, w)
	}
	http.Redirect(w, r, redirectTarget, http.StatusFound)
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
	redirectTarget := "/"
	if s.FrontendURL != "" {
		redirectTarget = s.FrontendURL
	}
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

func ensureUserProfile(ctx context.Context, c *atclient.APIClient, store *PgStore, did, handle, pdsEndpoint, cdnBaseURL string) {
	// Fetch the user's Bluesky profile to copy fields from
	var getBskyRecordResp struct {
		Value json.RawMessage `json:"value"`
	}
	err := c.Get(ctx, "com.atproto.repo.getRecord", map[string]any{
		"repo":       did,
		"collection": "app.bsky.actor.profile",
		"rkey":       "self",
	}, &getBskyRecordResp)
	if err != nil {
		slog.Warn("fetching app.bsky.actor.profile", "did", did, "err", err)
	}
	var bskyProfile bskyActorProfile
	if getBskyRecordResp.Value != nil {
		if err := json.Unmarshal(getBskyRecordResp.Value, &bskyProfile); err != nil {
			slog.Warn("parsing app.bsky.actor.profile", "did", did, "err", err)
		}
	}

	// Check if a Currents profile record already exists.
	// Some PDSes return 404, others return 400 with error name "RecordNotFound".
	var getCurrentsRecordResp struct {
		Value json.RawMessage `json:"value"`
	}
	profileExists := true
	err = c.Get(ctx, "com.atproto.repo.getRecord", map[string]any{
		"repo":       did,
		"collection": currentsProfileNSID,
		"rkey":       "self",
	}, &getCurrentsRecordResp)
	if err != nil {
		var apiErr *atclient.APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == 404 || (apiErr.StatusCode == 400 && apiErr.Name == "RecordNotFound")) {
			profileExists = false
		} else {
			slog.Warn("checking is.currents.actor.profile", "did", did, "err", err)
		}
	}

	var currentsProfile currentsProfileRecord
	currentsProfileLoaded := false
	if profileExists && getCurrentsRecordResp.Value != nil {
		if err := json.Unmarshal(getCurrentsRecordResp.Value, &currentsProfile); err != nil {
			slog.Warn("parsing is.currents.actor.profile", "did", did, "err", err)
		} else {
			currentsProfileLoaded = true
		}
	}

	if !profileExists {
		currentsProfile = currentsProfileFromBskyProfile(bskyProfile, syntax.DatetimeNow().String())
		record := map[string]any{
			"$type":       currentsProfileNSID,
			"displayName": currentsProfile.DisplayName,
			"description": currentsProfile.Description,
			"createdAt":   currentsProfile.CreatedAt,
		}
		if currentsProfile.Avatar != nil {
			record["avatar"] = currentsProfile.Avatar
		}
		if currentsProfile.Banner != nil {
			record["banner"] = currentsProfile.Banner
		}
		if err := c.Post(ctx, "com.atproto.repo.putRecord", map[string]any{
			"repo":       did,
			"collection": currentsProfileNSID,
			"rkey":       "self",
			"record":     record,
		}, nil); err != nil {
			slog.Error("creating is.currents.actor.profile", "did", did, "err", err)
		}
		currentsProfileLoaded = true
	}

	userRecord := UserRecord{
		DID:         did,
		Handle:      handle,
		CreatedAt:   time.Now(),
		PDSEndpoint: pdsEndpoint,
	}
	if currentsProfileLoaded {
		userRecord = userRecordFromCurrentsProfile(did, handle, pdsEndpoint, cdnBaseURL, currentsProfile, time.Now())
	} else if existing, err := store.GetActorByDID(ctx, did); err == nil && existing != nil {
		createdAt := time.Now()
		if existing.CreatedAt != nil {
			createdAt = *existing.CreatedAt
		}
		userRecord = UserRecord{
			DID:         did,
			Handle:      handle,
			DisplayName: existing.DisplayName,
			Description: existing.Description,
			Pronouns:    existing.Pronouns,
			Website:     existing.Website,
			Avatar:      existing.Avatar,
			Banner:      existing.Banner,
			CreatedAt:   createdAt,
			PDSEndpoint: pdsEndpoint,
		}
	}

	if err := store.CreateUser(ctx, userRecord); err != nil {
		slog.Error("creating user in db", "did", did, "err", err)
	}
}
