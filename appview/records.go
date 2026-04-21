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

type CollectionData struct {
	URI         string
	Rkey        string
	Name        string
	Description string
	CreatedAt   string
}

type SaveData struct {
	URI        string
	Rkey       string
	OriginURL  string
	Title      string
	Collection string
	CreatedAt  string
}

type CollectionsPageData struct {
	TmplData
	Collections []CollectionData
}

type SavesPageData struct {
	TmplData
	Saves       []SaveData
	Collections []CollectionData
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

func (s *Server) ListCollections(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/oauth/login", http.StatusFound)
		return
	}

	_, sessionID, handle := s.currentSessionDID(r)
	_ = sessionID

	out, err := comatproto.RepoListRecords(r.Context(), c, collectionNSID, "", 0, did.String(), false)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		slog.Error("listing collections", "err", err)
		tmplError.Execute(w, TmplData{DID: did, Handle: handle, Error: err.Error()})
		return
	}

	var collections []CollectionData
	for _, rec := range out.Records {
		if rec.Value == nil {
			continue
		}
		var val struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			CreatedAt   string `json:"createdAt"`
		}
		if err := json.Unmarshal(*rec.Value, &val); err != nil {
			slog.Warn("parsing collection record", "uri", rec.Uri, "err", err)
			continue
		}
		collections = append(collections, CollectionData{
			URI:         rec.Uri,
			Rkey:        rkeyFromURI(rec.Uri),
			Name:        val.Name,
			Description: val.Description,
			CreatedAt:   val.CreatedAt,
		})
	}

	tmplCollections.Execute(w, CollectionsPageData{
		TmplData:    TmplData{DID: did, Handle: handle},
		Collections: collections,
	})
}

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

	_, _, handle := s.currentSessionDID(r)
	rkey := r.PathValue("id")

	out, err := comatproto.RepoGetRecord(r.Context(), c, "", collectionNSID, did.String(), rkey)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		tmplError.Execute(w, TmplData{DID: did, Handle: handle, Error: err.Error()})
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
	if existing.Value != nil {
		var cur map[string]any
		if err := json.Unmarshal(*existing.Value, &cur); err == nil {
			if ca, ok := cur["createdAt"].(string); ok && ca != "" {
				createdAt = ca
			}
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

	saveRkeys, err := s.Store.GetSaveRkeysInCollection(r.Context(), collectionURI, did.String())
	if err != nil {
		slog.Error("listing saves for cascade", "err", err, "collection", collectionURI)
		// proceed without cascade rather than blocking the user
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

	slog.Info("deleted collection", "rkey", rkey, "cascadeSaves", len(saveRkeys))
	w.WriteHeader(http.StatusNoContent)

	if len(saveRkeys) > 0 {
		go s.cascadeDeleteSaves(*did, sessionID, saveRkeys)
	}
}

func (s *Server) cascadeDeleteSaves(did syntax.DID, sessionID string, rkeys []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	oauthSess, err := s.OAuth.ResumeSession(ctx, did, sessionID)
	if err != nil {
		slog.Error("cascade: resume session", "did", did.String(), "err", err)
		return
	}
	cli := oauthSess.APIClient()
	for _, rk := range rkeys {
		if err := cli.Post(ctx, "com.atproto.repo.deleteRecord", map[string]any{
			"repo":       did.String(),
			"collection": saveNSID,
			"rkey":       rk,
		}, nil); err != nil {
			slog.Error("cascade delete save", "rkey", rk, "err", err)
		}
	}
}

// --- Saves ---

func (s *Server) ListSaves(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/oauth/login", http.StatusFound)
		return
	}

	_, _, handle := s.currentSessionDID(r)

	savesOut, err := comatproto.RepoListRecords(r.Context(), c, saveNSID, "", 0, did.String(), false)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		slog.Error("listing saves", "err", err)
		tmplError.Execute(w, TmplData{DID: did, Handle: handle, Error: err.Error()})
		return
	}

	var saves []SaveData
	for _, rec := range savesOut.Records {
		if rec.Value == nil {
			continue
		}
		var val struct {
			OriginURL  string `json:"originUrl"`
			Text       string `json:"text"`
			Collection struct {
				URI string `json:"uri"`
			} `json:"collection"`
			CreatedAt string `json:"createdAt"`
		}
		if err := json.Unmarshal(*rec.Value, &val); err != nil {
			slog.Warn("parsing save record", "uri", rec.Uri, "err", err)
			continue
		}
		saves = append(saves, SaveData{
			URI:        rec.Uri,
			Rkey:       rkeyFromURI(rec.Uri),
			OriginURL:  val.OriginURL,
			Title:      val.Text,
			Collection: val.Collection.URI,
			CreatedAt:  val.CreatedAt,
		})
	}

	colsOut, err := comatproto.RepoListRecords(r.Context(), c, collectionNSID, "", 0, did.String(), false)
	if err != nil {
		slog.Warn("listing collections for saves page", "err", err)
	}
	var collections []CollectionData
	if colsOut != nil {
		for _, rec := range colsOut.Records {
			if rec.Value == nil {
				continue
			}
			var val struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(*rec.Value, &val); err != nil {
				continue
			}
			collections = append(collections, CollectionData{
				URI:  rec.Uri,
				Rkey: rkeyFromURI(rec.Uri),
				Name: val.Name,
			})
		}
	}

	tmplSaves.Execute(w, SavesPageData{
		TmplData:    TmplData{DID: did, Handle: handle},
		Saves:       saves,
		Collections: collections,
	})
}

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
		_, _, handle := s.currentSessionDID(r)
		tmplError.Execute(w, TmplData{DID: did, Handle: handle, Error: err.Error()})
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
	}
	if existing.Value != nil {
		json.Unmarshal(*existing.Value, &existingVal)
	}
	contentAny, err := buildSaveContentWithAttribution(
		existingVal.Content,
		saveAttributionFromFields(attrURL, attrLicense, attrCredit),
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

	// Look up the original save to get blob info
	var authorDID, blobCID string
	err = s.Store.pool.QueryRow(r.Context(),
		`SELECT author_did, pds_blob_cid FROM save WHERE uri = $1`, body.SaveURI,
	).Scan(&authorDID, &blobCID)
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

	record := map[string]any{
		"$type":      saveNSID,
		"collection": collectionStrongRef,
		"content":    buildImageContentRecord(blobAny),
		"resaveOf":   resaveRef,
		"createdAt":  syntax.DatetimeNow().String(),
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
