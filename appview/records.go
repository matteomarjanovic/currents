package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/agnostic"
	_ "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"golang.org/x/image/draw"
)

const (
	collectionNSID = "is.currents.feed.collection"
	saveNSID       = "is.currents.feed.save"
	followNSID     = "is.currents.graph.follow"
	maxBlobSize    = 19 * 1024 * 1024
)

// resizeToLimit shrinks an image iteratively until it fits under maxBlobSize,
// re-encoding as JPEG. Returns the original data unchanged if already within limit.
// Returns new bytes and "image/jpeg" content type when resizing was needed.
func resizeToLimit(data []byte) ([]byte, string, error) {
	if len(data) <= maxBlobSize {
		return data, "", nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	for range 10 {
		bounds := img.Bounds()
		w := int(float64(bounds.Dx()) * 0.85)
		h := int(float64(bounds.Dy()) * 0.85)
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", err
		}
		if buf.Len() <= maxBlobSize {
			return buf.Bytes(), "image/jpeg", nil
		}
	}
	return nil, "", fmt.Errorf("could not shrink image below 20 MB")
}

func prepareImageForUpload(ctx context.Context, inference *InferenceClient, data []byte, contentType string) ([]byte, string, error) {
	if len(data) <= maxBlobSize {
		return data, contentType, nil
	}
	resized, newCT, err := resizeToLimit(data)
	if err == nil {
		return resized, newCT, nil
	}
	if inference == nil {
		return nil, "", err
	}
	prepared, preparedCT, prepErr := inference.PrepareImage(ctx, data, contentType, maxBlobSize)
	if prepErr != nil {
		return nil, "", fmt.Errorf("resizing image in appview: %w; preparing in inference: %w", err, prepErr)
	}
	return prepared, preparedCT, nil
}

// handleSessionError checks if an error from a PDS call is due to a dead OAuth
// session (e.g. stale refresh token after container restart). If so, it cleans
// up the session from DB and cookie, returns 401 to the client, and returns true.
func (s *Server) handleSessionError(err error, w http.ResponseWriter, r *http.Request) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	if strings.Contains(errStr, "invalid_grant") || strings.Contains(errStr, "failed to refresh OAuth tokens") {
		did, sessionID, _ := s.currentSessionDID(r)
		if did != nil {
			s.Store.DeleteSession(r.Context(), *did, sessionID)
		}
		sess, _ := s.CookieStore.Get(r, "currents-session")
		sess.Values = make(map[any]any)
		sess.Save(r, w)
		slog.Warn("cleared dead OAuth session", "did", did)
		http.Error(w, "session expired", http.StatusUnauthorized)
		return true
	}
	return false
}

func (s *Server) apiClientFromSession(r *http.Request) (*atclient.APIClient, *syntax.DID, error) {
	did, sessionID, _ := s.currentSessionDID(r)
	if did == nil {
		return nil, nil, fmt.Errorf("not authenticated")
	}
	oauthSess, err := s.OAuth.ResumeSession(r.Context(), *did, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("session error: %w", err)
	}
	return oauthSess.APIClient(), did, nil
}

