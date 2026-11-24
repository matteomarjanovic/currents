package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/jackc/pgx/v5"
)

const (
	blueskyLabelerService = blueskyLabelerDID + "#atproto_labeler"
	blueskyListSubjects   = "tools.ozone.inbox.listActionedSubjects"
	blueskyGetSubject     = "tools.ozone.inbox.getActionedSubject"
	blueskyListScope      = "rpc:" + blueskyListSubjects + "?aud=" + blueskyLabelerDID + "%23atproto_labeler"
	blueskyDetailScope    = "rpc:" + blueskyGetSubject + "?aud=" + blueskyLabelerDID + "%23atproto_labeler"
)

type blueskyAction struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
	Labels    []string  `json:"labels,omitempty"`
	Policies  []struct {
		DisplayName string `json:"displayName"`
	} `json:"policies,omitempty"`
}

type blueskySubject struct {
	Src     string    `json:"src"`
	IsRead  bool      `json:"isRead"`
	Updated time.Time `json:"updatedAt"`
	Subject struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"subject"`
	Enforcement struct {
		State  string   `json:"state"`
		Labels []string `json:"labels,omitempty"`
	} `json:"enforcement"`
	LatestAction     *blueskyAction `json:"latestAction,omitempty"`
	AvailableActions []string       `json:"availableActions,omitempty"`
}

type blueskyModerationNotice struct {
	URI           string         `json:"uri"`
	Notice        blueskySubject `json:"notice"`
	PreviewURL    string         `json:"previewUrl,omitempty"`
	PreviewLabels []labelView    `json:"previewLabels,omitempty"`
}

type blueskyInboxPage struct {
	Status string                    `json:"status"`
	Items  []blueskyModerationNotice `json:"items"`
	Cursor string                    `json:"cursor,omitempty"`
}

func isOwnedCurrentsSave(uri, did string) bool {
	u, err := syntax.ParseATURI(uri)
	return err == nil && u.Authority().String() == did && u.Collection().String() == saveNSID && u.RecordKey() != ""
}

// The list and subject endpoints use the signed-in viewer as their default DID.
// Keep unrelated Bluesky records and account-level actions out of Currents' inbox.
func getBlueskyInboxSubjects(ctx context.Context, client *atclient.APIClient, did, cursor, subject string) ([]blueskySubject, string, error) {
	client = client.WithService(blueskyLabelerService)
	var page struct {
		Subjects []blueskySubject
		Cursor   string
	}
	if subject != "" {
		var detail struct {
			blueskySubject
			Actions []blueskyAction
		}
		if err := client.Get(ctx, blueskyGetSubject, map[string]any{"subject": subject, "limit": 1}, &detail); err != nil {
			return nil, "", err
		}
		if len(detail.Actions) > 0 {
			detail.LatestAction = &detail.Actions[0]
		}
		if detail.Subject.URI == subject {
			page.Subjects = append(page.Subjects, detail.blueskySubject)
		}
	} else {
		params := map[string]any{"limit": 50}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if err := client.Get(ctx, blueskyListSubjects, params, &page); err != nil {
			return nil, "", err
		}
	}
	var subjects []blueskySubject
	indices := make(map[string]int)
	for _, row := range page.Subjects {
		uri := row.Subject.URI
		if row.Src != blueskyLabelerDID || !isOwnedCurrentsSave(uri, did) {
			continue
		}
		if i, ok := indices[uri]; ok {
			if row.Updated.After(subjects[i].Updated) {
				subjects[i] = row
			}
			continue
		}
		indices[uri] = len(subjects)
		subjects = append(subjects, row)
	}
	if page.Cursor == cursor {
		page.Cursor = ""
	}
	return subjects, page.Cursor, nil
}

func blueskyInboxError(err error) string {
	var xrpc *atclient.APIError
	if errors.As(err, &xrpc) {
		switch xrpc.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "permission-denied"
		case http.StatusNotImplemented:
			return "unsupported"
		case http.StatusNotFound:
			if xrpc.Name == "NotFound" {
				return "not-found"
			}
			return "unsupported"
		}
		if xrpc.Name == "MethodNotImplemented" || xrpc.Name == "XRPCNotSupported" {
			return "unsupported"
		}
	}
	return "unavailable"
}

func (s *Server) APIMeBlueskyModeration(w http.ResponseWriter, r *http.Request) {
	did, sid, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	subject := r.URL.Query().Get("subject")
	cursor := r.URL.Query().Get("cursor")
	scope := blueskyListScope
	if subject != "" {
		if !isOwnedCurrentsSave(subject, did.String()) {
			http.NotFound(w, r)
			return
		}
		scope = blueskyDetailScope
	}
	if len(cursor) > 2048 {
		http.Error(w, "invalid cursor", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	out := blueskyInboxPage{Status: "connect", Items: []blueskyModerationNotice{}}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	session, err := s.OAuth.ResumeSession(ctx, *did, sid)
	if err != nil || !hasBlueskyInboxScope(session.Data.Scopes, scope) {
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	rows, next, err := getBlueskyInboxSubjects(ctx, session.APIClient(), did.String(), cursor, subject)
	if err != nil {
		out.Status = blueskyInboxError(err)
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	out.Cursor = next
	out.Items, err = s.matchBlueskyModerationNotices(ctx, did.String(), rows)
	out.Status = "available"
	if err != nil {
		slog.Warn("match Bluesky moderation subjects", "err", err)
		out.Status = "unavailable"
	}
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) matchBlueskyModerationNotices(ctx context.Context, did string, rows []blueskySubject) ([]blueskyModerationNotice, error) {
	notices := make([]blueskyModerationNotice, 0, len(rows))
	if len(rows) == 0 {
		return notices, nil
	}
	uris := make([]string, len(rows))
	for i, row := range rows {
		uris[i] = row.Subject.URI
	}
	saves, err := s.Store.GetSavesByURIs(ctx, uris, "")
	if err != nil {
		return nil, err
	}
	versions, err := s.Store.saveRecordVersions(ctx, uris)
	if err != nil {
		return nil, err
	}
	matches := make(map[string]saveView)
	var views []saveView
	for _, save := range saves {
		if save.AuthorDID == did && save.ContentNSID == saveContentImageNSID && save.BlobCID != "" {
			views = append(views, buildSaveView(save, profileView{DID: did}, false, s.CDNBaseURL))
		}
	}
	// A label lookup failure leaves the decision readable without a preview.
	if err := s.hydrateLabels(ctx, views); err == nil {
		for _, view := range views {
			matches[view.URI] = view
		}
	}
	for _, row := range rows {
		uri := row.Subject.URI
		notice := blueskyModerationNotice{URI: uri, Notice: row}
		if save, ok := matches[uri]; ok && (versions[uri] == "" || versions[uri] == row.Subject.CID) {
			notice.PreviewURL = save.Content.(imageView).ImageURL
			notice.PreviewLabels = save.Labels
		}
		notices = append(notices, notice)
	}
	return notices, nil
}

func (s *PgStore) saveRecordVersions(ctx context.Context, uris []string) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT uri, record_cid FROM save WHERE uri = ANY($1) AND record_cid IS NOT NULL", uris)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var uri, cid string
		if err := rows.Scan(&uri, &cid); err != nil {
			return nil, err
		}
		out[uri] = cid
	}
	return out, rows.Err()
}

func (s *Server) OAuthBlueskyModeration(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.sessionDIDFromCookie(r)
	if did == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Keep normal login scopes and request only the additional read permissions.
	// Copy the config so concurrent regular logins do not request these scopes.
	app := *s.OAuth
	cfg := *app.Config
	cfg.Scopes = withBlueskyInboxReadScopes(cfg.Scopes)
	app.Config = &cfg
	target, err := app.StartAuthFlow(r.Context(), did.String())
	if err != nil {
		slog.Warn("start Bluesky moderation consent", "err", err)
		s.redirectBlueskyModeration(w, r, "unavailable")
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) redirectBlueskyModeration(w http.ResponseWriter, r *http.Request, status string) {
	http.Redirect(w, r, strings.TrimRight(s.FrontendURL, "/")+"/explore/general?bluesky-moderation="+status, http.StatusSeeOther)
}

func withBlueskyInboxReadScopes(scopes []string) []string {
	return append(slices.Clone(scopes), blueskyListScope, blueskyDetailScope)
}

func (s *Server) completeBlueskyModerationConsent(w http.ResponseWriter, r *http.Request, expected *syntax.DID) {
	did, sid, _ := s.sessionDIDFromCookie(r)
	if did == nil || expected == nil || *expected != *did {
		slog.Info("Bluesky moderation consent session changed")
		s.redirectBlueskyModeration(w, r, "session-changed")
		return
	}
	ctx := r.Context()
	requiredScopes := s.OAuth.Config.Scopes
	previous, err := s.OAuth.Store.GetSession(ctx, *did, sid)
	switch {
	case err == nil:
		requiredScopes = previous.Scopes
	case errors.Is(err, pgx.ErrNoRows):
		// The signed browser cookie can outlive its stored OAuth session.
		// The callback still binds the fresh grant to the same account.
		slog.Info("renewing expired OAuth session during Bluesky moderation consent")
	default:
		slog.Warn("load current session during Bluesky moderation consent", "err", err)
		s.redirectBlueskyModeration(w, r, "unavailable")
		return
	}
	data, err := s.OAuth.ProcessCallback(ctx, r.URL.Query())
	if err != nil {
		status := "unavailable"
		var callbackErr *oauth.AuthRequestCallbackError
		if errors.As(err, &callbackErr) && callbackErr.ErrorCode == "access_denied" {
			status = "denied"
		} else {
			stage, _, _ := strings.Cut(err.Error(), ":")
			slog.Warn("complete Bluesky moderation consent", "stage", stage)
		}
		s.redirectBlueskyModeration(w, r, status)
		return
	}
	for _, scope := range withBlueskyInboxReadScopes(requiredScopes) {
		if !hasBlueskyInboxScope(data.Scopes, scope) {
			slog.Info("Bluesky moderation consent missing required scope", "scope", scope)
			_ = s.OAuth.Logout(ctx, data.AccountDID, data.SessionID)
			s.redirectBlueskyModeration(w, r, "incomplete")
			return
		}
	}
	sess, _ := s.CookieStore.Get(r, "currents-session")
	sess.Values["session_id"] = data.SessionID
	if err := sess.Save(r, w); err != nil {
		s.redirectBlueskyModeration(w, r, "unavailable")
		return
	}
	s.redirectBlueskyModeration(w, r, "connected")
}

func hasBlueskyInboxScope(scopes []string, scope string) bool {
	if slices.Contains(scopes, scope) {
		return true
	}
	if !strings.HasPrefix(scope, "rpc:tools.ozone.inbox.") {
		return false
	}
	canonical := strings.Replace(scope, "%23", "#", 1)
	return slices.ContainsFunc(scopes, func(grant string) bool {
		return strings.Replace(grant, "%23", "#", 1) == canonical
	})
}
