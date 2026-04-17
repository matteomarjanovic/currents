package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/jackc/pgx/v5"
	"golang.org/x/time/rate"
)

type SaveContentMigrationConfig struct {
	Store          *PgStore
	OAuth          *oauth.ClientApp
	Dir            identity.Directory
	ProgressFile   string
	DryRun         bool
	AuthorDID      string
	SaveURI        string
	Limit          int
	GlobalRate     float64
	PerAccountRate float64
}

type saveContentMigrationTarget struct {
	URI         string
	AuthorDID   string
	ResaveOfURI string
}

type saveContentMigrationRecord struct {
	URI       string
	AuthorDID string
	Repo      string
	Rkey      string
	CID       string
	Value     map[string]any
}

type saveContentMigrationProgress struct {
	Version   int             `json:"version"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
	PassA     map[string]bool `json:"passA,omitempty"`
	PassB     map[string]bool `json:"passB,omitempty"`
}

type saveContentMigrationSummary struct {
	Targets      int
	Resaves      int
	PassAUpdated int
	PassANoop    int
	PassASkipped int
	PassAFailed  int
	PassBUpdated int
	PassBNoop    int
	PassBSkipped int
	PassBFailed  int
}

type saveContentMigrationClient struct {
	API     *atclient.APIClient
	Limiter *rate.Limiter
}

func runSaveContentMigration(ctx context.Context, cfg SaveContentMigrationConfig) error {
	if cfg.Store == nil {
		return fmt.Errorf("missing store")
	}
	if cfg.OAuth == nil {
		return fmt.Errorf("missing oauth client")
	}
	if cfg.Dir == nil {
		return fmt.Errorf("missing identity directory")
	}
	if cfg.Limit < 0 {
		return fmt.Errorf("migration limit must be >= 0")
	}
	if cfg.ProgressFile == "" {
		cfg.ProgressFile = "save-content-migration-progress.json"
	}
	if cfg.GlobalRate <= 0 {
		return fmt.Errorf("migration global rate must be > 0")
	}
	if cfg.PerAccountRate <= 0 {
		return fmt.Errorf("migration per-account rate must be > 0")
	}
	if cfg.AuthorDID != "" {
		if _, err := syntax.ParseDID(cfg.AuthorDID); err != nil {
			return fmt.Errorf("invalid migration-author DID: %w", err)
		}
	}
	if cfg.SaveURI != "" {
		if _, err := syntax.ParseATURI(cfg.SaveURI); err != nil {
			return fmt.Errorf("invalid migration-uri: %w", err)
		}
	}

	targets, err := listSaveContentMigrationTargets(ctx, cfg.Store, cfg.AuthorDID, cfg.SaveURI, cfg.Limit)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		slog.Info("save-content migration: no matching saves")
		return nil
	}

	progress := newSaveContentMigrationProgress()
	if !cfg.DryRun {
		progress, err = loadSaveContentMigrationProgress(cfg.ProgressFile)
		if err != nil {
			return err
		}
	}

	selectedTargets := make(map[string]struct{}, len(targets))
	resaveCount := 0
	for _, target := range targets {
		selectedTargets[target.URI] = struct{}{}
		if target.ResaveOfURI != "" {
			resaveCount++
		}
	}

	summary := saveContentMigrationSummary{Targets: len(targets), Resaves: resaveCount}
	slog.Info(
		"starting save-content migration",
		"targets", summary.Targets,
		"resaves", summary.Resaves,
		"dry_run", cfg.DryRun,
		"author", cfg.AuthorDID,
		"uri", cfg.SaveURI,
		"limit", cfg.Limit,
		"progress_file", cfg.ProgressFile,
	)

	globalLimiter := rate.NewLimiter(rate.Limit(cfg.GlobalRate), 1)
	clients := map[string]*saveContentMigrationClient{}
	passAFinished := map[string]bool{}
	failureCount := 0

	for _, target := range targets {
		if !cfg.DryRun && progress.PassA[target.URI] {
			summary.PassASkipped++
			passAFinished[target.URI] = true
			continue
		}

		client, err := getSaveContentMigrationClient(ctx, cfg, clients, target.AuthorDID)
		if err != nil {
			summary.PassAFailed++
			failureCount++
			slog.Error("save-content migration pass A: session", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		record, err := fetchOwnedSaveMigrationRecord(ctx, client.API, target.URI)
		if err != nil {
			summary.PassAFailed++
			failureCount++
			slog.Error("save-content migration pass A: fetch", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		changed, err := rewriteSaveRecordForContent(record.Value)
		if err != nil {
			summary.PassAFailed++
			failureCount++
			slog.Error("save-content migration pass A: rewrite", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		passAFinished[target.URI] = true
		if !changed {
			summary.PassANoop++
			if !cfg.DryRun {
				progress.PassA[target.URI] = true
				if err := saveSaveContentMigrationProgress(cfg.ProgressFile, progress); err != nil {
					return err
				}
			}
			continue
		}

		if cfg.DryRun {
			summary.PassAUpdated++
			slog.Info("save-content migration pass A dry run", "uri", target.URI, "author", target.AuthorDID, "cid", record.CID)
			continue
		}

		oldCID := record.CID
		newCID, err := putOwnedSaveMigrationRecord(ctx, globalLimiter, client, record)
		if err != nil {
			summary.PassAFailed++
			failureCount++
			passAFinished[target.URI] = false
			slog.Error("save-content migration pass A: put", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		summary.PassAUpdated++
		progress.PassA[target.URI] = true
		if err := saveSaveContentMigrationProgress(cfg.ProgressFile, progress); err != nil {
			return err
		}
		slog.Info("save-content migration pass A updated", "uri", target.URI, "author", target.AuthorDID, "old_cid", oldCID, "new_cid", newCID)
	}

	for _, target := range targets {
		if target.ResaveOfURI == "" {
			continue
		}
		if !cfg.DryRun && progress.PassB[target.URI] {
			summary.PassBSkipped++
			continue
		}
		if !cfg.DryRun && !progress.PassA[target.URI] {
			summary.PassBSkipped++
			slog.Warn("save-content migration pass B skipped until pass A succeeds", "uri", target.URI, "author", target.AuthorDID)
			continue
		}
		if _, ok := selectedTargets[target.ResaveOfURI]; ok && !passAFinished[target.ResaveOfURI] {
			summary.PassBSkipped++
			slog.Warn("save-content migration pass B waiting on referenced save", "uri", target.URI, "resave_of", target.ResaveOfURI)
			continue
		}

		client, err := getSaveContentMigrationClient(ctx, cfg, clients, target.AuthorDID)
		if err != nil {
			summary.PassBFailed++
			failureCount++
			slog.Error("save-content migration pass B: session", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		record, err := fetchOwnedSaveMigrationRecord(ctx, client.API, target.URI)
		if err != nil {
			summary.PassBFailed++
			failureCount++
			slog.Error("save-content migration pass B: fetch", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		resaveRef, err := resolveStrongRefPublic(ctx, cfg.Store, cfg.Dir, target.ResaveOfURI)
		if err != nil {
			summary.PassBFailed++
			failureCount++
			slog.Error("save-content migration pass B: resolve", "uri", target.URI, "resave_of", target.ResaveOfURI, "err", err)
			continue
		}

		changed, err := rewriteSaveRecordResaveOf(record.Value, resaveRef)
		if err != nil {
			summary.PassBFailed++
			failureCount++
			slog.Error("save-content migration pass B: rewrite", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		resaveCID, _ := resaveRef["cid"].(string)
		if !changed {
			summary.PassBNoop++
			if !cfg.DryRun {
				if err := setSaveResaveOfCID(ctx, cfg.Store, target.URI, resaveCID); err != nil {
					summary.PassBFailed++
					failureCount++
					slog.Error("save-content migration pass B: update db", "uri", target.URI, "author", target.AuthorDID, "err", err)
					continue
				}
				progress.PassB[target.URI] = true
				if err := saveSaveContentMigrationProgress(cfg.ProgressFile, progress); err != nil {
					return err
				}
			}
			continue
		}

		if cfg.DryRun {
			summary.PassBUpdated++
			slog.Info("save-content migration pass B dry run", "uri", target.URI, "author", target.AuthorDID, "resave_of", target.ResaveOfURI)
			continue
		}

		oldCID := record.CID
		newCID, err := putOwnedSaveMigrationRecord(ctx, globalLimiter, client, record)
		if err != nil {
			summary.PassBFailed++
			failureCount++
			slog.Error("save-content migration pass B: put", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}
		if err := setSaveResaveOfCID(ctx, cfg.Store, target.URI, resaveCID); err != nil {
			summary.PassBFailed++
			failureCount++
			slog.Error("save-content migration pass B: update db", "uri", target.URI, "author", target.AuthorDID, "err", err)
			continue
		}

		summary.PassBUpdated++
		progress.PassB[target.URI] = true
		if err := saveSaveContentMigrationProgress(cfg.ProgressFile, progress); err != nil {
			return err
		}
		slog.Info("save-content migration pass B updated", "uri", target.URI, "author", target.AuthorDID, "old_cid", oldCID, "new_cid", newCID, "resave_of", target.ResaveOfURI, "resave_of_cid", resaveCID)
	}

	slog.Info(
		"save-content migration complete",
		"targets", summary.Targets,
		"resaves", summary.Resaves,
		"pass_a_updated", summary.PassAUpdated,
		"pass_a_noop", summary.PassANoop,
		"pass_a_skipped", summary.PassASkipped,
		"pass_a_failed", summary.PassAFailed,
		"pass_b_updated", summary.PassBUpdated,
		"pass_b_noop", summary.PassBNoop,
		"pass_b_skipped", summary.PassBSkipped,
		"pass_b_failed", summary.PassBFailed,
		"dry_run", cfg.DryRun,
	)

	if failureCount > 0 {
		return fmt.Errorf("save-content migration completed with %d failures", failureCount)
	}
	return nil
}

func listSaveContentMigrationTargets(ctx context.Context, store *PgStore, authorDID, saveURI string, limit int) ([]saveContentMigrationTarget, error) {
	query := `SELECT uri, author_did, COALESCE(resave_of_uri, '') FROM save`
	var args []any
	var where []string
	if authorDID != "" {
		args = append(args, authorDID)
		where = append(where, fmt.Sprintf("author_did = $%d", len(args)))
	}
	if saveURI != "" {
		args = append(args, saveURI)
		where = append(where, fmt.Sprintf("uri = $%d", len(args)))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY author_did ASC, created_at ASC NULLS LAST, uri ASC"
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := store.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []saveContentMigrationTarget
	for rows.Next() {
		var target saveContentMigrationTarget
		if err := rows.Scan(&target.URI, &target.AuthorDID, &target.ResaveOfURI); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func newSaveContentMigrationProgress() *saveContentMigrationProgress {
	return &saveContentMigrationProgress{
		Version: 1,
		PassA:   map[string]bool{},
		PassB:   map[string]bool{},
	}
}

func loadSaveContentMigrationProgress(path string) (*saveContentMigrationProgress, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newSaveContentMigrationProgress(), nil
		}
		return nil, fmt.Errorf("reading migration progress: %w", err)
	}

	progress := newSaveContentMigrationProgress()
	if err := json.Unmarshal(raw, progress); err != nil {
		return nil, fmt.Errorf("parsing migration progress: %w", err)
	}
	if progress.Version == 0 {
		progress.Version = 1
	}
	if progress.PassA == nil {
		progress.PassA = map[string]bool{}
	}
	if progress.PassB == nil {
		progress.PassB = map[string]bool{}
	}
	return progress, nil
}

func saveSaveContentMigrationProgress(path string, progress *saveContentMigrationProgress) error {
	progress.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing migration progress: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("writing migration progress: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming migration progress: %w", err)
	}
	return nil
}

func getSaveContentMigrationClient(ctx context.Context, cfg SaveContentMigrationConfig, cache map[string]*saveContentMigrationClient, authorDID string) (*saveContentMigrationClient, error) {
	if client, ok := cache[authorDID]; ok {
		return client, nil
	}

	sessionIDs, err := listOAuthSessionIDsForDID(ctx, cfg.Store, authorDID)
	if err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return nil, fmt.Errorf("no oauth session found for %s", authorDID)
	}

	did, err := syntax.ParseDID(authorDID)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, sessionID := range sessionIDs {
		oauthSess, err := cfg.OAuth.ResumeSession(ctx, did, sessionID)
		if err != nil {
			lastErr = err
			continue
		}
		client := &saveContentMigrationClient{
			API:     oauthSess.APIClient(),
			Limiter: rate.NewLimiter(rate.Limit(cfg.PerAccountRate), 1),
		}
		cache[authorDID] = client
		return client, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("resuming oauth session for %s: %w", authorDID, lastErr)
	}
	return nil, fmt.Errorf("no resumable oauth session found for %s", authorDID)
}

func listOAuthSessionIDsForDID(ctx context.Context, store *PgStore, did string) ([]string, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT session_id
		FROM oauth_sessions
		WHERE account_did = $1
		ORDER BY updated_at DESC, created_at DESC, session_id ASC
	`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, err
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	return sessionIDs, rows.Err()
}