func rkeyFromURI(uri string) string {
	// AT-URI format: at://<did>/<collection>/<rkey>
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func resolveStrongRef(ctx context.Context, c *atclient.APIClient, atURI string) (map[string]any, error) {
	parsed, err := syntax.ParseATURI(atURI)
	if err != nil {
		return nil, fmt.Errorf("invalid AT-URI: %w", err)
	}
	out, err := comatproto.RepoGetRecord(ctx, c, "", parsed.Collection().String(), parsed.Authority().String(), parsed.RecordKey().String())
	if err != nil {
		return nil, err
	}
	cid := ""
	if out.Cid != nil {
		cid = *out.Cid
	}
	return map[string]any{"uri": atURI, "cid": cid}, nil
}

// resolveStrongRefPublic resolves an AT-URI to a strong ref via an
// unauthenticated getRecord call to the record author's PDS. Use when the
// record being referenced is not owned by the session user.
func resolveStrongRefPublic(ctx context.Context, store *PgStore, dir identity.Directory, atURI string) (map[string]any, error) {
	parsed, err := syntax.ParseATURI(atURI)
	if err != nil {
		return nil, fmt.Errorf("invalid AT-URI: %w", err)
	}
	authorDID := parsed.Authority().String()

	pdsEndpoint, err := store.GetUserPDSEndpoint(ctx, authorDID)
	if err != nil || pdsEndpoint == "" {
		ident, err := dir.LookupDID(ctx, syntax.DID(authorDID))
		if err != nil {
			return nil, fmt.Errorf("resolving DID %s: %w", authorDID, err)
		}
		pdsEndpoint = ident.PDSEndpoint()
		if pdsEndpoint == "" {
			return nil, fmt.Errorf("no PDS endpoint for DID %s", authorDID)
		}
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.repo.getRecord?repo=%s&collection=%s&rkey=%s",
		pdsEndpoint, authorDID, parsed.Collection().String(), parsed.RecordKey().String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := blobHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getRecord: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getRecord returned %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding getRecord: %w", err)
	}
	return map[string]any{"uri": atURI, "cid": out.CID}, nil
}

// --- Collections ---

func (s *Server) CreateCollection(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	description := strings.TrimSpace(r.PostFormValue("description"))
	parentURI := strings.TrimSpace(r.PostFormValue("parent"))

	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	record := map[string]any{
		"$type":     collectionNSID,
		"name":      name,
		"createdAt": syntax.DatetimeNow().String(),
	}
	if description != "" {
		record["description"] = description
	}
	if parentURI != "" {
		parsed, err := syntax.ParseATURI(parentURI)
		if err != nil || parsed.Authority().String() != did.String() || parsed.Collection().String() != collectionNSID {
			http.Error(w, "parent must be your own is.currents.feed.collection record", http.StatusBadRequest)
			return
		}
		// Enforce a single level: the parent must itself be a root collection.
		if existing, err := s.Store.GetCollectionByURI(r.Context(), parentURI, ""); err == nil && existing != nil && existing.ParentURI != "" {
			http.Error(w, "sub-collections cannot have sub-collections", http.StatusBadRequest)
			return
		}
		ref, err := resolveStrongRef(r.Context(), c, parentURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("resolving parent: %s", err), http.StatusBadRequest)
			return
		}
		record["parent"] = ref
	}

	out, err := comatproto.RepoCreateRecord(r.Context(), c, &comatproto.RepoCreateRecord_Input{
		Collection: collectionNSID,
		Repo:       did.String(),
		Record:     record,
	})
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("creating record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("created collection", "uri", out.Uri)

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"uri":%q}`, out.Uri)
		return
	}
	http.Redirect(w, r, "/collection", http.StatusFound)
}

func (s *Server) GetCollection(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/oauth/login", http.StatusFound)
		return
	}

	rkey := r.PathValue("id")

	out, err := comatproto.RepoGetRecord(r.Context(), c, "", collectionNSID, did.String(), rkey)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	rkey := r.PathValue("id")

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	description := strings.TrimSpace(body.Description)
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	existing, err := comatproto.RepoGetRecord(r.Context(), c, "", collectionNSID, did.String(), rkey)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("fetching record: %s", err), http.StatusInternalServerError)
		return
	}
	createdAt := syntax.DatetimeNow().String()
	var parent any
	if existing.Value != nil {
		var cur map[string]any
		if err := json.Unmarshal(*existing.Value, &cur); err == nil {
			if ca, ok := cur["createdAt"].(string); ok && ca != "" {
				createdAt = ca
			}
			parent = cur["parent"]
		}
	}

	record := map[string]any{
		"$type":     collectionNSID,
		"name":      name,
		"createdAt": createdAt,
	}
	if description != "" {
		record["description"] = description
	}
	if parent != nil {
		record["parent"] = parent
	}

	out, err := comatproto.RepoPutRecord(r.Context(), c, &comatproto.RepoPutRecord_Input{
		Collection: collectionNSID,
		Repo:       did.String(),
		Rkey:       rkey,
		Record:     record,
	})
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("updating record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("updated collection", "uri", out.Uri)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"uri": out.Uri, "cid": out.Cid})
}

func (s *Server) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	_, sessionID, _ := s.currentSessionDID(r)

	rkey := r.PathValue("id")
	collectionURI := "at://" + did.String() + "/" + collectionNSID + "/" + rkey

	// Cascade: this collection's saves, plus every sub-collection (and its saves).
	saveRkeys, err := s.Store.GetSaveRkeysInCollection(r.Context(), collectionURI, did.String())
	if err != nil {
		slog.Error("listing saves for cascade", "err", err, "collection", collectionURI)
		// proceed without cascade rather than blocking the user
	}
	var subCollRkeys []string
	subURIs, err := s.Store.GetSubcollectionURIs(r.Context(), collectionURI, did.String())
	if err != nil {
		slog.Error("listing subcollections for cascade", "err", err, "collection", collectionURI)
	}
	for _, sub := range subURIs {
		subSaves, err := s.Store.GetSaveRkeysInCollection(r.Context(), sub, did.String())
		if err != nil {
			slog.Error("listing subcollection saves for cascade", "err", err, "subcollection", sub)
			continue
		}
		saveRkeys = append(saveRkeys, subSaves...)
		if rk := rkeyFromURI(sub); rk != "" {
			subCollRkeys = append(subCollRkeys, rk)
		}
	}

	if err := c.Post(r.Context(), "com.atproto.repo.deleteRecord", map[string]any{
		"repo":       did.String(),
		"collection": collectionNSID,
		"rkey":       rkey,
	}, nil); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("deleting record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("deleted collection", "rkey", rkey, "cascadeSaves", len(saveRkeys), "cascadeSubcollections", len(subCollRkeys))
	w.WriteHeader(http.StatusNoContent)

	if len(saveRkeys) > 0 || len(subCollRkeys) > 0 {
		go s.cascadeDelete(*did, sessionID, subCollRkeys, saveRkeys)
	}
}

// cascadeDelete removes the given save and collection records from the user's
// PDS in the background. Saves are deleted first, then the (sub-)collections.
func (s *Server) cascadeDelete(did syntax.DID, sessionID string, collRkeys, saveRkeys []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	oauthSess, err := s.OAuth.ResumeSession(ctx, did, sessionID)
	if err != nil {
		slog.Error("cascade: resume session", "did", did.String(), "err", err)
		return
	}
	cli := oauthSess.APIClient()
	del := func(collection, rk string) {
		if err := cli.Post(ctx, "com.atproto.repo.deleteRecord", map[string]any{
			"repo":       did.String(),
			"collection": collection,
			"rkey":       rk,
		}, nil); err != nil {
			slog.Error("cascade delete", "collection", collection, "rkey", rk, "err", err)
		}
	}
	for _, rk := range saveRkeys {
		del(saveNSID, rk)
	}
	for _, rk := range collRkeys {
		del(collectionNSID, rk)
	}
}

// --- Saves ---

func (s *Server) CreateSave(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.PostFormValue("url"))
	title := strings.TrimSpace(r.PostFormValue("title"))
	collectionURI := strings.TrimSpace(r.PostFormValue("collection"))
	resaveOfURI := strings.TrimSpace(r.PostFormValue("resaveOf"))
	attrURL := strings.TrimSpace(r.PostFormValue("attribution_url"))
	attrLicense := strings.TrimSpace(r.PostFormValue("attribution_license"))
	attrCredit := strings.TrimSpace(r.PostFormValue("attribution_credit"))
	selfLabelVals := parseSelfLabels(r.PostFormValue("labels"))

	if collectionURI == "" {
		http.Error(w, "collection is required", http.StatusBadRequest)
		return
	}

	// Require image
	file, header, fileErr := r.FormFile("image")
	if fileErr != nil {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	imageBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "reading image file", http.StatusInternalServerError)
		return
	}
	if len(imageBytes) > maxBlobSize {
		prepared, preparedCT, err := prepareImageForUpload(r.Context(), s.Inference, imageBytes, contentType)
		if err != nil {
			http.Error(w, "image too large and could not be prepared for upload", http.StatusBadRequest)
			return
		}
		imageBytes = prepared
		contentType = preparedCT
	}
	var uploadOut struct {
		Blob lexutil.LexBlob `json:"blob"`
	}
	if err := c.LexDo(r.Context(), "POST", contentType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(imageBytes), &uploadOut); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("uploading image: %s", err), http.StatusInternalServerError)
		return
	}
	blobJSON, _ := json.Marshal(uploadOut.Blob)
	var blobAny any
	json.Unmarshal(blobJSON, &blobAny)

	// Resolve collection strongRef
	collectionStrongRef, err := resolveStrongRef(r.Context(), c, collectionURI)
	if err != nil {
		http.Error(w, fmt.Sprintf("resolving collection: %s", err), http.StatusBadRequest)
		return
	}

	record := map[string]any{
		"$type":      saveNSID,
		"collection": collectionStrongRef,
		"content":    buildImageContentRecordWithAttribution(blobAny, saveAttributionFromFields(attrURL, attrLicense, attrCredit)),
		"createdAt":  syntax.DatetimeNow().String(),
	}
	if labels := buildSelfLabelsRecord(selfLabelVals); labels != nil {
		record["labels"] = labels
	}
	if url != "" {
		record["originUrl"] = url
	}
	if title != "" {
		record["text"] = title
	}
	if resaveOfURI != "" {
		resaveRef, err := resolveStrongRef(r.Context(), c, resaveOfURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("resolving resaveOf: %s", err), http.StatusBadRequest)
			return
		}
		record["resaveOf"] = resaveRef
	}

	out, err := comatproto.RepoCreateRecord(r.Context(), c, &comatproto.RepoCreateRecord_Input{
		Collection: saveNSID,
		Repo:       did.String(),
		Record:     record,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("creating record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("created save", "uri", out.Uri)
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"uri":%q}`, out.Uri)
		return
	}
	http.Redirect(w, r, "/save", http.StatusFound)
}

