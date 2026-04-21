package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
)

const maxProfileBlobSize = 1_000_000

type profileDraftResponse struct {
	DisplayName string       `json:"displayName,omitempty"`
	Description string       `json:"description,omitempty"`
	Avatar      string       `json:"avatar,omitempty"`
	Banner      string       `json:"banner,omitempty"`
	AvatarBlob  *repoBlobRef `json:"avatarBlob,omitempty"`
	BannerBlob  *repoBlobRef `json:"bannerBlob,omitempty"`
}

type actorProfileResponse struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Pronouns    string `json:"pronouns,omitempty"`
	Website     string `json:"website,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Banner      string `json:"banner,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

type profileUpdate struct {
	DisplayName  string
	Description  string
	Pronouns     string
	Website      string
	Avatar       *repoBlobRef
	Banner       *repoBlobRef
	RemoveAvatar bool
	RemoveBanner bool
}

func isRecordNotFound(err error) bool {
	var apiErr *atclient.APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || (apiErr.StatusCode == http.StatusBadRequest && apiErr.Name == "RecordNotFound"))
}

func getProfileRecord[T any](ctx context.Context, c *atclient.APIClient, did, collection string) (*T, bool, error) {
	var resp struct {
		Value json.RawMessage `json:"value"`
	}
	err := c.Get(ctx, "com.atproto.repo.getRecord", map[string]any{
		"repo":       did,
		"collection": collection,
		"rkey":       "self",
	}, &resp)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(resp.Value) == 0 {
		return nil, true, nil
	}
	var out T
	if err := json.Unmarshal(resp.Value, &out); err != nil {
		return nil, true, err
	}
	return &out, true, nil
}

func buildCurrentsProfileRecord(existing *currentsProfileRecord, update profileUpdate, createdAt string) currentsProfileRecord {
	record := currentsProfileRecord{
		DisplayName: update.DisplayName,
		Description: update.Description,
		Pronouns:    update.Pronouns,
		Website:     update.Website,
		CreatedAt:   createdAt,
	}
	if existing != nil && existing.CreatedAt != "" {
		record.CreatedAt = existing.CreatedAt
	}
	if !update.RemoveAvatar {
		switch {
		case update.Avatar != nil:
			record.Avatar = update.Avatar
		case existing != nil:
			record.Avatar = existing.Avatar
		}
	}
	if !update.RemoveBanner {
		switch {
		case update.Banner != nil:
			record.Banner = update.Banner
		case existing != nil:
			record.Banner = existing.Banner
		}
	}
	return record
}

func profileRecordPayload(record currentsProfileRecord) map[string]any {
	payload := map[string]any{
		"$type":     currentsProfileNSID,
		"createdAt": record.CreatedAt,
	}
	if record.DisplayName != "" {
		payload["displayName"] = record.DisplayName
	}
	if record.Description != "" {
		payload["description"] = record.Description
	}
	if record.Pronouns != "" {
		payload["pronouns"] = record.Pronouns
	}
	if record.Website != "" {
		payload["website"] = record.Website
	}
	if record.Avatar != nil {
		payload["avatar"] = record.Avatar
	}
	if record.Banner != nil {
		payload["banner"] = record.Banner
	}
	return payload
}

func profileResponseFromUserRecord(record UserRecord) actorProfileResponse {
	resp := actorProfileResponse{
		DID:         record.DID,
		Handle:      record.Handle,
		DisplayName: record.DisplayName,
		Description: record.Description,
		Pronouns:    record.Pronouns,
		Website:     record.Website,
		Avatar:      record.Avatar,
		Banner:      record.Banner,
	}
	if !record.CreatedAt.IsZero() {
		resp.CreatedAt = record.CreatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func normalizeProfileImageContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpg":
		return "image/jpeg"
	case "image/jpeg", "image/png":
		return strings.ToLower(strings.TrimSpace(contentType))
	default:
		return ""
	}
}

func parseProfileBlobField(raw string) (*repoBlobRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var blob repoBlobRef
	if err := json.Unmarshal([]byte(raw), &blob); err != nil {
		return nil, err
	}
	if blob.Ref["$link"] == "" {
		return nil, fmt.Errorf("missing blob ref")
	}
	return &blob, nil
}

func truthyFormValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

