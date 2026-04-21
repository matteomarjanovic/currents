package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	comatproto "github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

type AttributionMigrationStats struct {
	CandidateCount        int64
	AuthorCount           int64
	RewrittenCount        int64
	DeletedCount          int64
	SkippedNoSessionCount int64
	SkippedNoChangeCount  int64
	FailedCount           int64
}

func isBlobNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "BlobNotFound") || strings.Contains(errStr, "Blob not found")
}

func rewriteSaveRecordValue(raw json.RawMessage) (map[string]any, bool, error) {
	var recordMap map[string]any
	if err := json.Unmarshal(raw, &recordMap); err != nil {
		return nil, false, fmt.Errorf("unmarshal save record map: %w", err)
	}
	_, hasLegacyAttribution := recordMap["attribution"]

	var record saveRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, false, fmt.Errorf("unmarshal save record struct: %w", err)
	}
	content, err := decodeSaveImageContent(record.Content)
	if err != nil {
		return nil, false, fmt.Errorf("decode save image content: %w", err)
	}
	needsBlobTypeRepair := content != nil && content.Image.Type == ""
	if !hasLegacyAttribution && !needsBlobTypeRepair {
		return nil, false, nil
	}

	contentAny, err := buildSaveContentWithAttribution(record.Content, nil, record.Attribution)
	if err != nil {
		return nil, false, fmt.Errorf("rewrite save content attribution: %w", err)
	}
	recordMap["content"] = contentAny
	delete(recordMap, "attribution")
	return recordMap, true, nil
}

func runAttributionMigration(ctx context.Context, store *PgStore, oauthClient *oauth.ClientApp) (AttributionMigrationStats, error) {
	candidates, err := store.ListSaveAttributionRewriteCandidates(ctx)
	if err != nil {
		return AttributionMigrationStats{}, err
	}
	report := AttributionMigrationStats{CandidateCount: int64(len(candidates))}
	if len(candidates) == 0 {
		return report, nil
	}

	byAuthor := make(map[string][]SaveAttributionRewriteCandidate)
	accountDIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := byAuthor[candidate.AuthorDID]; !ok {
			accountDIDs = append(accountDIDs, candidate.AuthorDID)
		}
		byAuthor[candidate.AuthorDID] = append(byAuthor[candidate.AuthorDID], candidate)
	}
	report.AuthorCount = int64(len(byAuthor))

	sessionIDs, err := store.GetLatestSessionIDs(ctx, accountDIDs)
	if err != nil {
		return report, err
	}

	for authorDID, authorCandidates := range byAuthor {
		sessionID, ok := sessionIDs[authorDID]
		if !ok || sessionID == "" {
			report.SkippedNoSessionCount += int64(len(authorCandidates))
			slog.Warn("skipping attribution migration for author without session", "did", authorDID, "count", len(authorCandidates))
			continue
		}

		did, err := syntax.ParseDID(authorDID)
		if err != nil {
			report.FailedCount += int64(len(authorCandidates))
			slog.Warn("skipping attribution migration for invalid DID", "did", authorDID, "err", err)
			continue
		}

		oauthSess, err := oauthClient.ResumeSession(ctx, did, sessionID)
		if err != nil {
			report.SkippedNoSessionCount += int64(len(authorCandidates))
			slog.Warn("skipping attribution migration for author with dead session", "did", authorDID, "err", err)
			continue
		}
		client := oauthSess.APIClient()

		for _, candidate := range authorCandidates {
			rkey := rkeyFromURI(candidate.URI)
			if rkey == "" {
				report.FailedCount++
				slog.Warn("skipping attribution migration for save without rkey", "uri", candidate.URI)
				continue
			}

			recordOut, err := comatproto.RepoGetRecord(ctx, client, "", saveNSID, authorDID, rkey)
			if err != nil {
				report.FailedCount++
				slog.Warn("fetching save for attribution migration failed", "uri", candidate.URI, "err", err)
				continue
			}
			if recordOut.Value == nil {
				report.FailedCount++
				slog.Warn("save record missing body during attribution migration", "uri", candidate.URI)
				continue
			}

			record, changed, err := rewriteSaveRecordValue(*recordOut.Value)
			if err != nil {
				report.FailedCount++
				slog.Warn("rewriting save attribution failed", "uri", candidate.URI, "err", err)
				continue
			}
			if !changed {
				report.SkippedNoChangeCount++
				continue
			}

			if _, err := comatproto.RepoPutRecord(ctx, client, &comatproto.RepoPutRecord_Input{
				Collection: saveNSID,
				Repo:       authorDID,
				Rkey:       rkey,
				Record:     record,
			}); err != nil {
				if isBlobNotFoundError(err) {
					if _, delErr := comatprototypes.RepoDeleteRecord(ctx, client, &comatprototypes.RepoDeleteRecord_Input{
						Collection: saveNSID,
						Repo:       authorDID,
						Rkey:       rkey,
					}); delErr != nil {
						report.FailedCount++
						slog.Warn("deleting broken save during attribution migration failed", "uri", candidate.URI, "err", delErr)
						continue
					}
					report.DeletedCount++
					slog.Warn("deleted broken save during attribution migration", "uri", candidate.URI, "did", authorDID)
					continue
				}
				report.FailedCount++
				slog.Warn("saving rewritten attribution failed", "uri", candidate.URI, "err", err)
				continue
			}

			report.RewrittenCount++
			slog.Info("rewrote save attribution", "uri", candidate.URI, "did", authorDID)
		}
	}

	return report, nil
}
