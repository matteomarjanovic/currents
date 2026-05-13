package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
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
	ID       string
	ImageURL string
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

// IteratePins paginates BoardFeedResource for boardID. boardURL (e.g.
// "/user/board-name/") is used to prime an unauth session cookie before the
// first API call — without this, Pinterest returns 403. fn is called for every
// pin that has an "orig" image; pins without one (videos, story pins) are
// skipped. nextBookmark passed to fn is the cursor for that page; persist it
// in list_cursor to resume after a crash.
func IteratePins(ctx context.Context, boardID, boardURL, startBookmark string, fn func(pin PinterestPin, nextBookmark string) error) (string, error) {
	// Prime an unauth _pinterest_sess cookie by visiting the board page.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	primeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pinterestBase+boardURL, nil)
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
		options := map[string]any{
			"board_id":            boardID,
			"page_size":           250,
			"field_set_key":       "react_grid_pin",
			"filter_section_pins": false,
			"add_vase":            true,
			"is_react":            true,
		}
		if bookmark != "" {
			options["bookmarks"] = []string{bookmark}
		}
		data, err := json.Marshal(map[string]any{"options": options, "context": map[string]any{}})
		if err != nil {
			return bookmark, err
		}
		q := url.Values{}
		q.Set("source_url", boardURL)
		q.Set("data", string(data))
		endpoint := fmt.Sprintf("%s/resource/BoardFeedResource/get/?%s", pinterestBase, q.Encode())
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
			return bookmark, fmt.Errorf("BoardFeedResource: status %d: %s", resp.StatusCode, string(preview))
		}

		var result struct {
			ResourceResponse struct {
				Data []struct {
					ID     string `json:"id"`
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
			return bookmark, fmt.Errorf("parsing BoardFeedResource response: %w", err)
		}

		next := extractBookmark(result.ResourceResponse.Bookmark)
		for _, p := range result.ResourceResponse.Data {
			if p.Images.Orig.URL == "" {
				continue
			}
			if err := fn(PinterestPin{ID: p.ID, ImageURL: p.Images.Orig.URL}, next); err != nil {
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
