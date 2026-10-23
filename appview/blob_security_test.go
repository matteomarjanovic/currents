package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
)

// Valid tiny PNG; using actual image bytes also exercises the byte-derived MIME.
var testBlobPNG = func() []byte {
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aX1sAAAAASUVORK5CYII=")
	if err != nil {
		panic(err)
	}
	return data
}()

type blobRoundTripper func(*http.Request) (*http.Response, error)

func (f blobRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubBlobTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	original := blobHTTPClient
	client := *original
	client.Transport = transport
	blobHTTPClient = &client
	t.Cleanup(func() { blobHTTPClient = original })
}

func blobResponse(r *http.Request, body io.ReadCloser, size int64, contentType string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: body, ContentLength: size, Request: r}
}

func TestPDSFetchBlocksPrivateAddresses(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	t.Setenv("HTTP_PROXY", server.URL)
	t.Setenv("HTTPS_PROXY", server.URL)
	if blobHTTPClient.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("PDS fetches must not use environment proxies")
	}
	for _, endpoint := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1), "http://10.0.0.1", "http://172.16.8.2:8000", "http://192.168.1.1", "http://169.254.169.254", "http://169.254.42.42", "http://[::1]", "http://[::ffff:127.0.0.1]", "http://[fd00::1]"} {
		t.Run(endpoint, func(t *testing.T) {
			_, _, err := getBlobFromEndpoint(context.Background(), endpoint, "did:plc:test", "cid")
			if !errors.Is(err, errBlockedAddress) {
				t.Fatalf("got %v, want address blocked", err)
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("private server/proxy received %d requests", hits.Load())
	}
}

func TestPDSFetchRedirects(t *testing.T) {
	originalTransport := blobHTTPClient.Transport
	for _, test := range []struct {
		name, location string
		wantBlocked    bool
	}{
		{"private", "http://127.0.0.1:8000/private", true},
		{"metadata", "http://169.254.169.254/private", true},
		{"non HTTP scheme", "file:///etc/passwd", false},
		{"redirect loop", "https://pds.example/again", false},
		{"public image CDN", "https://cdn.example/image", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests int
			stubBlobTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
				requests++
				if requests > 6 {
					t.Fatal("redirect cap was bypassed")
				}
				if r.URL.Host == "cdn.example" {
					return blobResponse(r, io.NopCloser(bytes.NewReader(testBlobPNG)), int64(len(testBlobPNG)), "image/png"), nil
				}
				if r.URL.Host != "pds.example" {
					return originalTransport.RoundTrip(r)
				}
				return &http.Response{StatusCode: 302, Header: http.Header{"Location": {test.location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			}))
			data, mime, err := getBlobFromEndpoint(context.Background(), "https://pds.example", "did:plc:test", "cid")
			if test.name == "public image CDN" {
				if err != nil || mime != "image/png" || !bytes.Equal(data, testBlobPNG) {
					t.Fatalf("public redirect failed: %q %v", mime, err)
				}
			} else if err == nil {
				t.Fatal("unsafe/looping redirect accepted")
			} else if test.wantBlocked && !errors.Is(err, errBlockedAddress) {
				t.Fatalf("expected blocked address: %v", err)
			}
		})
	}
}

func TestPDSFetchValidatesURLAndEncodesParameters(t *testing.T) {
	var calls int
	stubBlobTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/prefix/xrpc/com.atproto.sync.getBlob" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("did") != "did:plc:test&admin=true" || r.URL.Query().Get("cid") != "cid&other=1" || len(r.URL.Query()) != 2 {
			t.Errorf("query injection: %s", r.URL.RawQuery)
		}
		if r.Header.Get("User-Agent") != blobUserAgent {
			t.Error("missing blob user agent")
		}
		return blobResponse(r, io.NopCloser(bytes.NewReader(testBlobPNG)), int64(len(testBlobPNG)), "text/html"), nil
	}))
	for _, endpoint := range []string{"file:///etc", "ftp://pds.example", "https://user:pass@pds.example", "https://pds.example?injected=1", "https://pds.example#fragment", "/relative"} {
		if _, _, err := getBlobFromEndpoint(context.Background(), endpoint, "did:plc:test", "cid"); err == nil {
			t.Errorf("accepted %q", endpoint)
		}
	}
	data, mime, err := getBlobFromEndpoint(context.Background(), "https://pds.example/prefix/", "did:plc:test&admin=true", "cid&other=1")
	if err != nil || mime != "image/png" || !bytes.Equal(data, testBlobPNG) {
		t.Fatalf("valid image mislabeled as HTML: %q %v", mime, err)
	}
	if calls != 1 {
		t.Fatalf("invalid endpoints made requests: %d", calls)
	}
}