func uploadProfileBlob(ctx context.Context, c *atclient.APIClient, inference *InferenceClient, file multipart.File, header *multipart.FileHeader) (*repoBlobRef, error) {
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("reading image file: %w", err)
	}
	contentType := normalizeProfileImageContentType(header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = normalizeProfileImageContentType(http.DetectContentType(data))
	}
	if contentType == "" {
		return nil, fmt.Errorf("profile images must be PNG or JPEG")
	}
	if len(data) > maxProfileBlobSize {
		if inference == nil {
			return nil, fmt.Errorf("profile image exceeds 1 MB")
		}
		prepared, preparedContentType, err := inference.PrepareImage(ctx, data, contentType, maxProfileBlobSize)
		if err != nil {
			return nil, fmt.Errorf("profile image exceeds 1 MB and could not be prepared")
		}
		data = prepared
		contentType = normalizeProfileImageContentType(preparedContentType)
		if contentType == "" {
			return nil, fmt.Errorf("prepared profile image must be PNG or JPEG")
		}
		if len(data) > maxProfileBlobSize {
			return nil, fmt.Errorf("profile image exceeds 1 MB")
		}
	}

	var uploadOut struct {
		Blob lexutil.LexBlob `json:"blob"`
	}
	if err := c.LexDo(ctx, "POST", contentType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(data), &uploadOut); err != nil {
		return nil, err
	}
	blobJSON, _ := json.Marshal(uploadOut.Blob)
	var blob repoBlobRef
	if err := json.Unmarshal(blobJSON, &blob); err != nil {
		return nil, fmt.Errorf("decoding uploaded blob: %w", err)
	}
	return &blob, nil
}

func optionalProfileBlobUpload(r *http.Request, field string, c *atclient.APIClient, inference *InferenceClient) (*repoBlobRef, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}
	return uploadProfileBlob(r.Context(), c, inference, file, header)
}

func (s *Server) APIImportBlueskyProfile(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	bskyProfile, found, err := getProfileRecord[bskyActorProfile](r.Context(), c, did.String(), "app.bsky.actor.profile")
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("fetching Bluesky profile: %s", err), http.StatusInternalServerError)
		return
	}
	if !found || bskyProfile == nil {
		http.Error(w, "Bluesky profile not found", http.StatusNotFound)
		return
	}

	profile := currentsProfileFromBskyProfile(*bskyProfile, "")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profileDraftResponse{
		DisplayName: profile.DisplayName,
		Description: profile.Description,
		Avatar:      profileBlobURL(s.CDNBaseURL, did.String(), profile.Avatar),
		Banner:      profileBlobURL(s.CDNBaseURL, did.String(), profile.Banner),
		AvatarBlob:  profile.Avatar,
		BannerBlob:  profile.Banner,
	})
}

func (s *Server) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	did, sessionID, handle := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	oauthSess, err := s.OAuth.ResumeSession(r.Context(), *did, sessionID)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, "session error", http.StatusUnauthorized)
		return
	}
	c := oauthSess.APIClient()

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	existingProfile, found, err := getProfileRecord[currentsProfileRecord](r.Context(), c, did.String(), currentsProfileNSID)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("fetching profile: %s", err), http.StatusInternalServerError)
		return
	}
	if !found {
		existingProfile = nil
	}

	avatarBlobField, err := parseProfileBlobField(r.FormValue("avatarBlob"))
	if err != nil {
		http.Error(w, "invalid avatar blob", http.StatusBadRequest)
		return
	}
	bannerBlobField, err := parseProfileBlobField(r.FormValue("bannerBlob"))
	if err != nil {
		http.Error(w, "invalid banner blob", http.StatusBadRequest)
		return
	}
	uploadedAvatar, err := optionalProfileBlobUpload(r, "avatar", c, s.Inference)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uploadedBanner, err := optionalProfileBlobUpload(r, "banner", c, s.Inference)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	avatarBlob := avatarBlobField
	if uploadedAvatar != nil {
		avatarBlob = uploadedAvatar
	}
	bannerBlob := bannerBlobField
	if uploadedBanner != nil {
		bannerBlob = uploadedBanner
	}

	profile := buildCurrentsProfileRecord(existingProfile, profileUpdate{
		DisplayName:  strings.TrimSpace(r.FormValue("displayName")),
		Description:  strings.TrimSpace(r.FormValue("description")),
		Pronouns:     strings.TrimSpace(r.FormValue("pronouns")),
		Website:      strings.TrimSpace(r.FormValue("website")),
		Avatar:       avatarBlob,
		Banner:       bannerBlob,
		RemoveAvatar: truthyFormValue(r.FormValue("removeAvatar")),
		RemoveBanner: truthyFormValue(r.FormValue("removeBanner")),
	}, syntax.DatetimeNow().String())

	if _, err := comatproto.RepoPutRecord(r.Context(), c, &comatproto.RepoPutRecord_Input{
		Collection: currentsProfileNSID,
		Repo:       did.String(),
		Rkey:       "self",
		Record:     profileRecordPayload(profile),
	}); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("updating profile: %s", err), http.StatusInternalServerError)
		return
	}

	userRecord := userRecordFromCurrentsProfile(did.String(), handle, oauthSess.Data.HostURL, s.CDNBaseURL, profile, time.Now())
	if err := s.Store.CreateUser(r.Context(), userRecord); err != nil {
		// Keep the record write successful; TAP will reconcile later if the DB upsert fails here.
		slog.Error("updating user row after profile save", "did", did.String(), "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profileResponseFromUserRecord(userRecord))
}
