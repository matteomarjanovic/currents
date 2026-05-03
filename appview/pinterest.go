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
	ID       string
	ImageURL string
}

// ListBoards scrapes the public profile page HTML to extract board data.
// Pinterest's unofficial resource API requires session cookies; the profile
// page HTML contains the same data bootstrapped in __PWS_DATA__ for SSR.
func ListBoards(ctx context.Context, username string) ([]PinterestBoard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pinterestBase+"/"+username+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", pinterestUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	resp, err := pinterestHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing boards: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pinterest user %q not found", username)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing boards: profile page status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("listing boards: %w", err)
	}
	return parseBoardsFromHTML(string(body))
}

func parseBoardsFromHTML(htmlStr string) ([]PinterestBoard, error) {
	// Board data is in __PWS_INITIAL_PROPS__ > initialReduxState.boards (flat map).
	// __PWS_DATA__ is app config only; __PWS_INITIAL_PROPS__ has the Redux state.
	raw, err := extractScriptByID(htmlStr, "__PWS_INITIAL_PROPS__")
	if err != nil {
		return nil, fmt.Errorf("pinterest: %w", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("pinterest: parse __PWS_INITIAL_PROPS__: %w", err)
	}

	stateRaw, ok := top["initialReduxState"]
	if !ok {
		return nil, fmt.Errorf("pinterest: no 'initialReduxState' in __PWS_INITIAL_PROPS__ (keys: %s)", rawMapKeys(top))
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(stateRaw, &state); err != nil {
		return nil, fmt.Errorf("pinterest: parse initialReduxState: %w", err)
	}

	// boards is a flat map of board_id -> board object (not nested under boards.boards)
	boardsMapRaw, ok := state["boards"]
	if !ok {
		return nil, fmt.Errorf("pinterest: no 'boards' in initialReduxState (keys: %s)", rawMapKeys(state))
	}
	var boardsMap map[string]json.RawMessage
	if err := json.Unmarshal(boardsMapRaw, &boardsMap); err != nil {
		return nil, fmt.Errorf("pinterest: parse boards map: %w", err)
	}

	out := make([]PinterestBoard, 0, len(boardsMap))
	for _, bRaw := range boardsMap {
		var b struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			PinCount int    `json:"pin_count"`
			URL      string `json:"url"`
			Privacy  string `json:"privacy"`
		}
		if err := json.Unmarshal(bRaw, &b); err != nil || b.ID == "" {
			continue
		}
		if b.Privacy != "public" {
			continue
		}
		out = append(out, PinterestBoard{
			ID:       b.ID,
			Name:     b.Name,
			PinCount: b.PinCount,
			URL:      b.URL,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// extractScriptByID finds <script id="<id>" ...>...</script> and returns the JSON content.
func extractScriptByID(htmlStr, id string) (json.RawMessage, error) {
	needle := `id="` + id + `"`
	idx := strings.Index(htmlStr, needle)
	if idx == -1 {
		return nil, fmt.Errorf("script id=%q not found in HTML", id)
	}
	// Skip past the closing > of the opening tag.
	tagEnd := strings.Index(htmlStr[idx:], ">")
	if tagEnd == -1 {
		return nil, fmt.Errorf("script id=%q: opening tag not closed", id)
	}
	start := idx + tagEnd + 1
	end := strings.Index(htmlStr[start:], "</script>")
	if end == -1 {
		return nil, fmt.Errorf("script id=%q: closing tag not found", id)
	}
	content := strings.TrimSpace(htmlStr[start : start+end])
	// Handle optional `window.__X__ = {...}` assignment form.
	if i := strings.Index(content, "{"); i > 0 {
		content = content[i:]
	}
	return json.RawMessage(content), nil
}

func rawMapKeys(m map[string]json.RawMessage) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
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
		opts := map[string]any{
			"options": map[string]any{
				"board_id":      boardID,
				"page_size":     25,
				"currentFilter": -1,
			},
			"context": map[string]any{},
		}
		if bookmark != "" {
			opts["options"].(map[string]any)["bookmarks"] = []string{bookmark}
		}
		data, err := json.Marshal(opts)
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
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
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
