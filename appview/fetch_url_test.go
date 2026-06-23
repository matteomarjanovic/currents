package main

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestIsDisallowedIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":         false, // public
		"1.1.1.1":         false,
		"127.0.0.1":       true,  // loopback
		"10.0.0.1":        true,  // private
		"192.168.1.1":     true,  // private
		"172.16.5.5":      true,  // private
		"169.254.169.254": true,  // link-local (cloud metadata)
		"0.0.0.0":         true,  // unspecified
		"::1":             true,  // loopback v6
		"fc00::1":         true,  // ULA
		"fe80::1":         true,  // link-local v6
		"2606:4700::1111": false, // public v6
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
