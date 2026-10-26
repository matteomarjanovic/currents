package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (s *Server) APIListImportSessions(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	sessions, err := s.Store.listImportSessions(r.Context(), did.String())
	if err != nil {
		http.Error(w, "Could not load previous imports.", http.StatusInternalServerError)
		return
	}
	type sessionView struct {
		SessionID string `json:"sessionId"`
		Username  string `json:"username"`
		StartedAt string `json:"startedAt"`
		Active    bool   `json:"active"`
	}
	out := []sessionView{}
	for _, session := range sessions {
		out = append(out, sessionView{session.SessionID, session.Username, session.StartedAt.UTC().Format("2006-01-02T15:04:05Z"), session.Active})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"sessions": out})
}

func (s *Server) APIRetryImportSession(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var body struct {
		JobID string `json:"jobId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	unlock, ok := s.lockRepositoryRequest(w, r, did.String())
	if !ok {
		return
	}
	defer unlock()
	client, _, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "Reconnect to continue your import.", http.StatusUnauthorized)
		return
	}
	sessionID := r.PathValue("id")
	status, err := s.Store.GetSessionStatus(r.Context(), sessionID, did.String())
	if err != nil {
		http.Error(w, "Could not load this import.", http.StatusInternalServerError)
		return
	}
	if len(status) == 0 {
		http.NotFound(w, r)
		return
	}
	eligible := map[string]bool{}
	for _, job := range status {
		if job.CanRetry && (body.JobID == "" || body.JobID == job.JobID) {
			eligible[job.JobID] = true
		}
	}
	ids := []string{}
	skipped := 0
	for _, state := range []string{"done", "failed"} {
		jobs, err := s.Store.ListJobsByOwnerStatus(r.Context(), did.String(), state)
		if err != nil {
			http.Error(w, "Could not load this import.", http.StatusInternalServerError)
			return
		}
		for _, job := range jobs {
			if job.SessionID != sessionID || !eligible[job.ID] {
				continue
			}
			if _, err := resolveCollectionRef(r.Context(), client, s.Store, did.String(), job.TargetCollectionURI, false); err != nil {
				if isRecordNotFound(err) || errors.Is(err, errCollectionDeleting) {
					skipped++
					continue
				}
				http.Error(w, "Could not check the destination collection. Please try again.", http.StatusBadGateway)
				return
			}
			ids = append(ids, job.ID)
		}
	}
	n, err := s.Store.retryImportJobs(r.Context(), sessionID, did.String(), ids)
	if err != nil {
		http.Error(w, "Could not retry this import.", http.StatusInternalServerError)
		return
	}
	if n > 0 {
		s.ImportWorker.KickUser(did.String())
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"retried": n, "skipped": skipped})
}
