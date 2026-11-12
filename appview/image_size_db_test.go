package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/gorilla/sessions"
)

func TestSaveAndResaveEnforceBlobSizeBeforeRecordWrite(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const owner = "did:plc:owner"
	const sourceURI = "at://" + owner + "/is.currents.feed.save/source"
	writes := 0
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/xrpc/com.atproto.repo.getRecord":
			json.NewEncoder(w).Encode(map[string]any{"uri": sourceURI, "cid": originTestCID, "value": map[string]any{}})
		case "/xrpc/com.atproto.repo.createRecord":
			writes++
			json.NewEncoder(w).Encode(map[string]any{"uri": sourceURI, "cid": originTestCID})
		default:
			t.Errorf("unexpected PDS call: %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer pds.Close()
	stubBlobTransport(t, http.DefaultTransport)
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	mem := oauth.NewMemStore()
	if err := mem.SaveSession(ctx, oauth.ClientSessionData{AccountDID: syntax.DID(owner), SessionID: "test", HostURL: pds.URL,
		AccessToken: "test", DPoPPrivateKeyMultibase: key.Multibase()}); err != nil {
		t.Fatal(err)
	}
	config := oauth.NewPublicConfig("https://currents.test/client.json", "https://currents.test/callback", []string{"atproto"})
	s := &Server{Store: store, OAuth: oauth.NewClientApp(&config, mem), CookieStore: sessions.NewCookieStore([]byte("test"))}
	token, err := s.encodeSessionToken(owner, "test", "owner.test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSave(ctx, UpsertSaveParams{URI: sourceURI, AuthorDID: owner, PdsBlobCID: originTestCID, ContentNSID: saveContentImageNSID}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser(ctx, UserRecord{DID: owner, Handle: "owner.test", PDSEndpoint: pds.URL}); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"save", "resave"} {
		for _, tt := range []struct{ size, status int }{{1, 200}, {maxBlobSize, 200}, {maxBlobSize + 1, 413}, {29_880_100, 413}, {0, 400}, {-1, 400}} {
			t.Run(fmt.Sprintf("%s/%d", endpoint, tt.size), func(t *testing.T) {
				blob := fmt.Sprintf(`{"$type":"blob","ref":{"$link":%q},"mimeType":"image/gif","size":%d}`, originTestCID, tt.size)
				var req *http.Request
				if endpoint == "save" {
					var body bytes.Buffer
					form := multipart.NewWriter(&body)
					form.WriteField("blob", blob)
					form.Close()
					req = httptest.NewRequest("POST", "/api/save", &body)
					req.Header.Set("Content-Type", form.FormDataContentType())
				} else {
					req = httptest.NewRequest("POST", "/api/resave", strings.NewReader(fmt.Sprintf(`{"saveUri":%q,"blob":%s}`, sourceURI, blob)))
				}
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Accept", "application/json")
				res := httptest.NewRecorder()
				before := writes
				if endpoint == "save" {
					s.CreateSave(res, req)
				} else {
					s.CreateResave(res, req)
				}
				if res.Code != tt.status {
					t.Fatalf("got %d: %s", res.Code, res.Body)
				}
				wantWrites := 0
				if tt.status == 200 {
					wantWrites = 1
				}
				if writes-before != wantWrites {
					t.Fatalf("created %d records; want %d", writes-before, wantWrites)
				}
			})
		}
	}

	// Raw oversized GIFs must fail before uploadBlob, even though the older
	// still-image path could decode their first frame and silently JPEG it.
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	image, err := form.CreateFormFile("image", "large.gif")
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, maxBlobSize+1)
	copy(data, "GIF89a")
	image.Write(data)
	form.Close()
	req := httptest.NewRequest("POST", "/api/save", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	s.CreateSave(res, req)
	if res.Code != 413 || strings.TrimSpace(res.Body.String()) != errImageTooLarge.Error() {
		t.Fatalf("raw GIF response=%d %s", res.Code, res.Body)
	}
}
