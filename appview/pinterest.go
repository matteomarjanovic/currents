package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	pinterestBase = "https://www.pinterest.com"
	pinterestUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

var pinterestHTTP = &http.Client{Timeout: 30 * time.Second}

var (
	errPinterestPrivateProfile  = errors.New("Pinterest profile is private")
	errPinterestProfileNotFound = errors.New("Pinterest profile not found")
)

func resolvePinterestUsername(ctx context.Context, input string) (string, string, error) {
	input = strings.TrimSpace(input)
	u, err := url.Parse(input)
	if err != nil || !strings.EqualFold(u.Hostname(), "pin.it") || u.Port() != "" || (u.Scheme != "http" && u.Scheme != "https") {
		username, err := normalizePinterestUsername(input)
		return username, input, err
	}

	u.Scheme = "https"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", fmt.Errorf("invalid Pinterest short URL")
	}
	req.Header.Set("User-Agent", pinterestUA)
	client := *pinterestHTTP
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		host := strings.ToLower(req.URL.Hostname())
		if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.Port() != "" ||
			(host != "pin.it" && host != "pinterest.com" && !strings.HasSuffix(host, ".pinterest.com")) {
			return fmt.Errorf("Pinterest short URL redirects outside Pinterest")
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("could not resolve Pinterest short URL")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Pinterest short URL returned status %d", resp.StatusCode)
	}
	resolved := resp.Request.URL.String()
	username, err := normalizePinterestUsername(resolved)
	return username, resolved, err
}

func pinterestBoardPath(input string) string {
	u, err := url.Parse(input)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || strings.HasPrefix(parts[1], "_") {
		return ""
	}
	return "/" + parts[0] + "/" + parts[1]
}

func normalizePinterestUsername(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("username is required")
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("invalid Pinterest URL")
		}
		host := strings.ToLower(u.Hostname())
		if host != "pinterest.com" && !strings.HasSuffix(host, ".pinterest.com") {
			return "", fmt.Errorf("URL must be on pinterest.com")
		}
		username := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
		if username == "" {
			return "", fmt.Errorf("Pinterest URL has no username")
		}
		return username, nil
	}

	username := strings.TrimPrefix(input, "@")
	if strings.ContainsAny(username, "/?#") {
		return "", fmt.Errorf("enter a Pinterest username or full profile/board URL")
	}
	return username, nil
}

type PinterestBoard struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	PinCount int    `json:"pinCount"`
	URL      string `json:"url"`
}

type PinterestPin struct {
	ID        string
	ImageURL  string
	SourceURL string // original external source, "" when none
}

type PinterestSection struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"` // board-relative section URL, e.g. "/user/board/section/"
}

// externalSourceURL returns raw only when it is an http(s) URL pointing
// somewhere other than Pinterest itself; otherwise "". Used to decide whether a
// pin's link is a real origin worth surfacing as the save source.
func externalSourceURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	if host == "pinterest.com" || host == "pin.it" || strings.HasSuffix(host, ".pinterest.com") || strings.HasPrefix(host, "pinterest.") {
		return ""
	}
	return raw
}