func fetchOwnedSaveMigrationRecord(ctx context.Context, client *atclient.APIClient, atURI string) (*saveContentMigrationRecord, error) {
	parsed, err := syntax.ParseATURI(atURI)
	if err != nil {
		return nil, fmt.Errorf("invalid AT-URI: %w", err)
	}

	out, err := comatproto.RepoGetRecord(ctx, client, "", parsed.Collection().String(), parsed.Authority().String(), parsed.RecordKey().String())
	if err != nil {
		return nil, err
	}
	if out.Value == nil {
		return nil, fmt.Errorf("record has no value")
	}

	var record map[string]any
	if err := json.Unmarshal(*out.Value, &record); err != nil {
		return nil, fmt.Errorf("decoding record: %w", err)
	}

	currentCID := ""
	if out.Cid != nil {
		currentCID = *out.Cid
	}

	return &saveContentMigrationRecord{
		URI:       atURI,
		AuthorDID: parsed.Authority().String(),
		Repo:      parsed.Authority().String(),
		Rkey:      parsed.RecordKey().String(),
		CID:       currentCID,
		Value:     record,
	}, nil
}

func rewriteSaveRecordForContent(record map[string]any) (bool, error) {
	contentRaw, hasContent := record["content"]
	legacyImageRaw, hasLegacyImage := record["image"]

	if !hasContent {
		if !hasLegacyImage {
			return false, fmt.Errorf("save record missing image/content")
		}
		record["content"] = buildImageContentRecord(legacyImageRaw)
		delete(record, "image")
		return true, nil
	}

	content, ok := contentRaw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("save content is not an object")
	}

	changed := false
	if _, ok := content["$type"]; !ok {
		content["$type"] = saveContentImageNSID
		changed = true
	}
	if hasLegacyImage {
		delete(record, "image")
		changed = true
	}
	return changed, nil
}

