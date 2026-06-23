package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
)

// Paste-from-URL: scrape a page for its images and download a chosen image
// server-side. Both run in the appview (not the browser) to dodge CORS, the
// same reason /resave fetches blobs server-side.

const (
	maxFetchHTMLSize   = 5 << 20  // 5 MB of HTML is plenty to scrape
	maxFetchImageSize  = 25 << 20 // hard cap before prepareImageForUpload downscales
	maxExtractedImages = 60
	fetchUserAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

var errBlockedAddress = errors.New("address not allowed")

// urlFetchClient fetches arbitrary user-supplied URLs. Because the target is
// attacker-controlled, it is the appview's SSRF boundary: the dial Control hook
// re-checks the *resolved* IP right before connecting (so it also covers
// redirect hops and defeats DNS rebinding), and redirects are capped.
var urlFetchClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				if isDisallowedIP(net.ParseIP(host)) {
					return errBlockedAddress
				}
				return nil
			},
		}).DialContext,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return errBlockedAddress
		}
		return nil
	},
}

// isDisallowedIP rejects anything that isn't a routable public address —
// loopback, private (RFC1918 / ULA), link-local (incl. cloud metadata at
// 169.254.169.254), unspecified, and multicast.
func isDisallowedIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast()
}

// validateFetchURL enforces the http/https scheme before we make a request. The
// IP-level checks happen at dial time in urlFetchClient.
func validateFetchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	return u, nil
}

// safeOriginURL returns raw only when it's an http(s) URL, else "". originUrl is
// rendered as a clickable link in the web client, so a "javascript:"/"data:"
// value would be a stored-XSS vector — it must never be persisted.
func safeOriginURL(raw string) string {
	if _, err := validateFetchURL(raw); err != nil {
		return ""
	}
	return raw
}

func fetchURL(ctx context.Context, raw string) (*http.Response, error) {
	u, err := validateFetchURL(raw)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	return urlFetchClient.Do(req)
}

// fetchRemoteImage downloads a single image by URL for CreateSave's paste path.
func fetchRemoteImage(ctx context.Context, raw string) ([]byte, string, error) {
	resp, err := fetchURL(ctx, raw)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch status %d", resp.StatusCode)
	}
	ct := strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0])
	if !strings.HasPrefix(ct, "image/") {
		return nil, "", fmt.Errorf("not an image: %q", ct)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchImageSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxFetchImageSize {
		return nil, "", fmt.Errorf("image too large")
	}
	return data, ct, nil
}

// extractPageImages fetches a page and returns the absolute URLs of images on
// it. If the URL is itself an image, it returns just that one. Best-effort:
// lazy-loaded/CSS-background images won't always be found.
func extractPageImages(ctx context.Context, raw string) ([]string, error) {
	resp, err := fetchURL(ctx, raw)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch status %d", resp.StatusCode)
	}

	// resp.Request.URL is the final URL after redirects — the right base for
	// resolving relative references.
	base := resp.Request.URL
	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "image/") {
		return []string{base.String()}, nil
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, maxFetchHTMLSize))
	if err != nil {
		return nil, err
	}
	return collectImageURLs(doc, base), nil
}

func collectImageURLs(doc *html.Node, base *url.URL) []string {
	if href := findAttr(doc, "base", "href"); href != "" {
		if b, err := base.Parse(href); err == nil {
			base = b
		}
	}

	var out []string
	seen := map[string]bool{}
	add := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" || len(out) >= maxExtractedImages {
			return
		}
		r, err := base.Parse(ref)
		if err != nil || (r.Scheme != "http" && r.Scheme != "https") {
			return
		}
		r.Fragment = ""
		s := r.String()
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img":
				// og:image (in <head>) is walked first, so the hero image
				// tends to rank first. Lazy-load attrs cover common patterns.
				add(attr(n, "src"))
				add(attr(n, "data-src"))
				add(attr(n, "data-original"))
				add(attr(n, "data-lazy-src"))
				add(largestFromSrcset(attr(n, "srcset")))
			case "source":
				add(largestFromSrcset(attr(n, "srcset")))
			case "meta":
				switch {
				case attr(n, "property") == "og:image",
					attr(n, "property") == "og:image:url",
					attr(n, "name") == "twitter:image",
					attr(n, "name") == "twitter:image:src":
					add(attr(n, "content"))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// findAttr returns the named attribute of the first element of the given tag.
func findAttr(n *html.Node, tag, key string) string {
	if n.Type == html.ElementNode && n.Data == tag {
		return attr(n, key)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if v := findAttr(c, tag, key); v != "" {
			return v
		}
	}
	return ""
}

// largestFromSrcset picks the highest-resolution candidate from a srcset value
// ("url 320w, url 640w" or "url 1x, url 2x").
func largestFromSrcset(srcset string) string {
	best := ""
	bestScore := -1
	for _, part := range strings.Split(srcset, ",") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		score := 0
		if len(fields) > 1 {
			score, _ = strconv.Atoi(strings.TrimRight(fields[1], "wx"))
		}
		if score > bestScore {
			bestScore = score
			best = fields[0]
		}
	}
	return best
}

func (s *Server) APIExtractImages(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if _, err := validateFetchURL(body.URL); err != nil {
		http.Error(w, "invalid URL", http.StatusBadRequest)
		return
	}
	images, err := extractPageImages(r.Context(), body.URL)
	if err != nil {
		slog.Warn("extract images failed", "url", body.URL, "err", err)
		http.Error(w, "could not fetch images from that URL", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"images": images})
}