// ListBoards paginates Pinterest's BoardsResource for the user's public boards.
func ListBoards(ctx context.Context, username string) ([]PinterestBoard, error) {
	sourceURL := "/" + username + "/"
	bookmark := ""
	out := make([]PinterestBoard, 0, 25)
	for {
		options := map[string]any{
			"username":      username,
			"page_size":     25,
			"field_set_key": "profile_grid_item",
		}
		if bookmark != "" {
			options["bookmarks"] = []string{bookmark}
		}
		data, err := json.Marshal(map[string]any{"options": options, "context": map[string]any{}})
		if err != nil {
			return nil, err
		}
		q := url.Values{}
		q.Set("source_url", sourceURL)
		q.Set("data", string(data))
		endpoint := pinterestBase + "/resource/BoardsResource/get/?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", pinterestUA)
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req.Header.Set("X-Pinterest-PWS-Handler", "www/"+username+".js")
		resp, err := pinterestHTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("listing boards: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("listing boards: %w", readErr)
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, errPinterestProfileNotFound
		}
		if resp.StatusCode == http.StatusForbidden {
			var failure struct {
				ResourceResponse struct {
					Error struct {
						Code int `json:"code"`
					} `json:"error"`
				} `json:"resource_response"`
			}
			if json.Unmarshal(body, &failure) == nil && failure.ResourceResponse.Error.Code == 4808 {
				return nil, errPinterestPrivateProfile
			}
		}
		if resp.StatusCode != http.StatusOK {
			preview := body
			if len(preview) > 512 {
				preview = preview[:512]
			}
			return nil, fmt.Errorf("BoardsResource: status %d: %s", resp.StatusCode, string(preview))
		}

		var result struct {
			ResourceResponse struct {
				Data []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					URL      string `json:"url"`
					PinCount int    `json:"pin_count"`
					Privacy  string `json:"privacy"`
				} `json:"data"`
				Bookmark json.RawMessage `json:"bookmark"`
			} `json:"resource_response"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing BoardsResource response: %w", err)
		}
		for _, b := range result.ResourceResponse.Data {
			if b.ID == "" || b.Privacy != "public" {
				continue
			}
			out = append(out, PinterestBoard{ID: b.ID, Name: b.Name, PinCount: b.PinCount, URL: b.URL})
		}
		next := extractBookmark(result.ResourceResponse.Bookmark)
		if next == "" || next == "-end-" {
			break
		}
		bookmark = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListSections paginates BoardSectionsResource for boardID. boardURL (e.g.
// "/user/board-name/") is used as the source_url and to build each section's
// board-relative URL from its slug.
func ListSections(ctx context.Context, boardID, boardURL string) ([]PinterestSection, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	primeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pinterestBase+boardURL, nil)
	if err != nil {
		return nil, err
	}
	primeReq.Header.Set("User-Agent", pinterestUA)
	primeReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	primeReq.Header.Set("Accept-Language", "en-US,en;q=0.5")
	primeResp, err := client.Do(primeReq)
	if err != nil {
		return nil, fmt.Errorf("priming session: %w", err)
	}
	io.Copy(io.Discard, primeResp.Body)
	primeResp.Body.Close()

	bookmark := ""
	out := make([]PinterestSection, 0, 8)
	for {
		options := map[string]any{
			"board_id":  boardID,
			"page_size": 25,
		}
		if bookmark != "" {
			options["bookmarks"] = []string{bookmark}
		}
		data, err := json.Marshal(map[string]any{"options": options, "context": map[string]any{}})
		if err != nil {
			return nil, err
		}
		q := url.Values{}
		q.Set("source_url", boardURL)
		q.Set("data", string(data))
		endpoint := pinterestBase + "/resource/BoardSectionsResource/get/?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", pinterestUA)
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Pinterest-AppState", "active")
		req.Header.Set("X-Pinterest-PWS-Handler", "www/[username]/[slug].js")
		req.Header.Set("Referer", "https://www.pinterest.com/")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("listing sections: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("listing sections: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			preview := body
			if len(preview) > 512 {
				preview = preview[:512]
			}
			return nil, fmt.Errorf("BoardSectionsResource: status %d: %s", resp.StatusCode, string(preview))
		}

		var result struct {
			ResourceResponse struct {
				Data []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Slug  string `json:"slug"`
				} `json:"data"`
				Bookmark json.RawMessage `json:"bookmark"`
			} `json:"resource_response"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing BoardSectionsResource response: %w", err)
		}
		for _, sec := range result.ResourceResponse.Data {
			if sec.ID == "" {
				continue
			}
			out = append(out, PinterestSection{ID: sec.ID, Title: sec.Title, URL: boardURL + sec.Slug + "/"})
		}
		next := extractBookmark(result.ResourceResponse.Bookmark)
		if next == "" || next == "-end-" {
			break
		}
		bookmark = next
	}
	return out, nil
}