func (s *Server) GetSave(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/oauth/login", http.StatusFound)
		return
	}

	rkey := r.PathValue("id")

	out, err := comatproto.RepoGetRecord(r.Context(), c, "", saveNSID, did.String(), rkey)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) UpdateSave(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	rkey := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.PostFormValue("url"))
	title := strings.TrimSpace(r.PostFormValue("title"))
	collectionURI := strings.TrimSpace(r.PostFormValue("collection"))
	attrURL := strings.TrimSpace(r.PostFormValue("attribution_url"))
	attrLicense := strings.TrimSpace(r.PostFormValue("attribution_license"))
	attrCredit := strings.TrimSpace(r.PostFormValue("attribution_credit"))

	if collectionURI == "" {
		http.Error(w, "collection is required", http.StatusBadRequest)
		return
	}

	// Fetch existing record to preserve content and fields not in the form.
	existing, err := comatproto.RepoGetRecord(r.Context(), c, "", saveNSID, did.String(), rkey)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("fetching existing save: %s", err), http.StatusInternalServerError)
		return
	}
	var existingVal struct {
		Content   json.RawMessage `json:"content"`
		CreatedAt string          `json:"createdAt"`
		OriginURL string          `json:"originUrl"`
		Text      string          `json:"text"`
		ResaveOf  json.RawMessage `json:"resaveOf"`
		Labels    json.RawMessage `json:"labels"`
	}
	if existing.Value != nil {
		json.Unmarshal(*existing.Value, &existingVal)
	}
	contentAny, err := buildSaveContentWithAttribution(
		existingVal.Content,
		saveAttributionFromFields(attrURL, attrLicense, attrCredit),
		false,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing existing save content: %s", err), http.StatusInternalServerError)
		return
	}

	// Resolve collection strongRef
	collectionStrongRef, err := resolveStrongRef(r.Context(), c, collectionURI)
	if err != nil {
		http.Error(w, fmt.Sprintf("resolving collection: %s", err), http.StatusBadRequest)
		return
	}

	record := map[string]any{
		"$type":      saveNSID,
		"collection": collectionStrongRef,
		"content":    contentAny,
		"createdAt":  existingVal.CreatedAt,
	}

	// Use form value if provided, otherwise preserve existing
	if url != "" {
		record["originUrl"] = url
	} else if existingVal.OriginURL != "" {
		record["originUrl"] = existingVal.OriginURL
	}
	if title != "" {
		record["text"] = title
	} else if existingVal.Text != "" {
		record["text"] = existingVal.Text
	}

	// Preserve resaveOf — not editable
	if existingVal.ResaveOf != nil {
		var resaveAny any
		json.Unmarshal(existingVal.ResaveOf, &resaveAny)
		record["resaveOf"] = resaveAny
	}

	// Preserve self-labels — RepoPutRecord replaces the whole record, so editing
	// other fields must not strip the creator's content-warning declaration.
	if len(existingVal.Labels) > 0 && string(existingVal.Labels) != "null" {
		var labelsAny any
		if json.Unmarshal(existingVal.Labels, &labelsAny) == nil {
			record["labels"] = labelsAny
		}
	}

	out, err := comatproto.RepoPutRecord(r.Context(), c, &comatproto.RepoPutRecord_Input{
		Collection: saveNSID,
		Repo:       did.String(),
		Rkey:       rkey,
		Record:     record,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("updating record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("updated save", "uri", out.Uri)
	http.Redirect(w, r, "/save", http.StatusFound)
}

// UpdateSaveAttribution applies attribution fields to every save record in the
// viewer's PDS that shares the given blob CID. PutRecord calls fan out in
// parallel goroutines since N is typically small (a few collections).
func (s *Server) UpdateSaveAttribution(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	blobCID := strings.TrimSpace(r.PostFormValue("blob_cid"))
	if blobCID == "" {
		http.Error(w, "blob_cid is required", http.StatusBadRequest)
		return
	}
	attribution := saveAttributionFromFields(
		strings.TrimSpace(r.PostFormValue("attribution_url")),
		strings.TrimSpace(r.PostFormValue("attribution_license")),
		strings.TrimSpace(r.PostFormValue("attribution_credit")),
	)

	rkeys, err := s.Store.GetSaveRkeysByAuthorAndBlob(r.Context(), did.String(), blobCID)
	if err != nil {
		slog.Error("GetSaveRkeysByAuthorAndBlob", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(rkeys) == 0 {
		http.Error(w, "no saves for this blob", http.StatusNotFound)
		return
	}

	var wg sync.WaitGroup
	var ok atomic.Int64
	for _, rkey := range rkeys {
		wg.Add(1)
		go func(rkey string) {
			defer wg.Done()
			if err := s.putAttributionForRkey(r.Context(), c, did, rkey, attribution); err != nil {
				slog.Warn("attribution update failed", "rkey", rkey, "err", err)
				return
			}
			ok.Add(1)
		}(rkey)
	}
	wg.Wait()

	updated := int(ok.Load())
	if updated == 0 {
		http.Error(w, "all PDS updates failed", http.StatusInternalServerError)
		return
	}

	slog.Info("updated save attribution", "did", did.String(), "blob_cid", blobCID, "updated", updated, "total", len(rkeys))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"updated": updated})
}

// putAttributionForRkey rebuilds a single save record with the given attribution
// applied to its image content, preserving all other fields, and writes it back
// to the viewer's PDS via RepoPutRecord.
func (s *Server) putAttributionForRkey(ctx context.Context, c *atclient.APIClient, did *syntax.DID, rkey string, attribution *saveAttribution) error {
	existing, err := comatproto.RepoGetRecord(ctx, c, "", saveNSID, did.String(), rkey)
	if err != nil {
		return fmt.Errorf("get existing: %w", err)
	}
	var existingVal struct {
		Content    json.RawMessage `json:"content"`
		Collection json.RawMessage `json:"collection"`
		CreatedAt  string          `json:"createdAt"`
		OriginURL  string          `json:"originUrl"`
		Text       string          `json:"text"`
		ResaveOf   json.RawMessage `json:"resaveOf"`
		Labels     json.RawMessage `json:"labels"`
	}
	if existing.Value != nil {
		if err := json.Unmarshal(*existing.Value, &existingVal); err != nil {
			return fmt.Errorf("unmarshal existing: %w", err)
		}
	}

	contentAny, err := buildSaveContentWithAttribution(existingVal.Content, attribution, true)
	if err != nil {
		return fmt.Errorf("build content: %w", err)
	}

	record := map[string]any{
		"$type":     saveNSID,
		"content":   contentAny,
		"createdAt": existingVal.CreatedAt,
	}
	if existingVal.Collection != nil {
		var collectionAny any
		if err := json.Unmarshal(existingVal.Collection, &collectionAny); err != nil {
			return fmt.Errorf("unmarshal collection: %w", err)
		}
		record["collection"] = collectionAny
	}
	if existingVal.OriginURL != "" {
		record["originUrl"] = existingVal.OriginURL
	}
	if existingVal.Text != "" {
		record["text"] = existingVal.Text
	}
	if existingVal.ResaveOf != nil {
		var resaveAny any
		if err := json.Unmarshal(existingVal.ResaveOf, &resaveAny); err != nil {
			return fmt.Errorf("unmarshal resaveOf: %w", err)
		}
		record["resaveOf"] = resaveAny
	}
	// Preserve self-labels — RepoPutRecord replaces the whole record.
	if len(existingVal.Labels) > 0 && string(existingVal.Labels) != "null" {
		var labelsAny any
		if err := json.Unmarshal(existingVal.Labels, &labelsAny); err == nil {
			record["labels"] = labelsAny
		}
	}

	if _, err := comatproto.RepoPutRecord(ctx, c, &comatproto.RepoPutRecord_Input{
		Collection: saveNSID,
		Repo:       did.String(),
		Rkey:       rkey,
		Record:     record,
	}); err != nil {
		return fmt.Errorf("put record: %w", err)
	}
	return nil
}

// applyLabelsToOwnedSave merges newLabels (add-only) into the self-labels of one
// of the viewer's own saves and writes the record back. It returns the resulting
// label set, whether the record was actually updated, and whether it was skipped
// because the save is a resave (only originators self-label). RepoPutRecord
// rewrites the record's `labels` field; the TAP listener then re-issues and fans
// out the labeler labels via the normal save-upsert path, so no propagation logic
// is duplicated here. Shared by the single-save and bulk endpoints.
func applyLabelsToOwnedSave(ctx context.Context, c *atclient.APIClient, did *syntax.DID, rkey string, newLabels []string) (vals []string, applied bool, isResave bool, err error) {
	existing, err := comatproto.RepoGetRecord(ctx, c, "", saveNSID, did.String(), rkey)
	if err != nil {
		return nil, false, false, fmt.Errorf("get record: %w", err)
	}
	var existingVal struct {
		Content    json.RawMessage `json:"content"`
		Collection json.RawMessage `json:"collection"`
		CreatedAt  string          `json:"createdAt"`
		OriginURL  string          `json:"originUrl"`
		Text       string          `json:"text"`
		ResaveOf   json.RawMessage `json:"resaveOf"`
		Labels     *selfLabels     `json:"labels"`
	}
	if existing.Value != nil {
		if err := json.Unmarshal(*existing.Value, &existingVal); err != nil {
			return nil, false, false, fmt.Errorf("unmarshal save: %w", err)
		}
	}
	if existingVal.ResaveOf != nil && string(existingVal.ResaveOf) != "null" {
		return nil, false, true, nil
	}

	// Add-only merge: existing self-labels ∪ submitted (dedup, preserve order).
	have := map[string]bool{}
	if existingVal.Labels != nil {
		for _, lv := range existingVal.Labels.Values {
			if _, ok := allowedSelfLabelVals[lv.Val]; ok && !have[lv.Val] {
				have[lv.Val] = true
				vals = append(vals, lv.Val)
			}
		}
	}
	for _, v := range newLabels {
		if !have[v] {
			have[v] = true
			vals = append(vals, v)
			applied = true
		}
	}
	if !applied {
		return vals, false, false, nil // nothing new to add
	}

	record := map[string]any{
		"$type":     saveNSID,
		"createdAt": existingVal.CreatedAt,
		"labels":    buildSelfLabelsRecord(vals),
	}
	if existingVal.Content != nil {
		var contentAny any
		json.Unmarshal(existingVal.Content, &contentAny)
		record["content"] = contentAny
	}
	if existingVal.Collection != nil {
		var collectionAny any
		json.Unmarshal(existingVal.Collection, &collectionAny)
		record["collection"] = collectionAny
	}
	if existingVal.OriginURL != "" {
		record["originUrl"] = existingVal.OriginURL
	}
	if existingVal.Text != "" {
		record["text"] = existingVal.Text
	}
	if _, err := comatproto.RepoPutRecord(ctx, c, &comatproto.RepoPutRecord_Input{
		Collection: saveNSID,
		Repo:       did.String(),
		Rkey:       rkey,
		Record:     record,
	}); err != nil {
		return nil, false, false, fmt.Errorf("put record: %w", err)
	}
	return vals, true, false, nil
}

// UpdateSaveLabels adds creator self-labels to a single save the viewer owns (the
// save-detail editor). Add-only; disallowed on resaves. See applyLabelsToOwnedSave.
func (s *Server) UpdateSaveLabels(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	rkey := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	newLabels := parseSelfLabels(r.PostFormValue("labels"))
	if len(newLabels) == 0 {
		http.Error(w, "no valid labels", http.StatusBadRequest)
		return
	}

	vals, _, isResave, err := applyLabelsToOwnedSave(r.Context(), c, did, rkey, newLabels)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("updating labels: %s", err), http.StatusInternalServerError)
		return
	}
	if isResave {
		http.Error(w, "cannot add labels to a resave", http.StatusForbidden)
		return
	}

	uri := "at://" + did.String() + "/" + saveNSID + "/" + rkey
	slog.Info("updated save labels", "uri", uri, "labels", vals)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"uri": uri, "labels": vals})
}

