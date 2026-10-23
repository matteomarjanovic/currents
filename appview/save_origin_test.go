package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/gorilla/sessions"
)

const originTestURI = "at://did:plc:ewvi7nxzyoun6zhxrhs64oiz/app.bsky.feed.post/3k4duaz5vfs2b"
const originTestCID = "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"

func TestParseOriginAtprotoRef(t *testing.T) {
	for _, tt := range []struct {
		name    string
		raw     string
		wantRef bool
		wantErr bool
	}{
		{"absent", "", false, false},
		{"clear", " null ", false, false},
		{"valid", fmt.Sprintf(`{"uri":%q,"cid":%q}`, originTestURI, originTestCID), true, false},
		{"trim", fmt.Sprintf(`{"uri":" %s ","cid":" %s "}`, originTestURI, originTestCID), true, false},
		{"empty object", `{}`, false, true},
		{"malformed", `{`, false, true},
		{"string", `"at://example.com"`, false, true},
		{"array", `[]`, false, true},
		{"missing cid", fmt.Sprintf(`{"uri":%q}`, originTestURI), false, true},
		{"invalid cid", fmt.Sprintf(`{"uri":%q,"cid":"notarealcid"}`, originTestURI), false, true},
		{"web link", fmt.Sprintf(`{"uri":"https://bsky.app/profile/test/post/key","cid":%q}`, originTestCID), false, true},
		{"handle", fmt.Sprintf(`{"uri":"at://alice.test/app.bsky.feed.post/key","cid":%q}`, originTestCID), false, true},
		{"repo only", fmt.Sprintf(`{"uri":"at://did:plc:test","cid":%q}`, originTestCID), false, true},
		{"collection only", fmt.Sprintf(`{"uri":"at://did:plc:test/app.bsky.feed.post","cid":%q}`, originTestCID), false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseOriginAtprotoRef([]byte(tt.raw))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ref = %#v, err = %v; want error %v", ref, err, tt.wantErr)
			}
			if (ref != nil) != tt.wantRef {
				t.Fatalf("ref = %#v; want present %v", ref, tt.wantRef)
			}
			if ref != nil && (ref.URI != originTestURI || ref.CID != originTestCID) {
				t.Fatalf("unexpected reference: %#v", ref)
			}
		})
	}
}