func rewriteSaveRecordResaveOf(record map[string]any, ref map[string]any) (bool, error) {
	currentRaw, ok := record["resaveOf"]
	if !ok || currentRaw == nil {
		return false, fmt.Errorf("save record missing resaveOf")
	}
	current, ok := currentRaw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("save resaveOf is not an object")
	}

	desiredURI, _ := ref["uri"].(string)
	desiredCID, _ := ref["cid"].(string)
	if desiredURI == "" || desiredCID == "" {
		return false, fmt.Errorf("resolved resaveOf missing uri/cid")
	}

	currentURI, _ := current["uri"].(string)
	currentCID, _ := current["cid"].(string)
	if currentURI == desiredURI && currentCID == desiredCID {
		return false, nil
	}

	record["resaveOf"] = map[string]any{"uri": desiredURI, "cid": desiredCID}
	return true, nil
}

func putOwnedSaveMigrationRecord(ctx context.Context, globalLimiter *rate.Limiter, client *saveContentMigrationClient, record *saveContentMigrationRecord) (string, error) {
	if err := globalLimiter.Wait(ctx); err != nil {
		return "", err
	}
	if err := client.Limiter.Wait(ctx); err != nil {
		return "", err
	}

	input := &comatproto.RepoPutRecord_Input{
		Collection: saveNSID,
		Repo:       record.Repo,
		Rkey:       record.Rkey,
		Record:     record.Value,
		Validate:   boolPtr(false),
	}
	if record.CID != "" {
		input.SwapRecord = &record.CID
	}

	out, err := comatproto.RepoPutRecord(ctx, client.API, input)
	if err != nil {
		return "", err
	}
	return out.Cid, nil
}

func setSaveResaveOfCID(ctx context.Context, store *PgStore, uri, resaveOfCID string) error {
	ct, err := store.pool.Exec(ctx, `UPDATE save SET resave_of_cid = $2 WHERE uri = $1`, uri, resaveOfCID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
