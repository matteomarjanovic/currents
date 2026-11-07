package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func stubURLFetchTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	original := urlFetchClient
	client := *original
	client.Transport = transport
	urlFetchClient = &client
	t.Cleanup(func() { urlFetchClient = original })
}

func TestFetchRemoteImageDetectsTypeFromBytes(t *testing.T) {
	webp, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, header, want string
		data               []byte
	}{
		{"PNG as binary", "application/octet-stream", "image/png", testBlobPNG},
		{"WebP as binary", "application/octet-stream", "image/webp", webp},
		{"PNG without header", "", "image/png", testBlobPNG},
		{"PNG with wrong image header", "image/jpeg", "image/png", testBlobPNG},
		{"HTML pretending to be an image", "image/png", "", []byte("<!doctype html><html>not an image</html>")},
		{"SVG", "image/svg+xml", "", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			stubURLFetchTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
				return blobResponse(r, io.NopCloser(bytes.NewReader(test.data)), int64(len(test.data)), test.header), nil
			}))
			data, mime, err := fetchRemoteImage(context.Background(), "https://example.com/image")
			if test.want == "" {
				if !errors.Is(err, errInvalidImage) {
					t.Fatalf("got %v, want invalid image", err)
				}
				return
			}
			if err != nil || mime != test.want || !bytes.Equal(data, test.data) {
				t.Fatalf("got mime %q, unchanged bytes %v, err %v; want %q", mime, bytes.Equal(data, test.data), err, test.want)
			}
		})
	}
}

func TestExtractPageImagesDetectsImageWithGenericHeader(t *testing.T) {
	stubURLFetchTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		return blobResponse(r, io.NopCloser(bytes.NewReader(testBlobPNG)), int64(len(testBlobPNG)), "application/octet-stream"), nil
	}))
	const raw = "https://example.com/logo"
	images, err := extractPageImages(context.Background(), raw)
	if err != nil || len(images) != 1 || images[0] != raw {
		t.Fatalf("got %v, %v; want direct image URL", images, err)
	}
}

func TestExtractPageImagesKeepsBufferedHTML(t *testing.T) {
	page := `<html><body><img src="/image.png"></body></html>`
	stubURLFetchTransport(t, blobRoundTripper(func(r *http.Request) (*http.Response, error) {
		return blobResponse(r, io.NopCloser(strings.NewReader(page)), int64(len(page)), "text/html"), nil
	}))
	images, err := extractPageImages(context.Background(), "https://example.com/page")
	if err != nil || len(images) != 1 || images[0] != "https://example.com/image.png" {
		t.Fatalf("got %v, %v; want page image URL", images, err)
	}
}

func TestIsDisallowedIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":          false, // public
		"1.1.1.1":          false,
		"127.0.0.1":        true,  // loopback
		"10.0.0.1":         true,  // private
		"192.168.1.1":      true,  // private
		"172.16.5.5":       true,  // private
		"169.254.169.254":  true,  // link-local (cloud metadata)
		"0.0.0.0":          true,  // unspecified
		"::1":              true,  // loopback v6
		"fc00::1":          true,  // ULA
		"fe80::1":          true,  // link-local v6
		"2606:4700::1111":  false, // public v6
		"::ffff:127.0.0.1": true,  // mapped IPv4 must not bypass the check
		"::ffff:10.0.0.1":  true,
		"255.255.255.255":  true,
		"224.0.0.1":        true,
	}
	for ipStr, want := range cases {
		if got := isDisallowedIP(net.ParseIP(ipStr)); got != want {
			t.Errorf("isDisallowedIP(%s) = %v, want %v", ipStr, got, want)
		}
	}
	if !isDisallowedIP(nil) {
		t.Error("isDisallowedIP(nil) should be true")
	}
}

func TestSafeOriginURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/article":       "https://example.com/article",
		"http://example.com":                "http://example.com",
		"javascript:alert(document.cookie)": "", // stored-XSS payload
		"data:text/html,<script>x</script>": "",
		"  javascript:alert(1)  ":           "", // whitespace-padded
		"/relative/path":                    "",
		"ftp://example.com/f":               "",
		"":                                  "",
	}
	for in, want := range cases {
		if got := safeOriginURL(in); got != want {
			t.Errorf("safeOriginURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLargestFromSrcset(t *testing.T) {
	cases := map[string]string{
		"a.jpg 320w, b.jpg 640w, c.jpg 1280w": "c.jpg",
		"a.jpg 1x, b.jpg 2x":                  "b.jpg",
		"only.jpg":                            "only.jpg",
		"":                                    "",
	}
	for in, want := range cases {
		if got := largestFromSrcset(in); got != want {
			t.Errorf("largestFromSrcset(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectImageURLs(t *testing.T) {
	page := `<html><head>
		<meta property="og:image" content="https://cdn.example.com/hero.jpg">
		<base href="https://example.com/blog/">
	</head><body>
		<img src="a.jpg">
		<img src="/abs/b.png">
		<img data-src="lazy.gif">
		<img srcset="s-320.jpg 320w, s-640.jpg 640w">
		<img src="a.jpg">  <!-- dup -->
		<img src="data:image/png;base64,xxx">  <!-- skipped -->
		<picture><source srcset="p-1.jpg 1x, p-2.jpg 2x"></picture>
	</body></html>`

	base, _ := url.Parse("https://example.com/blog/index.html")
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	got := collectImageURLs(doc, base)

	want := []string{
		"https://cdn.example.com/hero.jpg", // og:image walked first
		"https://example.com/blog/a.jpg",   // relative to <base>
		"https://example.com/abs/b.png",    // root-relative
		"https://example.com/blog/lazy.gif",
		"https://example.com/blog/s-640.jpg", // largest srcset
		"https://example.com/blog/p-2.jpg",   // largest <source> srcset
	}
	if len(got) != len(want) {
		t.Fatalf("got %d urls %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("url[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
