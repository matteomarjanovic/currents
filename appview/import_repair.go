package main

import (
	"context"
	"errors"
	"fmt"
)

type pinterestRepairCandidate struct {
	ID        string `json:"jobId"`
	OwnerDID  string `json:"ownerDid"`
	SessionID string `json:"sessionId"`
	Expected  int    `json:"expected"`
	Listed    int    `json:"listed"`
}

// Select the latest import of each board, including any change from a flat
// board to sections. Older duplicate destinations are deliberately excluded.
// A non-null listed_count also excludes jobs already checked by this release.
func (m *PgStore) pinterestRepairCandidates(ctx context.Context, did string) ([]pinterestRepairCandidate, error) {
	rows, err := m.pool.Query(ctx, `WITH latest AS (
		SELECT DISTINCT ON(owner_did, source_board_id) owner_did, source_board_id, session_id
		FROM import_job WHERE source='pinterest' AND ($1='' OR owner_did=$1)
		ORDER BY owner_did, source_board_id, created_at DESC, id DESC
	)
	SELECT j.id, j.owner_did, j.session_id, j.expected_count, count(i.id)::int
	FROM latest l JOIN import_job j USING(owner_did, source_board_id, session_id)
	LEFT JOIN import_item i ON i.job_id=j.id
	WHERE j.status='done' AND j.listed_count IS NULL AND j.listing_error='' AND j.expected_count>25
	GROUP BY j.id HAVING count(i.id)<=25 ORDER BY j.owner_did, j.session_id, j.created_at`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pinterestRepairCandidate{}
	for rows.Next() {
		var c pinterestRepairCandidate
		if err := rows.Scan(&c.ID, &c.OwnerDID, &c.SessionID, &c.Expected, &c.Listed); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type pinterestRepairReport struct {
	DryRun              bool                       `json:"dryRun"`
	Candidates          []pinterestRepairCandidate `json:"candidates"`
	Eligible            int                        `json:"eligible"`
	Queued              int                        `json:"queued"`
	SkippedDestinations int                        `json:"skippedDestinations"`
	Busy                bool                       `json:"busy"`
	ReconnectRequired   bool                       `json:"reconnectRequired"`
}

// Queue only; the running server owns listing, upload pacing and write budgets.
// Applying requires one DID so operators can pace a platform-wide recovery.
func (w *ImportWorker) repairPinterestImports(ctx context.Context, did string, dryRun bool) (pinterestRepairReport, error) {
	report := pinterestRepairReport{DryRun: dryRun}
	if did == "" && !dryRun {
		return report, fmt.Errorf("applying Pinterest repair requires --did; use --dry-run for the global plan")
	}
	candidates, err := w.Store.pinterestRepairCandidates(ctx, did)
	report.Candidates = candidates
	if err != nil || did == "" || len(candidates) == 0 {
		return report, err
	}
	unlock, err := w.Store.lockRepository(ctx, did)
	if err != nil {
		return report, err
	}
	defer unlock()
	busy, err := w.Store.repositoryBusy(ctx, did)
	if err != nil {
		return report, err
	}
	if busy {
		report.Busy = true
		return report, nil
	}
	session := w.resumeLatestSession(ctx, did)
	if session == nil {
		report.ReconnectRequired = true
		return report, nil
	}
	jobs, err := w.Store.ListJobsByOwnerStatus(ctx, did, "done")
	if err != nil {
		return report, err
	}
	targets := map[string]bool{}
	for _, c := range candidates {
		targets[c.ID] = true
	}
	bySession := map[string][]string{}
	for _, job := range jobs {
		if !targets[job.ID] {
			continue
		}
		if _, err := resolveCollectionRef(ctx, session.APIClient(), w.Store, did, job.TargetCollectionURI, false); err != nil {
			if isRecordNotFound(err) || errors.Is(err, errCollectionDeleting) {
				report.SkippedDestinations++
				continue
			}
			return report, fmt.Errorf("checking import destination %s: %w", job.ID, err)
		}
		report.Eligible++
		bySession[job.SessionID] = append(bySession[job.SessionID], job.ID)
	}
	if !dryRun {
		for id, jobs := range bySession {
			n, err := w.Store.retryImportJobs(ctx, id, did, jobs)
			if err != nil {
				return report, err
			}
			report.Queued += n
		}
	}
	return report, nil
}