// PinterestClient keeps one anonymous visitor session across a user's import.
// Pinterest restricts some visitors to a preview; retrying with the same cookie
// does not remove that restriction.
type PinterestClient struct {
	http    *http.Client
	healthy bool
}

const pinterestSessionAttempts = 5

var pinterestRetryDelay = time.Second

type PinterestPage struct {
	Pins        []PinterestPin
	Count       int // actual pins, excluding recommendation modules
	Unsupported int
	Next        string
	First       bool
}

func newPinterestClient() *PinterestClient {
	jar, _ := cookiejar.New(nil)
	client := *pinterestHTTP
	client.Jar = jar
	return &PinterestClient{http: &client}
}

func waitPinterest(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type pinterestHTTPError struct{ status int }

func (e pinterestHTTPError) Error() string {
	return fmt.Sprintf("Pinterest returned HTTP %d", e.status)
}

func (p *PinterestClient) get(ctx context.Context, endpoint string, prime bool, retry func() error) ([]byte, error) {
	var last error
	for attempt := range 3 {
		if attempt > 0 {
			if err := retry(); err != nil {
				return nil, err
			}
			if err := waitPinterest(ctx, time.Duration(attempt)*pinterestRetryDelay); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", pinterestUA)
		if prime {
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
			req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		} else {
			req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.Header.Set("X-Pinterest-AppState", "active")
			req.Header.Set("X-Pinterest-PWS-Handler", "www/[username]/[slug].js")
			req.Header.Set("Referer", pinterestBase+"/")
		}
		resp, err := p.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			last = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			last = pinterestHTTPError{resp.StatusCode}
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return nil, last
			}
			continue
		}
		if readErr != nil {
			last = readErr
			continue
		}
		return body, nil
	}
	return nil, last
}

func (p *PinterestClient) prime(ctx context.Context, path string, retry func() error) error {
	_, err := p.get(ctx, pinterestBase+path, true, retry)
	return err
}

