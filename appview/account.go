package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// Account deletion wipes the user's rows from the appview DB and stops TAP
// from tracking their repo. Their is.currents.* records stay on their PDS by
// default (logging in again re-adds the repo to TAP and backfill restores
// everything); on request they are deleted too, as a background job, because
// PDS record writes are rate-limited per account (5,000 points/hour and
// 35,000/day; DELETE = 1 point) so large repos can't be wiped in-request.

var tapAdminHTTP = &http.Client{Timeout: 30 * time.Second}

// tapRepos calls TAP's admin API to start ("add") or stop ("remove") tracking
// a repo. Adding triggers a backfill of the repo's existing records.
func (s *Server) tapRepos(ctx context.Context, action, did string) error {
	body, err := json.Marshal(map[string][]string{"dids": {did}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.TapAdminURL+"/repos/"+action, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.TapAdminPassword != "" {
		req.SetBasicAuth("admin", s.TapAdminPassword)
	}
	resp, err := tapAdminHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("tap /repos/%s: status %d: %.200s", action, resp.StatusCode, msg)
	}
	return nil
}

// APIAccountDelete deletes the viewer's account: refuses while a Polar
// subscription is active, stops TAP sync, wipes the appview DB, and — when
// requested — queues the background PDS wipe. Every step is idempotent, so a
// failed request is simply retried.
func (s *Server) APIAccountDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Resumes the session now — a dead session must fail here, not after the
	// TAP remove or inside the wipe job.
	_, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	_, sessionID, _ := s.currentSessionDID(r)

	var body struct {
		DeletePdsData bool `json:"deletePdsData"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	subscribed, err := s.Store.HasSupporterSubscription(ctx, did.String())
	if err != nil {
		slog.Error("account delete: HasSupporterSubscription", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if subscribed {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "ActiveSubscription", "message": "cancel your subscription before deleting your account"})
		return
	}

	if err := s.tapRepos(ctx, "remove", did.String()); err != nil {
		slog.Error("account delete: tap repos/remove", "did", did.String(), "err", err)
		http.Error(w, "could not stop repo sync — please try again", http.StatusBadGateway)
		return
	}

	keepSessionID := ""
	if body.DeletePdsData {
		if err := s.Store.CreatePdsWipe(ctx, did.String(), sessionID); err != nil {
			slog.Error("account delete: CreatePdsWipe", "did", did.String(), "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		keepSessionID = sessionID
	} else {
		// Best-effort upstream token revocation; also deletes the session row.
		if err := s.OAuth.Logout(ctx, *did, sessionID); err != nil {
			slog.Warn("account delete: oauth logout", "did", did.String(), "err", err)
		}
	}

	if err := s.Store.DeleteUserData(ctx, did.String(), keepSessionID); err != nil {
		slog.Error("account delete: db wipe", "did", did.String(), "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("account deleted", "did", did.String(), "pds_wipe", body.DeletePdsData)

	sess, _ := s.CookieStore.Get(r, "currents-session")
	sess.Values = make(map[any]any)
	sess.Save(r, w)

	if body.DeletePdsData {
		s.WipeWorker.Kick()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// wipeNSIDs are every record collection Currents writes to a user's repo.
var wipeNSIDs = []string{currentsProfileNSID, collectionNSID, saveNSID, followNSID, favouriteNSID}

// PdsWipeWorker processes pds_wipe jobs: it deletes every Currents record
// from a deleted user's PDS under the OAuth session kept for the job. Jobs
// are advanced as far as the PDS rate limit allows and re-enumerated from
// scratch on the next pass, so no progress state exists and restarts are
// free. Deletions are rare — one job at a time is plenty.
type PdsWipeWorker struct {
	Context context.Context
	Store   *PgStore
	OAuth   *oauth.ClientApp
	wake    chan struct{}
}

// Kick pokes the worker to look for new jobs. Safe from any goroutine.
func (w *PdsWipeWorker) Kick() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *PdsWipeWorker) Run() {
	for {
		jobs, err := w.Store.ListPdsWipes(w.Context)
		if err != nil {
			slog.Warn("pds wipe: listing jobs", "err", err)
		}
		for _, job := range jobs {
			w.process(job)
		}
		// The timer doubles as the rate-limit backoff: a 429'd job returns
		// from process and resumes on the next pass.
		select {
		case <-w.Context.Done():
			return
		case <-w.wake:
		case <-time.After(importRateLimitWait):
		}
	}
}

// process advances one job as far as the PDS allows. It returns early on any
// error or rate limit, leaving the job row in place for the next pass; an
// uninterrupted pass has deleted every record and drops the job.
func (w *PdsWipeWorker) process(job PdsWipeRow) {
	ctx := w.Context
	did := syntax.DID(job.DID)

	if w.userReturned(job) {
		return
	}

	sess, err := w.OAuth.ResumeSession(ctx, did, job.OAuthSessionID)
	if err != nil {
		// Session gone or refresh failed — the job can never proceed. Drop it;
		// leftover records can be removed by logging in and deleting again.
		slog.Error("pds wipe dropped: session unusable", "did", job.DID, "err", err)
		_ = w.Store.DeletePdsWipe(ctx, job.DID)
		return
	}
	c := sess.APIClient()

	for _, nsid := range wipeNSIDs {
		rkeys, err := listAllRecordKeys(ctx, c, job.DID, nsid)
		if err != nil {
			slog.Warn("pds wipe: listing records", "did", job.DID, "collection", nsid, "err", err)
			return
		}
		for chunk := range slices.Chunk(rkeys, 200) { // applyWrites caps at 200 ops
			if w.userReturned(job) {
				return
			}
			writes := make([]map[string]any, len(chunk))
			for i, rk := range chunk {
				writes[i] = map[string]any{"$type": "com.atproto.repo.applyWrites#delete", "collection": nsid, "rkey": rk}
			}
			err := c.Post(ctx, "com.atproto.repo.applyWrites", map[string]any{"repo": job.DID, "writes": writes}, nil)
			if isRateLimited(err) {
				// The batch applied nothing (applyWrites is atomic); the next
				// pass retries it once the rolling window frees points.
				slog.Info("pds wipe rate limited, backing off", "did", job.DID, "collection", nsid)
				return
			}
			if err != nil {
				slog.Warn("pds wipe: applyWrites", "did", job.DID, "collection", nsid, "err", err)
				return
			}
		}
	}

	// Revoke and drop the kept session (Logout deletes the row too).
	if err := w.OAuth.Logout(ctx, did, job.OAuthSessionID); err != nil {
		slog.Warn("pds wipe: final logout", "did", job.DID, "err", err)
		_ = w.Store.DeleteSession(ctx, did, job.OAuthSessionID)
	}
	_ = w.Store.DeletePdsWipe(ctx, job.DID)
	slog.Info("pds wipe complete", "did", job.DID)
}

// userReturned aborts the job when the user logged back in mid-wipe (their
// repo is being backfilled again — deleting records now would destroy the
// account they came back for). Their fresh login owns its own session; the
// job's kept one is revoked.
func (w *PdsWipeWorker) userReturned(job PdsWipeRow) bool {
	actor, err := w.Store.GetActorByDID(w.Context, job.DID)
	if err != nil || actor == nil {
		return false
	}
	slog.Info("pds wipe aborted: user returned", "did", job.DID)
	if err := w.OAuth.Logout(w.Context, syntax.DID(job.DID), job.OAuthSessionID); err != nil {
		_ = w.Store.DeleteSession(w.Context, syntax.DID(job.DID), job.OAuthSessionID)
	}
	_ = w.Store.DeletePdsWipe(w.Context, job.DID)
	return true
}

// listAllRecordKeys enumerates every rkey in one of the repo's collections —
// fully, before any deletion, because deleting while paginating shifts
// cursors.
func listAllRecordKeys(ctx context.Context, c *atclient.APIClient, did, nsid string) ([]string, error) {
	var rkeys []string
	cursor := ""
	for {
		params := map[string]any{"repo": did, "collection": nsid, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var out struct {
			Cursor  *string `json:"cursor"`
			Records []struct {
				URI string `json:"uri"`
			} `json:"records"`
		}
		if err := c.Get(ctx, "com.atproto.repo.listRecords", params, &out); err != nil {
			return nil, err
		}
		for _, rec := range out.Records {
			if rk := rkeyFromURI(rec.URI); rk != "" {
				rkeys = append(rkeys, rk)
			}
		}
		if out.Cursor == nil || *out.Cursor == "" || len(out.Records) == 0 {
			return rkeys, nil
		}
		cursor = *out.Cursor
	}
}
