package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	_ "github.com/gen2brain/avif"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	collectionNSID = "is.currents.feed.collection"
	saveNSID       = "is.currents.feed.save"
	followNSID     = "is.currents.graph.follow"
	favouriteNSID  = "is.currents.graph.favourite"
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

	// Parent is a pointer so the field is tri-state: absent keeps the current
	// parent (the edit dialog only sends name/description), "" promotes the
	// collection back to the top level, a URI nests it under that collection.
	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Parent      *string `json:"parent"`
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

	if body.Parent != nil {
		newParent := strings.TrimSpace(*body.Parent)
		if newParent == "" {
			parent = nil
		} else {
			uri := "at://" + did.String() + "/" + collectionNSID + "/" + rkey
			parsed, err := syntax.ParseATURI(newParent)
			if err != nil || parsed.Authority().String() != did.String() || parsed.Collection().String() != collectionNSID || newParent == uri {
				http.Error(w, "parent must be another of your own is.currents.feed.collection records", http.StatusBadRequest)
				return
			}
			// Enforce a single level from both ends: the new parent must be a root
			// collection, and this collection must not have sections of its own.
			if existing, err := s.Store.GetCollectionByURI(r.Context(), newParent, ""); err == nil && existing != nil && existing.ParentURI != "" {
				http.Error(w, "sub-collections cannot have sub-collections", http.StatusBadRequest)
				return
			}
			if subs, err := s.Store.GetSubcollectionURIs(r.Context(), uri, did.String()); err != nil {
				http.Error(w, "checking sub-collections", http.StatusInternalServerError)
				return
			} else if len(subs) > 0 {
				http.Error(w, "a collection with sections cannot become a section", http.StatusBadRequest)
				return
			}
			ref, err := resolveStrongRef(r.Context(), c, newParent)
			if err != nil {
				http.Error(w, fmt.Sprintf("resolving parent: %s", err), http.StatusBadRequest)
				return
			}
			parent = ref
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

// rateLimitMessage is returned (with HTTP 429) when the user's PDS rejects a
// blob upload with its own 429. PDSes cap blob uploads per IP, and the appview
// uploads on the user's behalf, so this can trip during busy periods even for
// light users. Phrased for a non-technical reader and kept identical to the
// frontend copy (see frontend/src/lib/rate-limit.ts).
const rateLimitMessage = "Your data server is temporarily limiting uploads. Please try again in a few minutes."

// pdsServiceDID derives a PDS's service DID from its base URL. atproto PDSes
// identify themselves as did:web:<hostname>, which is the audience a user
// service-auth token must be scoped to when calling that PDS.
func pdsServiceDID(hostURL string) (string, error) {
	u, err := url.Parse(hostURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("bad PDS host %q", hostURL)
	}
	return "did:web:" + u.Hostname(), nil
}

// mintUploadToken mints a short-lived service-auth JWT bound to uploadBlob on
// the session's PDS, returning it with the PDS base URL to send it to. Shared by
// the standalone token endpoint and the resave rate-limit fallback.
func mintUploadToken(ctx context.Context, c *atclient.APIClient) (token, pdsURL string, err error) {
	pdsURL = strings.TrimSuffix(c.Host, "/")
	aud, err := pdsServiceDID(pdsURL)
	if err != nil {
		return "", "", err
	}
	var out struct {
		Token string `json:"token"`
	}
	// Bind the token to uploadBlob only, valid for a couple of minutes — enough
	// for the browser to push the bytes, short enough to be low-risk in transit.
	err = c.Get(ctx, "com.atproto.server.getServiceAuth", map[string]any{
		"aud": aud,
		"lxm": "com.atproto.repo.uploadBlob",
		"exp": time.Now().Add(2 * time.Minute).Unix(),
	}, &out)
	if err != nil {
		return "", "", err
	}
	return out.Token, pdsURL, nil
}

// CreateUploadToken mints a short-lived service-auth JWT the browser uses to
// upload a blob straight to the user's PDS (com.atproto.repo.uploadBlob). The
// PDS rate-limits blob uploads per IP; when the appview uploads on the user's
// behalf every user shares the appview's single IP bucket. Uploading directly
// from the browser puts the request on the user's own IP, so each user gets
// their own bucket. The client then hands the returned blob ref back to
// POST /save (the `blob` field), where the record is created as usual — the
// PDS validates the ref against the uploading DID, so nothing else is trusted.
func (s *Server) CreateUploadToken(w http.ResponseWriter, r *http.Request) {
	c, _, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	token, pdsURL, err := mintUploadToken(r.Context(), c)
	if err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		// A 403 from getServiceAuth means the session predates the uploadBlob rpc:
		// scope. Surface it as 403 so the client can offer re-authorization (it falls
		// back to a server-side upload meanwhile); other errors are a real 500.
		var apiErr *atclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			http.Error(w, "missing uploadBlob scope; re-authorization required", http.StatusForbidden)
			return
		}
		http.Error(w, fmt.Sprintf("minting upload token: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token, "pdsUrl": pdsURL})
}

// blobRefFromSaveRecord fetches one of the viewer's own save records and returns
// its embedded image blob ref, so a resave of an already-held image can
// reference the blob directly instead of re-uploading it.
func blobRefFromSaveRecord(ctx context.Context, c *atclient.APIClient, did *syntax.DID, saveURI string) (any, error) {
	out, err := comatproto.RepoGetRecord(ctx, c, "", saveNSID, did.String(), rkeyFromURI(saveURI))
	if err != nil {
		return nil, err
	}
	if out.Value == nil {
		return nil, fmt.Errorf("save record has no value")
	}
	return extractImageBlobRef(*out.Value)
}

// extractImageBlobRef pulls the image blob ref out of a save record's value
// (value.content.image), the shape produced by buildImageContentRecordWithAttribution.
func extractImageBlobRef(value json.RawMessage) (any, error) {
	var rec struct {
		Content struct {
			Image json.RawMessage `json:"image"`
		} `json:"content"`
	}
	if err := json.Unmarshal(value, &rec); err != nil {
		return nil, err
	}
	if len(rec.Content.Image) == 0 {
		return nil, fmt.Errorf("save record has no image blob")
	}
	var blob any
	if err := json.Unmarshal(rec.Content.Image, &blob); err != nil {
		return nil, err
	}
	return blob, nil
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
	altText := strings.TrimSpace(r.PostFormValue("alt"))
	attrURL := strings.TrimSpace(r.PostFormValue("attribution_url"))
	attrLicense := strings.TrimSpace(r.PostFormValue("attribution_license"))
	attrCredit := strings.TrimSpace(r.PostFormValue("attribution_credit"))
	selfLabelVals := parseSelfLabels(r.PostFormValue("labels"))

	var blobAny any
	if preUploaded := strings.TrimSpace(r.PostFormValue("blob")); preUploaded != "" {
		// The client uploaded the blob straight to its own PDS with a service-auth
		// token (POST /api/blob/upload-token) and passed back the returned blob ref,
		// so uploadBlob counted against the user's own per-IP rate-limit bucket
		// rather than the appview's shared one. createRecord below references this
		// ref, and the PDS rejects a ref the uploading DID never actually holds, so
		// there is nothing here to re-validate.
		if err := json.Unmarshal([]byte(preUploaded), &blobAny); err != nil {
			http.Error(w, "invalid blob ref", http.StatusBadRequest)
			return
		}
	} else {
		// No pre-uploaded blob: the appview uploads on the user's behalf (from the
		// shared appview IP). Image bytes come from an uploaded file or a remote URL
		// (paste-from-URL, which must be fetched server-side).
		var imageBytes []byte
		var contentType string
		if file, header, fileErr := r.FormFile("image"); fileErr == nil {
			defer file.Close()
			contentType = header.Header.Get("Content-Type")
			imageBytes, err = io.ReadAll(file)
			if err != nil {
				http.Error(w, "reading image file", http.StatusInternalServerError)
				return
			}
		} else if imageURL := strings.TrimSpace(r.PostFormValue("imageUrl")); imageURL != "" {
			imageBytes, contentType, err = fetchRemoteImage(r.Context(), imageURL)
			if err != nil {
				http.Error(w, "could not fetch image from URL", http.StatusBadGateway)
				return
			}
		} else {
			http.Error(w, "image, imageUrl, or blob is required", http.StatusBadRequest)
			return
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		// uploadBlob stores this verbatim as the blob's mimeType, and the PDS checks it
		// against the granted `blob:image/*` scope — so a wildcard (`image/*`, what Android
		// share intents usually carry) or a generic type is a 403, not a mislabelled blob.
		// Sniff the bytes whenever what we were handed isn't a concrete image type.
		if !strings.HasPrefix(contentType, "image/") || strings.Contains(contentType, "*") {
			if sniffed := http.DetectContentType(imageBytes); strings.HasPrefix(sniffed, "image/") {
				contentType = sniffed
			}
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
			if isRateLimited(err) {
				http.Error(w, rateLimitMessage, http.StatusTooManyRequests)
				return
			}
			http.Error(w, fmt.Sprintf("uploading image: %s", err), http.StatusInternalServerError)
			return
		}
		blobJSON, _ := json.Marshal(uploadOut.Blob)
		json.Unmarshal(blobJSON, &blobAny)
	}

	record := map[string]any{
		"$type":     saveNSID,
		"content":   buildImageContentRecordWithAttribution(blobAny, saveAttributionFromFields(attrURL, attrLicense, attrCredit), altText),
		"createdAt": syntax.DatetimeNow().String(),
	}
	// A save with no collection is "unsorted" — it lives on the user's profile only.
	if collectionURI != "" {
		collectionStrongRef, err := resolveStrongRef(r.Context(), c, collectionURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("resolving collection: %s", err), http.StatusBadRequest)
			return
		}
		record["collection"] = collectionStrongRef
	}
	if labels := buildSelfLabelsRecord(selfLabelVals); labels != nil {
		record["labels"] = labels
	}
	if safe := safeOriginURL(url); safe != "" {
		record["originUrl"] = safe
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

	// Use form value if provided, otherwise preserve existing. Validate the
	// scheme either way so a non-http(s) originUrl can never reach the client.
	if safe := safeOriginURL(url); safe != "" {
		record["originUrl"] = safe
	} else if safe := safeOriginURL(existingVal.OriginURL); safe != "" {
		record["originUrl"] = safe
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
	return s.putSaveContentForRkey(ctx, c, did, rkey, func(contentRaw json.RawMessage) (any, error) {
		return buildSaveContentWithAttribution(contentRaw, attribution, true)
	})
}

// putAltForRkey rebuilds a single save record with the given alt text applied to
// its image content, preserving all other fields.
func (s *Server) putAltForRkey(ctx context.Context, c *atclient.APIClient, did *syntax.DID, rkey string, alt string) error {
	return s.putSaveContentForRkey(ctx, c, did, rkey, func(contentRaw json.RawMessage) (any, error) {
		return buildSaveContentWithAlt(contentRaw, alt)
	})
}

// putSaveContentForRkey reads one of the viewer's save records, hands its content
// to rebuild, and writes the record back with every other field preserved —
// RepoPutRecord replaces the whole record, so anything not copied across is lost.
func (s *Server) putSaveContentForRkey(ctx context.Context, c *atclient.APIClient, did *syntax.DID, rkey string, rebuild func(json.RawMessage) (any, error)) error {
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

	contentAny, err := rebuild(existingVal.Content)
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

// UpdateSaveAlt sets the alt text on one save the viewer owns. Scoped to a single
// rkey, unlike UpdateSaveAttribution, which fans out over every save of the blob:
// alt is edited from a panel showing one save, so the edit stays where it was made
// (the same image saved twice can carry different alt text). Allowed on resaves —
// the record is the viewer's own copy of the image.
func (s *Server) UpdateSaveAlt(w http.ResponseWriter, r *http.Request) {
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
	// Empty is meaningful: it clears the alt text.
	alt := strings.TrimSpace(r.PostFormValue("alt"))

	if err := s.putAltForRkey(r.Context(), c, did, rkey, alt); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("updating alt: %s", err), http.StatusInternalServerError)
		return
	}

	uri := "at://" + did.String() + "/" + saveNSID + "/" + rkey
	slog.Info("updated save alt", "uri", uri)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"uri": uri, "alt": alt})
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

func (s *Server) CreateFavourite(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var body struct {
		SubjectURI string `json:"subjectUri"`
		SubjectCID string `json:"subjectCid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SubjectURI == "" || body.SubjectCID == "" {
		http.Error(w, "subjectUri and subjectCid are required", http.StatusBadRequest)
		return
	}
	subjectURI, err := syntax.ParseATURI(body.SubjectURI)
	if err != nil {
		http.Error(w, "invalid subject URI", http.StatusBadRequest)
		return
	}
	// You favourite other people's collections, not your own.
	if subjectURI.Authority().String() == did.String() {
		http.Error(w, "cannot favourite your own collection", http.StatusBadRequest)
		return
	}

	out, err := comatproto.RepoCreateRecord(r.Context(), c, &comatproto.RepoCreateRecord_Input{
		Collection: favouriteNSID,
		Repo:       did.String(),
		Record: map[string]any{
			"$type": favouriteNSID,
			"subject": map[string]any{
				"uri": body.SubjectURI,
				"cid": body.SubjectCID,
			},
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
		slog.Error("creating favourite", "err", err, "subject", body.SubjectURI, "viewer", did.String())
		http.Error(w, fmt.Sprintf("creating favourite: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("created favourite", "uri", out.Uri)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"uri":%q}`, out.Uri)
}

func (s *Server) DeleteFavourite(w http.ResponseWriter, r *http.Request) {
	c, did, err := s.apiClientFromSession(r)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	rkey := r.PathValue("rkey")

	if err := c.Post(r.Context(), "com.atproto.repo.deleteRecord", map[string]any{
		"repo":       did.String(),
		"collection": favouriteNSID,
		"rkey":       rkey,
	}, nil); err != nil {
		if s.handleSessionError(err, w, r) {
			return
		}
		http.Error(w, fmt.Sprintf("deleting favourite: %s", err), http.StatusInternalServerError)
		return
	}

	slog.Info("deleted favourite", "rkey", rkey)
	w.WriteHeader(http.StatusNoContent)
}

// respondResaveRateLimited answers a resave that tripped the PDS's shared per-IP
// blob rate limit with everything the client needs to finish from its own IP: a
// service-auth token, the PDS URL, and the image bytes the appview already
// fetched (base64, so the client uploads the exact original bytes and the blob
// CID stays identical). The client uploads, then retries POST /resave with the
// resulting blob ref. If the token can't be minted it degrades to a plain 429.
func (s *Server) respondResaveRateLimited(w http.ResponseWriter, r *http.Request, c *atclient.APIClient, imageBytes []byte, contentType string) {
	token, pdsURL, err := mintUploadToken(r.Context(), c)
	if err != nil {
		// A 403 means the session lacks the uploadBlob rpc: scope, so the client-side
		// rescue can't run — but reconnecting would unlock it. Signal the client to
		// nudge re-authorization instead of just reporting a dead-end rate limit.
		var apiErr *atclient.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"rateLimited": true, "needsReauth": true})
			return
		}
		http.Error(w, rateLimitMessage, http.StatusTooManyRequests)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]any{
		"rateLimited": true,
		"token":       token,
		"pdsUrl":      pdsURL,
		"image":       base64.StdEncoding.EncodeToString(imageBytes),
		"contentType": contentType,
	})
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
		// Blob, when present, is a blob ref the client already uploaded to its own
		// PDS after a rate-limit fallback — the second leg of respondResaveRateLimited.
		Blob json.RawMessage `json:"blob"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.SaveURI == "" {
		http.Error(w, "saveUri is required", http.StatusBadRequest)
		return
	}

	// Look up the original save to get blob info, source URL, and attribution
	var authorDID, blobCID, origOriginURL, origAlt string
	var origAttrURL, origAttrLicense, origAttrCredit string
	err = s.Store.pool.QueryRow(r.Context(),
		`SELECT author_did, pds_blob_cid, COALESCE(origin_url, ''), COALESCE(alt_text, ''), COALESCE(attribution_url, ''), COALESCE(attribution_license, ''), COALESCE(attribution_credit, '') FROM save WHERE uri = $1`, body.SaveURI,
	).Scan(&authorDID, &blobCID, &origOriginURL, &origAlt, &origAttrURL, &origAttrLicense, &origAttrCredit)
	if err != nil {
		http.Error(w, "save not found", http.StatusNotFound)
		return
	}

	// Acquire a blob ref for the viewer's repo, cheapest path first:
	//  1. the client already uploaded it (rate-limit fallback's second call) — use its ref
	//  2. the viewer already holds this blob — reuse the ref from one of their saves, no upload
	//  3. otherwise fetch the source bytes and upload; on the shared-bucket 429, hand the bytes
	//     plus a service-auth token to the client so it can upload from its own IP and retry
	var blobAny any
	if len(body.Blob) > 0 {
		if err := json.Unmarshal(body.Blob, &blobAny); err != nil {
			http.Error(w, "invalid blob ref", http.StatusBadRequest)
			return
		}
	}
	if blobAny == nil {
		if heldURI, err := s.Store.ViewerBlobSaveURI(r.Context(), did.String(), blobCID); err == nil && heldURI != "" {
			if ref, err := blobRefFromSaveRecord(r.Context(), c, did, heldURI); err == nil {
				blobAny = ref
			} else {
				slog.Warn("resave: reusing held blob ref failed, will re-upload", "err", err)
			}
		}
	}
	if blobAny == nil {
		imageBytes, contentType, err := fetchBlobFromPDS(r.Context(), s.Store, s.Dir, authorDID, blobCID)
		if err != nil {
			slog.Error("fetching blob for resave", "err", err)
			http.Error(w, "could not fetch image", http.StatusBadGateway)
			return
		}
		var uploadOut struct {
			Blob lexutil.LexBlob `json:"blob"`
		}
		if err := c.LexDo(r.Context(), "POST", contentType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(imageBytes), &uploadOut); err != nil {
			if s.handleSessionError(err, w, r) {
				return
			}
			if isRateLimited(err) {
				s.respondResaveRateLimited(w, r, c, imageBytes, contentType)
				return
			}
			http.Error(w, fmt.Sprintf("uploading image: %s", err), http.StatusInternalServerError)
			return
		}
		blobJSON, _ := json.Marshal(uploadOut.Blob)
		json.Unmarshal(blobJSON, &blobAny)
	}

	// Resolve strong refs
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

	// If the viewer already has this image saved (same blob CID), inherit their
	// original save time so copying/moving it between collections doesn't reorder
	// it to the top of time-sorted views. Otherwise it's genuinely new to them.
	createdAt := syntax.DatetimeNow().String()
	if earliest, err := s.Store.EarliestSaveCreatedAt(r.Context(), did.String(), blobCID); err == nil && earliest != nil {
		createdAt = earliest.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	record := map[string]any{
		"$type":     saveNSID,
		"content":   buildImageContentRecordWithAttribution(blobAny, resolvedAttribution, origAlt),
		"resaveOf":  resaveRef,
		"createdAt": createdAt,
	}
	// No collection → an "unsorted" resave that lives on the viewer's profile only.
	if body.CollectionURI != "" {
		collectionStrongRef, err := resolveStrongRef(r.Context(), c, body.CollectionURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("resolving collection: %s", err), http.StatusBadRequest)
			return
		}
		record["collection"] = collectionStrongRef
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