// UpdateSaveLabelsBulk applies the same add-only self-labels to many of the
// viewer's saves at once (collection-page bulk labeling). Resaves are skipped
// server-side regardless of the UI. PutRecords fan out with bounded concurrency.
func (s *Server) UpdateSaveLabelsBulk(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var body struct {
		Rkeys  []string `json:"rkeys"`
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	// Validate against the allowed self-label vocab (dedup, preserve order).
	seen := map[string]bool{}
	var newLabels []string
	for _, v := range body.Labels {
		if _, ok := allowedSelfLabelVals[v]; ok && !seen[v] {
			seen[v] = true
			newLabels = append(newLabels, v)
		}
	}
	if len(newLabels) == 0 || len(body.Rkeys) == 0 {
		http.Error(w, "rkeys and labels are required", http.StatusBadRequest)
		return
	}
	if len(body.Rkeys) > 500 {
		http.Error(w, "too many saves (max 500 per request)", http.StatusBadRequest)
		return
	}

	var wg sync.WaitGroup
	var applied, skipped, failed atomic.Int64
	sem := make(chan struct{}, 8)
	for _, rkey := range body.Rkeys {
		wg.Add(1)
		go func(rkey string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			_, ok, isResave, err := applyLabelsToOwnedSave(r.Context(), c, did, rkey, newLabels)
			switch {
			case err != nil:
				slog.Warn("bulk label apply failed", "rkey", rkey, "err", err)
				failed.Add(1)
			case isResave, !ok:
				skipped.Add(1)
			default:
				applied.Add(1)
			}
		}(rkey)
	}
	wg.Wait()

	slog.Info("bulk applied save labels", "did", did.String(),
		"applied", applied.Load(), "skipped", skipped.Load(), "failed", failed.Load())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{
		"applied": applied.Load(),
		"skipped": skipped.Load(),
		"failed":  failed.Load(),
	})
}