func TestSaveEditsPreserveOriginAtprotoRef(t *testing.T) {
	for _, edit := range []string{"alt", "attribution", "labels", "replace origin", "clear origin"} {
		t.Run(edit, func(t *testing.T) {
			did := syntax.DID("did:plc:owner")
			original := map[string]any{
				"$type": saveNSID,
				"content": map[string]any{
					"$type": saveContentImageNSID,
					"image": map[string]any{"$type": "blob", "ref": map[string]any{"$link": originTestCID}, "mimeType": "image/jpeg"},
					"alt":   "Original description",
				},
				"createdAt":        "2026-01-01T00:00:00Z",
				"collection":       map[string]any{"uri": "at://did:plc:owner/" + collectionNSID + "/one", "cid": originTestCID},
				"originUrl":        "https://bsky.app/profile/test/post/key",
				"text":             "My note",
				"originAtprotoRef": map[string]any{"uri": originTestURI, "cid": originTestCID},
				"labels":           map[string]any{"$type": "com.atproto.label.defs#selfLabels", "values": []any{map[string]any{"val": "nudity"}}},
			}
			if edit == "replace origin" || edit == "clear origin" {
				original["resaveOf"] = map[string]any{"uri": "at://did:plc:other/" + saveNSID + "/source", "cid": originTestCID}
				original["futureField"] = "preserve unknown fields too"
			}
			var written map[string]any
			pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/xrpc/com.atproto.repo.getRecord":
					if r.URL.Query().Get("repo") != did.String() || r.URL.Query().Get("collection") != saveNSID || r.URL.Query().Get("rkey") != "one" {
						t.Error("edit did not target the viewer's save")
					}
					json.NewEncoder(w).Encode(map[string]any{"uri": "at://" + did.String() + "/" + saveNSID + "/one", "cid": originTestCID, "value": original})
				case "/xrpc/com.atproto.repo.putRecord":
					var body struct {
						Repo       string         `json:"repo"`
						SwapRecord string         `json:"swapRecord"`
						Record     map[string]any `json:"record"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.SwapRecord != originTestCID || body.Repo != did.String() {
						t.Error("write must target the owner and protect against concurrent edits")
					}
					written = body.Record
					json.NewEncoder(w).Encode(map[string]any{"uri": "at://" + did.String() + "/" + saveNSID + "/one", "cid": originTestCID})
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer pds.Close()
			client := atclient.NewAPIClient(pds.URL)
			s := &Server{}
			var err error
			switch edit {
			case "alt":
				err = s.putAltForRkey(context.Background(), client, &did, "one", "New alt")
			case "attribution":
				err = s.putAttributionForRkey(context.Background(), client, &did, "one", &saveAttribution{Credit: "An artist"})
			case "labels":
				_, _, _, err = applyLabelsToOwnedSave(context.Background(), client, &did, "one", []string{"sexual"})
			case "replace origin":
				err = putOriginAtprotoRefForRkey(context.Background(), client, &did, "one", &strongRef{URI: originTestURI + "other", CID: originTestCID})
			case "clear origin":
				err = putOriginAtprotoRefForRkey(context.Background(), client, &did, "one", nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if written == nil {
				t.Fatal("no record was written")
			}
			if edit == "replace origin" || edit == "clear origin" {
				if edit == "clear origin" {
					if _, exists := written["originAtprotoRef"]; exists {
						t.Fatal("cleared reference must be omitted")
					}
				} else if !reflect.DeepEqual(written["originAtprotoRef"], map[string]any{"uri": originTestURI + "other", "cid": originTestCID}) {
					t.Fatalf("reference was not replaced: %#v", written["originAtprotoRef"])
				}
				delete(written, "originAtprotoRef")
				delete(original, "originAtprotoRef")
				if !reflect.DeepEqual(written, original) {
					t.Fatalf("origin edit changed other fields: %#v", written)
				}
			} else if !reflect.DeepEqual(written["originAtprotoRef"], original["originAtprotoRef"]) {
				t.Fatalf("%s edit lost provenance: %#v", edit, written)
			}
		})
	}
}

func TestBuildSaveViewOriginAtprotoRef(t *testing.T) {
	for _, complete := range []bool{false, true} {
		row := SaveRow{URI: "at://did:plc:owner/" + saveNSID + "/one", OriginAtprotoURI: originTestURI}
		if complete {
			row.OriginAtprotoCID = originTestCID
		}
		view := buildSaveView(row, profileView{}, false, "https://cdn.example.com")
		if complete {
			if view.OriginAtprotoRef == nil || *view.OriginAtprotoRef != (strongRef{URI: originTestURI, CID: originTestCID}) {
				t.Fatalf("unexpected origin reference: %#v", view.OriginAtprotoRef)
			}
		} else {
			raw, _ := json.Marshal(view)
			var fields map[string]any
			json.Unmarshal(raw, &fields)
			if _, ok := fields["originAtprotoRef"]; ok {
				t.Fatal("incomplete origin reference must be omitted")
			}
		}
	}
}

// Exercise the extension's actual multipart contract and the authenticated edit
// and resave handlers against a mock PDS and the isolated test database.
func TestSaveOriginAtprotoRefAPI(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const owner = "did:plc:owner"
	const sourceURI = "at://" + owner + "/is.currents.feed.save/source"
	var written map[string]any
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/xrpc/com.atproto.repo.getRecord":
			json.NewEncoder(w).Encode(map[string]any{"uri": sourceURI, "cid": originTestCID, "value": written})
		case "/xrpc/com.atproto.repo.createRecord", "/xrpc/com.atproto.repo.putRecord":
			var body struct {
				Repo   string         `json:"repo"`
				Record map[string]any `json:"record"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Repo != owner {
				t.Errorf("wrong repository: %s", body.Repo)
			}
			written = body.Record
			json.NewEncoder(w).Encode(map[string]any{"uri": sourceURI, "cid": originTestCID})
		default:
			t.Errorf("unexpected PDS call: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer pds.Close()
	stubBlobTransport(t, http.DefaultTransport) // this API regression uses a local fixture PDS
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatal(err)
	}
	sessionsStore := oauth.NewMemStore()
	if err := sessionsStore.SaveSession(ctx, oauth.ClientSessionData{
		AccountDID: syntax.DID(owner), SessionID: "test", HostURL: pds.URL,
		AccessToken: "test-token", DPoPPrivateKeyMultibase: key.Multibase(),
	}); err != nil {
		t.Fatal(err)
	}
	config := oauth.NewPublicConfig("https://currents.test/oauth-client-metadata.json", "https://currents.test/oauth/callback", []string{"atproto"})
	s := &Server{Store: store, CookieStore: sessions.NewCookieStore([]byte("test-secret")), OAuth: oauth.NewClientApp(&config, sessionsStore)}
	token, err := s.encodeSessionToken(owner, "test", "owner.test")
	if err != nil {
		t.Fatal(err)
	}
	ref := map[string]any{"uri": originTestURI, "cid": originTestCID}
	refJSON, _ := json.Marshal(ref)
	blob := fmt.Sprintf(`{"$type":"blob","ref":{"$link":%q},"mimeType":"image/jpeg","size":1}`, originTestCID)
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	writer.WriteField("blob", blob)
	writer.WriteField("originAtprotoRef", string(refJSON))
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/save", &form)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res := httptest.NewRecorder()
	s.CreateSave(res, req)
	if res.Code != 200 || !reflect.DeepEqual(written["originAtprotoRef"], ref) {
		t.Fatalf("create = %d %s, record = %#v", res.Code, res.Body, written)
	}
	rows, err := store.GetSavesByURIs(ctx, []string{sourceURI}, owner)
	if err != nil || len(rows) != 0 {
		t.Fatalf("handler must leave indexing to ingestion: %#v, %v", rows, err)
	}
	if err := store.UpsertSave(ctx, UpsertSaveParams{
		URI: sourceURI, AuthorDID: owner, PdsBlobCID: originTestCID,
		ContentNSID: saveContentImageNSID, OriginAtprotoURI: originTestURI, OriginAtprotoCID: originTestCID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser(ctx, UserRecord{DID: owner, Handle: "owner.test", PDSEndpoint: pds.URL}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/resave", strings.NewReader(fmt.Sprintf(`{"saveUri":%q,"blob":%s}`, sourceURI, blob)))
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	s.CreateResave(res, req)
	if res.Code != 200 || !reflect.DeepEqual(written["originAtprotoRef"], ref) || written["resaveOf"] == nil {
		t.Fatalf("resave = %d %s, record = %#v", res.Code, res.Body, written)
	}
	for _, tt := range []struct {
		body       string
		wantStatus int
	}{
		{`{}`, 400},
		{`{"originAtprotoRef":{"uri":"https://bsky.app","cid":"bad"}}`, 400},
		{`{"originAtprotoRef":` + string(refJSON) + `}`, 200},
		{`{"originAtprotoRef":null}`, 200},
	} {
		req = httptest.NewRequest(http.MethodPut, "/api/save/source/origin-atproto-ref", strings.NewReader(tt.body))
		req.SetPathValue("id", "source")
		req.Header.Set("Authorization", "Bearer "+token)
		res = httptest.NewRecorder()
		s.UpdateSaveOriginAtprotoRef(res, req)
		if res.Code != tt.wantStatus {
			t.Fatalf("edit %s = %d %s", tt.body, res.Code, res.Body)
		}
	}
	if _, ok := written["originAtprotoRef"]; ok {
		t.Fatal("clear must omit the field from the PDS record")
	}
	if written["resaveOf"] == nil {
		t.Fatal("origin edit must preserve resave provenance")
	}
	req = httptest.NewRequest(http.MethodPut, "/api/save/source/origin-atproto-ref", strings.NewReader(`{"originAtprotoRef":null}`))
	res = httptest.NewRecorder()
	s.UpdateSaveOriginAtprotoRef(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated edit = %d, want 401", res.Code)
	}
}