func (p *PinterestClient) page(ctx context.Context, resource, path string, options map[string]any, retry func() error) (PinterestPage, error) {
	data, err := json.Marshal(map[string]any{"options": options, "context": map[string]any{}})
	if err != nil {
		return PinterestPage{}, err
	}
	q := url.Values{"source_url": {path}, "data": {string(data)}}
	body, err := p.get(ctx, pinterestBase+"/resource/"+resource+"/get/?"+q.Encode(), false, retry)
	if err != nil {
		return PinterestPage{}, err
	}
	var result struct {
		Response *struct {
			Status string `json:"status"`
			Data   []struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				Link   string `json:"link"`
				Images struct {
					Orig struct {
						URL string `json:"url"`
					} `json:"orig"`
				} `json:"images"`
			} `json:"data"`
			Bookmark json.RawMessage `json:"bookmark"`
		} `json:"resource_response"`
		Resource struct {
			Options struct {
				Bookmarks []string `json:"bookmarks"`
			} `json:"options"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return PinterestPage{}, fmt.Errorf("parsing Pinterest response: %w", err)
	}
	if result.Response == nil || (result.Response.Status != "" && result.Response.Status != "success") {
		return PinterestPage{}, fmt.Errorf("Pinterest did not return a pin listing")
	}
	page := PinterestPage{Next: extractBookmark(result.Response.Bookmark)}
	if page.Next == "" && len(result.Resource.Options.Bookmarks) > 0 {
		page.Next = result.Resource.Options.Bookmarks[0]
	}
	for _, pin := range result.Response.Data {
		if pin.ID == "" || (pin.Type != "" && pin.Type != "pin") {
			continue
		}
		page.Count++
		if pin.Images.Orig.URL == "" {
			page.Unsupported++
			continue
		}
		page.Pins = append(page.Pins, PinterestPin{ID: pin.ID, ImageURL: pin.Images.Orig.URL, SourceURL: externalSourceURL(pin.Link)})
	}
	return page, nil
}

func pinterestBoardOptions(board string, filter bool) map[string]any {
	return map[string]any{"board_id": board, "page_size": 250, "field_set_key": "react_grid_pin", "filter_section_pins": filter, "add_vase": true, "is_react": true}
}

func (p PinterestPage) ended() bool { return p.Next == "" || p.Next == "-end-" }
func (p PinterestPage) preview(expected int) bool {
	return p.ended() && p.Count <= 25 && expected > p.Count
}

// Iterate stores complete pages only. A preview may contribute new images, but
// never establishes that a large board has been completely listed.
func iteratePinterest(p **PinterestClient, ctx context.Context, job ImportJobRow, onPage func(PinterestPage) error, retry func() error) error {
	path := job.SourceBoardURL
	rootPath := path
	if job.SourceSectionID != "" {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 {
			rootPath = "/" + parts[0] + "/" + parts[1] + "/"
		}
	}
	for attempt := range pinterestSessionAttempts {
		if *p == nil || attempt > 0 {
			if attempt > 0 {
				if err := retry(); err != nil {
					return err
				}
				if err := waitPinterest(ctx, time.Duration(attempt)*pinterestRetryDelay); err != nil {
					return err
				}
			}
			*p = newPinterestClient()
			if err := (*p).prime(ctx, rootPath, retry); err != nil {
				*p = nil
				return err
			}
		}
		// Sections and section-filtered boards have no comparable expected
		// count. Check their whole parent board before trusting a new session.
		if !(*p).healthy && (job.SourceSectionID != "" || job.FilterSectionPins || job.ListCursor != "") && max(job.BoardPinCount, job.ExpectedCount) > 25 {
			probe, err := (*p).page(ctx, "BoardFeedResource", rootPath, pinterestBoardOptions(job.SourceBoardID, false), retry)
			if err != nil {
				return err
			}
			if probe.preview(max(job.BoardPinCount, job.ExpectedCount)) {
				if attempt == pinterestSessionAttempts-1 {
					*p = nil
					return fmt.Errorf("Pinterest kept returning a limited board preview")
				}
				continue
			}
			(*p).healthy = !probe.ended() || probe.Count >= max(job.BoardPinCount, job.ExpectedCount)
		}
		bookmark := job.ListCursor
		if attempt > 0 {
			bookmark = ""
		}
		seen := map[string]bool{}
		first := true
		for {
			resource := "BoardFeedResource"
			options := pinterestBoardOptions(job.SourceBoardID, job.FilterSectionPins)
			if job.SourceSectionID != "" {
				resource = "BoardSectionPinsResource"
				options = map[string]any{"section_id": job.SourceSectionID, "page_size": 50, "field_set_key": "react_grid_pin"}
			}
			if bookmark != "" {
				options["bookmarks"] = []string{bookmark}
			}
			page, err := (*p).page(ctx, resource, path, options, retry)
			if err != nil {
				return err
			}
			page.First = first && bookmark == ""
			preview := first && bookmark == "" && page.preview(job.ExpectedCount)
			if preview {
				page.Next = "" // a new attempt must start at page one
			} else if page.ended() {
				page.Next = "-end-"
			}
			if err := onPage(page); err != nil {
				return err
			}
			if preview {
				if attempt == pinterestSessionAttempts-1 {
					*p = nil
					return fmt.Errorf("Pinterest kept returning a limited board preview")
				}
				break
			}
			if !page.ended() {
				(*p).healthy = true
			}
			if page.ended() {
				return nil
			}
			if seen[page.Next] || page.Next == bookmark {
				return fmt.Errorf("Pinterest repeated a page instead of continuing the board")
			}
			seen[page.Next] = true
			bookmark = page.Next
			first = false
		}
	}
	return nil
}

// extractBookmark handles Pinterest's string and string-array formats.
func extractBookmark(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr[0]
	}
	return ""
}