func (s *Server) DeleteSave(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	rkey := r.PathValue("id")

	if err := c.Post(r.Context(), "com.atproto.repo.deleteRecord", map[string]any{
		"repo":       did.String(),
		"collection": saveNSID,
		"rkey":       rkey,
	}, nil); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("deleting record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("deleted save", "rkey", rkey)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CreateFollow(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var body struct {
		Subject string `json:"subject"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Subject == "" {
		http.Error(w, "subject is required", http.StatusBadRequest)
		return
	}
	if _, err := syntax.ParseDID(body.Subject); err != nil {
		http.Error(w, "invalid subject DID", http.StatusBadRequest)
		return
	}

	out, err := comatproto.RepoCreateRecord(r.Context(), c, &comatproto.RepoCreateRecord_Input{
		Collection: followNSID,
		Repo:       did.String(),
		Record: map[string]any{
			"$type":     followNSID,
			"subject":   body.Subject,
			"createdAt": syntax.DatetimeNow().String(),
		},
	})
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		if strings.Contains(err.Error(), "ScopeMissingError") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "ScopeMissing"})
			return
		}
		slog.Error("creating follow", "err", err, "subject", body.Subject, "follower", did.String())
		http.Error(w, fmt.Sprintf("creating follow: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("created follow", "uri", out.Uri)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"uri":%q}`, out.Uri)
}

