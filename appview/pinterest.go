package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
			return nil, fmt.Errorf("pinterest user %q not found", username)
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

// IteratePins paginates BoardFeedResource for boardID. When filterSectionPins
// is true, only pins not assigned to any section are returned (used when the
// board's sections are imported separately). See iteratePinResource for the
// callback contract.
func IteratePins(ctx context.Context, boardID, boardURL, startBookmark string, filterSectionPins bool, fn func(pin PinterestPin, nextBookmark string) error) (string, error) {
	buildOptions := func(bookmark string) map[string]any {
		o := map[string]any{
			"board_id":            boardID,
			"page_size":           250,
			"field_set_key":       "react_grid_pin",
			"filter_section_pins": filterSectionPins,
			"add_vase":            true,
			"is_react":            true,
		}
		if bookmark != "" {
			o["bookmarks"] = []string{bookmark}
		}
		return o
	}
	return iteratePinResource(ctx, "BoardFeedResource", boardURL, boardURL, startBookmark, buildOptions, fn)
}

// IterateSectionPins paginates BoardSectionPinsResource for sectionID.
// sectionURL (e.g. "/user/board/section/") primes the session cookie and is
// used as the source_url.
func IterateSectionPins(ctx context.Context, sectionID, sectionURL, startBookmark string, fn func(pin PinterestPin, nextBookmark string) error) (string, error) {
	buildOptions := func(bookmark string) map[string]any {
		o := map[string]any{
			"section_id": sectionID,
			// BoardSectionPinsResource caps page_size at 50 (BoardFeedResource allows 250).
			"page_size":     50,
			"field_set_key": "react_grid_pin",
		}
		if bookmark != "" {
			o["bookmarks"] = []string{bookmark}
		}
		return o
	}
	return iteratePinResource(ctx, "BoardSectionPinsResource", sectionURL, sectionURL, startBookmark, buildOptions, fn)
}

// iteratePinResource paginates a Pinterest pin-grid resource. primeURL is
// visited first to prime an unauth session cookie — without it Pinterest
// returns 403. fn is called for every pin that has an "orig" image; pins
// without one (videos, story pins) are skipped. nextBookmark passed to fn is
// the cursor for that page; persist it in list_cursor to resume after a crash.
func iteratePinResource(ctx context.Context, resource, primeURL, sourceURL, startBookmark string, buildOptions func(bookmark string) map[string]any, fn func(pin PinterestPin, nextBookmark string) error) (string, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	primeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pinterestBase+primeURL, nil)
	if err != nil {
		return startBookmark, err
	}
	primeReq.Header.Set("User-Agent", pinterestUA)
	primeReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	primeReq.Header.Set("Accept-Language", "en-US,en;q=0.5")
	primeResp, err := client.Do(primeReq)
	if err != nil {
		return startBookmark, fmt.Errorf("priming session: %w", err)
	}
	io.Copy(io.Discard, primeResp.Body)
	primeResp.Body.Close()

	bookmark := startBookmark
	for {
		data, err := json.Marshal(map[string]any{"options": buildOptions(bookmark), "context": map[string]any{}})
		if err != nil {
			return bookmark, err
		}
		q := url.Values{}
		q.Set("source_url", sourceURL)
		q.Set("data", string(data))
		endpoint := fmt.Sprintf("%s/resource/%s/get/?%s", pinterestBase, resource, q.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return bookmark, err
		}
		req.Header.Set("User-Agent", pinterestUA)
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Pinterest-AppState", "active")
		req.Header.Set("X-Pinterest-PWS-Handler", "www/[username]/[slug].js")
		req.Header.Set("Referer", "https://www.pinterest.com/")
		resp, err := client.Do(req)
		if err != nil {
			return bookmark, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			return bookmark, readErr
		}
		if resp.StatusCode != http.StatusOK {
			preview := body
			if len(preview) > 512 {
				preview = preview[:512]
			}
			return bookmark, fmt.Errorf("%s: status %d: %s", resource, resp.StatusCode, string(preview))
		}

		var result struct {
			ResourceResponse struct {
				Data []struct {
					ID     string `json:"id"`
					Link   string `json:"link"`
					Images struct {
						Orig struct {
							URL string `json:"url"`
						} `json:"orig"`
					} `json:"images"`
				} `json:"data"`
				Bookmark json.RawMessage `json:"bookmark"` // string or []string
			} `json:"resource_response"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return bookmark, fmt.Errorf("parsing %s response: %w", resource, err)
		}

		next := extractBookmark(result.ResourceResponse.Bookmark)
		for _, p := range result.ResourceResponse.Data {
			if p.Images.Orig.URL == "" {
				slog.Info("pinterest: skipping pin without orig image", "pin_id", p.ID, "link", p.Link, "url", fmt.Sprintf("https://www.pinterest.com/pin/%s/", p.ID))
				continue
			}
			if err := fn(PinterestPin{ID: p.ID, ImageURL: p.Images.Orig.URL, SourceURL: externalSourceURL(p.Link)}, next); err != nil {
				return bookmark, err
			}
		}
		bookmark = next
		if bookmark == "" || bookmark == "-end-" {
			return bookmark, nil
		}
	}
}

// extractBookmark handles the bookmark field which Pinterest returns as either
// a plain string or a JSON array of strings.
func extractBookmark(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
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
