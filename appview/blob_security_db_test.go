package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func TestImageProxySafeResponses(t *testing.T) {
	heic := []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00heicmif1")
	for _, test := range []struct {
		name      string
		blob      []byte
		declared  string
		transcode []byte
		status    int
	}{
		{"normal PNG", testBlobPNG, "image/png", nil, 200},
		{"valid image mislabeled as HTML", testBlobPNG, "text/html", nil, 200},
		{"HTML mislabeled as image", []byte("<!doctype html><script>alert(1)</script>"), "image/png", nil, 502},
		{"active SVG", []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`), "image/svg+xml", nil, 502},
		{"HEIC transcode", heic, "image/heic", testBlobPNG, 200},
		{"invalid transcode", heic, "image/heic", []byte("<html>invalid transcode</html>"), 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			const did = "did:plc:proxytest"
			if _, err := store.pool.Exec(context.Background(), `INSERT INTO "user" (did,created_at,pds_endpoint) VALUES ($1,now(),'https://pds.example')`, did); err != nil {
				t.Fatal(err)
			}
			dir := identity.NewMockDirectory()
			dir.Insert(identity.Identity{DID: syntax.DID(did), Services: map[string]identity.ServiceEndpoint{"atproto_pds": {URL: "https://pds.example"}}})
			stubBlobTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
				return blobResponse(r, io.NopCloser(bytes.NewReader(test.blob)), int64(len(test.blob)), test.declared), nil
			}))
			inference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/transcode/image" {
					t.Errorf("unexpected inference request %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "image/jpeg") // proxy must inspect this response too
				w.Write(test.transcode)
			}))
			defer inference.Close()
			s := &Server{Store: store, Dir: dir, Inference: NewInferenceClient(inference.URL)}
			req := httptest.NewRequest(http.MethodGet, "https://currents.is/img/"+did+"/cid", nil)
			req.SetPathValue("did", did)
			req.SetPathValue("cid", "cid")
			response := httptest.NewRecorder()
			s.ImageProxy(response, req)
			if response.Code != test.status {
				t.Fatalf("got %d: %s; want %d", response.Code, response.Body.String(), test.status)
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Content-Security-Policy") != "default-src 'none'; sandbox" {
				t.Fatalf("missing browser protections: %v", response.Header())
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "*" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Fatal("public image CORS changed")
			}
			if test.status == 200 {
				if response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(response.Body.Bytes(), testBlobPNG) {
					t.Fatalf("unsafe MIME/body: %v", response.Header())
				}
				if !strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
					t.Fatal("valid images lost immutable caching")
				}
			} else {
				if bytes.Contains(response.Body.Bytes(), []byte("<script>")) || strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
					t.Fatal("unsafe response or cacheable error")
				}
			}
		})
	}
}