func (s *Server) DeleteFollow(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	rkey := r.PathValue("rkey")

	if err := c.Post(r.Context(), "com.atproto.repo.deleteRecord", map[string]any{
		"repo":       did.String(),
		"collection": followNSID,
		"rkey":       rkey,
	}, nil); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("deleting follow: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("deleted follow", "rkey", rkey)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CreateResave(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var body struct {
		SaveURI       string `json:"saveUri"`
		CollectionURI string `json:"collectionUri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.SaveURI == "" || body.CollectionURI == "" {
		http.Error(w, "saveUri and collectionUri are required", http.StatusBadRequest)
		return
	}

	// Look up the original save to get blob info, source URL, and attribution
	var authorDID, blobCID, origOriginURL string
	var origAttrURL, origAttrLicense, origAttrCredit string
	err = s.Store.pool.QueryRow(r.Context(),
		`SELECT author_did, pds_blob_cid, COALESCE(origin_url, ''), COALESCE(attribution_url, ''), COALESCE(attribution_license, ''), COALESCE(attribution_credit, '') FROM save WHERE uri = $1`, body.SaveURI,
	).Scan(&authorDID, &blobCID, &origOriginURL, &origAttrURL, &origAttrLicense, &origAttrCredit)
	if err != nil {
		http.Error(w, "save not found", http.StatusNotFound)
		return
	}

	// Fetch blob from the original author's PDS
	imageBytes, contentType, err := fetchBlobFromPDS(r.Context(), s.Store, s.Dir, authorDID, blobCID)
	if err != nil {
		slog.Error("fetching blob for resave", "err", err)
		http.Error(w, "could not fetch image", http.StatusBadGateway)
		return
	}

	// Upload blob to the viewer's PDS
	var uploadOut struct {
		Blob lexutil.LexBlob `json:"blob"`
	}
	if err := c.LexDo(r.Context(), "POST", contentType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(imageBytes), &uploadOut); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("uploading image: %s", err), http.StatusInternalServerError)
		return
	}
	blobJSON, _ := json.Marshal(uploadOut.Blob)
	var blobAny any
	json.Unmarshal(blobJSON, &blobAny)

	// Resolve strong refs
	collectionStrongRef, err := resolveStrongRef(r.Context(), c, body.CollectionURI)
	if err != nil {
		http.Error(w, fmt.Sprintf("resolving collection: %s", err), http.StatusBadRequest)
		return
	}
	resaveRef, err := resolveStrongRefPublic(r.Context(), s.Store, s.Dir, body.SaveURI)
	if err != nil {
		http.Error(w, fmt.Sprintf("resolving save: %s", err), http.StatusBadRequest)
		return
	}

	// Check if the viewer already has their own attribution for this blob; viewer's attribution takes priority
	var viewerAttrURL, viewerAttrLicense, viewerAttrCredit string
	_ = s.Store.pool.QueryRow(r.Context(),
		`SELECT COALESCE(attribution_url, ''), COALESCE(attribution_license, ''), COALESCE(attribution_credit, '')
		 FROM save WHERE author_did = $1 AND pds_blob_cid = $2
		   AND (COALESCE(attribution_url, '') <> '' OR COALESCE(attribution_license, '') <> '' OR COALESCE(attribution_credit, '') <> '')
		 ORDER BY created_at DESC NULLS LAST LIMIT 1`,
		did.String(), blobCID,
	).Scan(&viewerAttrURL, &viewerAttrLicense, &viewerAttrCredit)

	resolvedAttribution := saveAttributionFromFields(viewerAttrURL, viewerAttrLicense, viewerAttrCredit)
	if resolvedAttribution == nil {
		resolvedAttribution = saveAttributionFromFields(origAttrURL, origAttrLicense, origAttrCredit)
	}

	record := map[string]any{
		"$type":      saveNSID,
		"collection": collectionStrongRef,
		"content":    buildImageContentRecordWithAttribution(blobAny, resolvedAttribution),
		"resaveOf":   resaveRef,
		"createdAt":  syntax.DatetimeNow().String(),
	}
	if origOriginURL != "" {
		record["originUrl"] = origOriginURL
	}

	out, err := comatproto.RepoCreateRecord(r.Context(), c, &comatproto.RepoCreateRecord_Input{
		Collection: saveNSID,
		Repo:       did.String(),
		Record:     record,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("creating record: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("created resave", "uri", out.Uri)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"uri":%q}`, out.Uri)
}
