package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
)

// Each imported item is one record CREATE on the user's PDS, which costs 3 of
// the PDS write points (capped at 5,000/hour and 35,000/day per account). The
// pace and budgets below keep imports at ~72% of the hourly cap and ~77% of
// the daily cap, so the user can keep posting on Bluesky while a large import
// runs. Budgets are enforced by counting recently completed items in the DB,
// so they hold across restarts and parallel jobs of the same user.
const importItemPaceDelay = 3 * time.Second
const importHourlyCreateBudget = 1200 // 3,600 of 5,000 hourly write points
const importDailyCreateBudget = 9000  // 27,000 of 35,000 daily write points
const importBudgetRecheck = time.Minute
const importRateLimitWait = 5 * time.Minute
const maxImportAttempts = 3

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) APIPinterestBoards(w http.ResponseWriter, r *http.Request) {
	if did, _, _ := s.currentSessionDID(r); did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
	boards, err := ListBoards(r.Context(), username)
	if err != nil {
		slog.Warn("pinterest boards list failed", "username", username, "err", err)
		http.Error(w, fmt.Sprintf("listing boards: %s", err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"boards": boards})
}

func (s *Server) APIPinterestSections(w http.ResponseWriter, r *http.Request) {
	if did, _, _ := s.currentSessionDID(r); did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	boardID := strings.TrimSpace(r.URL.Query().Get("boardId"))
	boardURL := strings.TrimSpace(r.URL.Query().Get("boardUrl"))
	if boardID == "" || boardURL == "" {
		http.Error(w, "boardId and boardUrl are required", http.StatusBadRequest)
		return
	}
	sections, err := ListSections(r.Context(), boardID, boardURL)
	if err != nil {
		slog.Warn("pinterest sections list failed", "board", boardID, "err", err)
		http.Error(w, fmt.Sprintf("listing sections: %s", err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"sections": sections})
}

func (s *Server) APICreatePinterestJob(w http.ResponseWriter, r *http.Request) {
	did, sessionID, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var body struct {
		ImportSessionID    string `json:"importSessionId"`
		PinterestBoardID   string `json:"pinterestBoardId"`
		PinterestBoardName string `json:"pinterestBoardName"`
		PinterestBoardURL  string `json:"pinterestBoardUrl"`
		PinterestSectionID string `json:"pinterestSectionId"`
		FilterSectionPins  bool   `json:"filterSectionPins"`
		PinterestUsername  string `json:"pinterestUsername"`
		CollectionURI      string `json:"collectionUri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.ImportSessionID == "" || body.PinterestBoardID == "" || body.PinterestBoardURL == "" || body.CollectionURI == "" {
		http.Error(w, "importSessionId, pinterestBoardId, pinterestBoardUrl and collectionUri are required", http.StatusBadRequest)
		return
	}
	parsed, err := syntax.ParseATURI(body.CollectionURI)
	if err != nil || parsed.Authority().String() != did.String() || parsed.Collection().String() != collectionNSID {
		http.Error(w, "collectionUri must be your own is.currents.feed.collection record", http.StatusBadRequest)
		return
	}

	if err := s.Store.UpsertImportSession(r.Context(), body.ImportSessionID, did.String(), body.PinterestUsername); err != nil {
		http.Error(w, fmt.Sprintf("creating session: %s", err), http.StatusInternalServerError)
		return
	}
	jobID, err := s.Store.CreateImportJob(r.Context(), ImportJobRow{
		SessionID:           body.ImportSessionID,
		OwnerDID:            did.String(),
		OAuthSessionID:      sessionID,
		Source:              "pinterest",
		SourceBoardID:       body.PinterestBoardID,
		SourceBoardName:     body.PinterestBoardName,
		SourceBoardURL:      body.PinterestBoardURL,
		SourceSectionID:     body.PinterestSectionID,
		FilterSectionPins:   body.FilterSectionPins,
		TargetCollectionURI: body.CollectionURI,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("creating job: %s", err), http.StatusInternalServerError)
		return
	}
	s.ImportWorker.KickUser(did.String())

	slog.Info("created import job", "job_id", jobID, "did", did.String(), "board", body.PinterestBoardID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"jobId": jobID})
}

func (s *Server) APIGetImportSession(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	sessionID := r.PathValue("id")
	jobs, err := s.Store.GetSessionStatus(r.Context(), sessionID, did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading session: %s", err), http.StatusInternalServerError)
		return
	}
	type jobView struct {
		JobID     string `json:"jobId"`
		BoardName string `json:"boardName"`
		Status    string `json:"status"`
		Queued    int    `json:"queued"`
		Running   int    `json:"running"`
		Done      int    `json:"done"`
		Failed    int    `json:"failed"`
	}
	out := struct {
		SessionID string    `json:"sessionId"`
		Jobs      []jobView `json:"jobs"`
	}{SessionID: sessionID, Jobs: make([]jobView, 0, len(jobs))}
	for _, j := range jobs {
		out.Jobs = append(out.Jobs, jobView{
			JobID: j.JobID, BoardName: j.BoardName, Status: j.Status,
			Queued: j.Queued, Running: j.Running, Done: j.Done, Failed: j.Failed,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) APIGetActiveImportSession(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	active, err := s.Store.GetLatestActiveSession(r.Context(), did.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("loading active session: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if active == nil {
		json.NewEncoder(w).Encode(map[string]any{})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"sessionId": active.SessionID,
		"username":  active.Username,
		"startedAt": active.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

// ── Worker ────────────────────────────────────────────────────────────────────

type ImportWorker struct {
	Context   context.Context
	Store     *PgStore
	OAuth     *oauth.ClientApp
	Dir       identity.Directory
	Inference *InferenceClient

	mu     sync.Mutex
	drains map[string]chan struct{}
}

// Run resumes any inflight imports left over from the previous process and
// then blocks until the parent context is cancelled. Drains for newly-created
// jobs are started by KickUser from the HTTP handler.
func (w *ImportWorker) Run() {
	users, err := w.Store.ListInflightUsers(w.Context)
	if err != nil {
		slog.Error("listing inflight import users", "err", err)
	}
	for _, did := range users {
		if err := w.Store.ResetRunningItemsForUser(w.Context, did); err != nil {
			slog.Warn("resetting running items", "did", did, "err", err)
		}
		w.startDrain(did)
	}

	go func() {
		t := time.NewTicker(2 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-w.Context.Done():
				return
			case <-t.C:
				inflight, err := w.Store.ListInflightUsers(w.Context)
				if err != nil {
					slog.Warn("import watchdog", "err", err)
					continue
				}
				for _, did := range inflight {
					w.KickUser(did)
				}
			}
		}
	}()

	<-w.Context.Done()
}

// KickUser ensures a drain goroutine is running for did and pokes it to look
// for newly-queued work. Safe to call from any goroutine.
func (w *ImportWorker) KickUser(did string) {
	w.mu.Lock()
	ch, ok := w.drains[did]
	w.mu.Unlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
		return
	}
	w.startDrain(did)
}

func (w *ImportWorker) startDrain(did string) {
	w.mu.Lock()
	if _, exists := w.drains[did]; exists {
		w.mu.Unlock()
		return
	}
	wake := make(chan struct{}, 1)
	w.drains[did] = wake
	w.mu.Unlock()
	go w.runForUser(did, wake)
}

func (w *ImportWorker) removeDrain(did string) {
	w.mu.Lock()
	delete(w.drains, did)
	w.mu.Unlock()
}

func (w *ImportWorker) runForUser(did string, wake <-chan struct{}) {
	defer w.removeDrain(did)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("import worker panic", "did", did, "panic", r)
		}
	}()
	ctx := w.Context

	for {
		if ctx.Err() != nil {
			return
		}

		listing, err := w.Store.ListJobsByOwnerStatus(ctx, did, "listing")
		if err != nil {
			slog.Warn("listing import jobs", "did", did, "err", err)
		}
		for _, job := range listing {
			w.runListingStage(ctx, job)
			if ctx.Err() != nil {
				return
			}
			// Finalize immediately if listing produced zero items.
			if err := w.Store.MaybeFinalizeJob(ctx, job.ID); err != nil {
				slog.Warn("finalizing import job after listing", "job_id", job.ID, "err", err)
			}
		}

		hourly, daily, err := w.Store.CountRecentImportDone(ctx, did)
		if err != nil {
			slog.Warn("counting recent import creates", "did", did, "err", err)
		} else if hourly >= importHourlyCreateBudget || daily >= importDailyCreateBudget {
			slog.Debug("import write budget exhausted, waiting", "did", did, "hourly", hourly, "daily", daily)
			select {
			case <-ctx.Done():
				return
			case <-time.After(importBudgetRecheck):
			}
			continue
		}

		item, err := w.Store.ClaimNextImportItem(ctx, did)
		if err != nil {
			slog.Warn("claiming import item", "did", did, "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		if item == nil {
			select {
			case <-ctx.Done():
				return
			case <-wake:
				continue
			case <-time.After(30 * time.Second):
				continue
			}
		}

		oauthSess := w.resumeLatestSession(ctx, did)
		if oauthSess == nil {
			// No usable session right now (logged out, rotated, or refresh
			// failed). Return the item to the queue without burning a retry and
			// pause this drain; the watchdog restarts it and the import resumes
			// once a valid session is available again.
			_ = w.Store.PauseImportItem(ctx, item.ID)
			slog.Warn("import paused: no valid session", "did", did)
			return
		}

		if w.processItem(ctx, item, did, oauthSess) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(importRateLimitWait):
			}
			continue
		}
		if err := w.Store.MaybeFinalizeJob(ctx, item.JobID); err != nil {
			slog.Warn("finalizing import job", "job_id", item.JobID, "err", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(importItemPaceDelay):
		}
	}
}

// resumeLatestSession resolves the user's most recent stored OAuth session,
// refreshing tokens as needed. Returns nil when the user has no usable session,
// so the caller can pause the import rather than fail it.
func (w *ImportWorker) resumeLatestSession(ctx context.Context, did string) *oauth.ClientSession {
	sid, err := w.Store.LatestOAuthSessionID(ctx, did)
	if err != nil {
		slog.Warn("looking up import session", "did", did, "err", err)
		return nil
	}
	if sid == "" {
		return nil
	}
	sess, err := w.OAuth.ResumeSession(ctx, syntax.DID(did), sid)
	if err != nil {
		// Session vanished between lookup and resume, or a token refresh failed.
		// Treat as pausable rather than a hard failure.
		slog.Warn("import session resume failed", "did", did, "err", err)
		return nil
	}
	return sess
}

func (w *ImportWorker) runListingStage(ctx context.Context, job ImportJobRow) {
	batch := make([]PinterestPin, 0, 100)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if _, err := w.Store.BulkInsertImportItems(ctx, job.ID, job.OwnerDID, batch); err != nil {
			slog.Warn("inserting import items", "job_id", job.ID, "err", err)
		}
		batch = batch[:0]
	}

	onPin := func(p PinterestPin, nextBookmark string) error {
		batch = append(batch, p)
		if len(batch) >= 100 {
			flush()
			if cerr := w.Store.UpdateImportJobCursor(ctx, job.ID, nextBookmark); cerr != nil {
				slog.Warn("updating list cursor", "job_id", job.ID, "err", cerr)
			}
		}
		return nil
	}

	var last string
	var err error
	if job.SourceSectionID != "" {
		last, err = IterateSectionPins(ctx, job.SourceSectionID, job.SourceBoardURL, job.ListCursor, onPin)
	} else {
		last, err = IteratePins(ctx, job.SourceBoardID, job.SourceBoardURL, job.ListCursor, job.FilterSectionPins, onPin)
	}
	flush()

	if err != nil {
		_ = w.Store.UpdateImportJobStatus(ctx, job.ID, "failed", err.Error())
		slog.Warn("listing pinterest board failed", "job_id", job.ID, "board", job.SourceBoardURL, "err", err)
		return
	}
	_ = w.Store.UpdateImportJobCursor(ctx, job.ID, last)
	if err := w.Store.UpdateImportJobStatus(ctx, job.ID, "running", ""); err != nil {
		slog.Warn("marking job running", "job_id", job.ID, "err", err)
	}
	slog.Info("listing complete", "job_id", job.ID, "board", job.SourceBoardURL)
}

func (w *ImportWorker) failOrRequeue(ctx context.Context, item *ImportItemRow, reason string) {
	if item.AttemptCount < maxImportAttempts {
		_ = w.Store.RequeueImportItem(ctx, item.ID, reason)
		slog.Info("import item requeued for retry", "item_id", item.ID, "job_id", item.JobID, "attempt", item.AttemptCount, "reason", reason)
	} else {
		_ = w.Store.MarkImportItemFailed(ctx, item.ID, reason)
		slog.Warn("import item permanently failed", "item_id", item.ID, "job_id", item.JobID, "attempts", item.AttemptCount, "reason", reason)
	}
}

// isRateLimited reports whether err is an HTTP 429 from the PDS.
func isRateLimited(err error) bool {
	var apiErr *atclient.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests
}

// processItem imports one item. It returns true when a 429 was hit (the item
// is returned to the queue without consuming an attempt) so the caller can
// back off before processing more work for this user.
func (w *ImportWorker) processItem(ctx context.Context, item *ImportItemRow, did string, oauthSess *oauth.ClientSession) (rateLimited bool) {
	c := oauthSess.APIClient()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.ImageURL, nil)
	if err != nil {
		w.failOrRequeue(ctx, item, "fetch req: "+err.Error())
		return false
	}
	req.Header.Set("User-Agent", pinterestUA)
	resp, err := pinterestHTTP.Do(req)
	if err != nil {
		w.failOrRequeue(ctx, item, "fetch: "+err.Error())
		return false
	}
	imageBytes, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		slog.Warn("import rate limited by image host, backing off", "did", did, "wait", importRateLimitWait)
		_ = w.Store.PauseImportItem(ctx, item.ID)
		return true
	}
	if resp.StatusCode != http.StatusOK {
		w.failOrRequeue(ctx, item, fmt.Sprintf("fetch status %d", resp.StatusCode))
		return false
	}
	if readErr != nil {
		w.failOrRequeue(ctx, item, "fetch body: "+readErr.Error())
		return false
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(imageBytes)
	}

	imageBytes, contentType, err = prepareImageForUpload(ctx, w.Inference, imageBytes, contentType)
	if err != nil {
		w.failOrRequeue(ctx, item, "prepare: "+err.Error())
		return false
	}

	var uploadOut struct {
		Blob lexutil.LexBlob `json:"blob"`
	}
	if err := c.LexDo(ctx, "POST", contentType, "com.atproto.repo.uploadBlob", nil,
		bytes.NewReader(imageBytes), &uploadOut); err != nil {
		if isRateLimited(err) {
			slog.Warn("import rate limited by PDS on uploadBlob, backing off", "did", did, "wait", importRateLimitWait)
			_ = w.Store.PauseImportItem(ctx, item.ID)
			return true
		}
		w.failOrRequeue(ctx, item, "uploadBlob: "+err.Error())
		return false
	}
	blobJSON, _ := json.Marshal(uploadOut.Blob)
	var blobAny any
	json.Unmarshal(blobJSON, &blobAny)

	collRef, err := resolveStrongRef(ctx, c, item.TargetCollectionURI)
	if err != nil {
		if isRateLimited(err) {
			slog.Warn("import rate limited by PDS on getRecord, backing off", "did", did, "wait", importRateLimitWait)
			_ = w.Store.PauseImportItem(ctx, item.ID)
			return true
		}
		// A missing target collection dooms every item in the job (they all
		// point at the same record), so fail the whole job at once instead of
		// retrying each item. Happens when the collection was deleted after the
		// items were queued — e.g. cleaning up a previous import's leftovers.
		if isRecordNotFound(err) {
			_ = w.Store.FailImportJob(ctx, item.JobID, "target collection missing: "+err.Error())
			slog.Warn("import job failed: target collection missing", "job_id", item.JobID, "collection", item.TargetCollectionURI)
			return false
		}
		w.failOrRequeue(ctx, item, "collection: "+err.Error())
		return false
	}

	originURL := item.SourceURL
	if originURL == "" {
		originURL = fmt.Sprintf("https://www.pinterest.com/pin/%s/", item.SourcePinID)
	}
	record := map[string]any{
		"$type":      saveNSID,
		"collection": collRef,
		"content":    buildImageContentRecordWithAttribution(blobAny, nil, ""),
		"createdAt":  syntax.DatetimeNow().String(),
		"originUrl":  originURL,
	}

	out, err := comatproto.RepoPutRecord(ctx, c, &comatproto.RepoPutRecord_Input{
		Collection: saveNSID,
		Repo:       did,
		Rkey:       item.Rkey,
		Record:     record,
	})
	if err != nil {
		if isRateLimited(err) {
			slog.Warn("import rate limited by PDS on putRecord, backing off", "did", did, "wait", importRateLimitWait)
			_ = w.Store.PauseImportItem(ctx, item.ID)
			return true
		}
		w.failOrRequeue(ctx, item, "putRecord: "+err.Error())
		return false
	}
	_ = w.Store.MarkImportItemDone(ctx, item.ID, out.Uri)
	return false
}