type countedBlobBody struct {
	reader io.Reader
	read   int
	closed bool
}

func (b *countedBlobBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.read += n
	return n, err
}
func (b *countedBlobBody) Close() error { b.closed = true; return nil }

func TestPDSFetchBoundsDownloads(t *testing.T) {
	for _, test := range []struct {
		name      string
		length    int64
		bodySize  int
		wantError bool
		wantRead  int
	}{
		{"declared oversized", maxFetchedBlobSize + 1, 1, true, 0},
		{"chunked oversized", -1, maxFetchedBlobSize + 100, true, maxFetchedBlobSize + 1},
		{"lying content length", 1, maxFetchedBlobSize + 100, true, maxFetchedBlobSize + 1},
		{"exact limit", maxFetchedBlobSize, maxFetchedBlobSize, false, maxFetchedBlobSize},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := make([]byte, test.bodySize)
			copy(payload, testBlobPNG)
			body := &countedBlobBody{reader: bytes.NewReader(payload)}
			stubBlobTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
				return blobResponse(r, body, test.length, "image/png"), nil
			}))
			_, _, err := getBlobFromEndpoint(context.Background(), "https://pds.example", "did:plc:test", "cid")
			if test.wantError && !errors.Is(err, errInvalidImage) || !test.wantError && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if body.read != test.wantRead || !body.closed {
				t.Fatalf("read %d closed=%v, want %d and closed", body.read, body.closed, test.wantRead)
			}
		})
	}
}

func TestPDSFetchRejectsActiveContent(t *testing.T) {
	for _, data := range [][]byte{[]byte("<!doctype html><script>alert(1)</script>"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`), nil, []byte("not an image")} {
		t.Run(fmt.Sprintf("%q", data), func(t *testing.T) {
			stubBlobTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
				return blobResponse(r, io.NopCloser(bytes.NewReader(data)), int64(len(data)), "image/png"), nil
			}))
			if _, _, err := getBlobFromEndpoint(context.Background(), "https://pds.example", "did:plc:test", "cid"); err == nil {
				t.Fatal("active/invalid content accepted under forged image MIME")
			}
		})
	}
}

func TestBlobImageTypePreservesRasterFormats(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		mime string
	}{
		{"PNG", testBlobPNG, "image/png"},
		{"JPEG", []byte{0xff, 0xd8, 0xff, 0xe0}, "image/jpeg"},
		{"GIF", []byte("GIF89a"), "image/gif"},
		{"WebP", append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 8)...), "image/webp"},
		{"BMP", []byte("BM\x00\x00"), "image/bmp"},
		{"ICO", []byte("\x00\x00\x01\x00"), "image/x-icon"},
		{"TIFF LE", []byte("II\x2a\x00"), "image/tiff"},
		{"TIFF BE", []byte("MM\x00\x2a"), "image/tiff"},
		{"HEIC", []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00heicmif1"), "image/heic"},
		{"HEIF", []byte("\x00\x00\x00\x18ftypmif1\x00\x00\x00\x00mif1heic"), "image/heif"},
		{"AVIF", []byte("\x00\x00\x00\x18ftypavif\x00\x00\x00\x00avifmif1"), "image/avif"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mime, err := blobImageType(test.data)
			if err != nil || mime != test.mime {
				t.Fatalf("got %q %v, want %q", mime, err, test.mime)
			}
		})
	}
}

func TestIdentityHTTPRequestsBlockPrivateAddresses(t *testing.T) {
	dir := publicIdentityDirectory().(*identity.CacheDirectory)
	base := dir.Inner.(*identity.BaseDirectory)
	if base.HTTPClient.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("identity requests must not use environment proxies")
	}
	_, err := base.HTTPClient.Get("http://127.0.0.1:8000/.well-known/did.json")
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("identity lookup can reach private addresses: %v", err)
	}
}
