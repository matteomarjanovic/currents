package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func stubPinterest(t *testing.T, serve func(*http.Request) (int, http.Header, string)) {
	t.Helper()
	original, delay := pinterestHTTP, pinterestRetryDelay
	t.Cleanup(func() { pinterestHTTP, pinterestRetryDelay = original, delay })
	pinterestRetryDelay = 0
	pinterestHTTP = &http.Client{Transport: pinterestRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, header, body := serve(r)
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
}

func pinPage(count int, next string) string {
	pins := []any{map[string]any{"id": "module", "type": "story"}}
	for i := range count {
		pins = append(pins, map[string]any{"id": fmt.Sprintf("pin%d", i), "type": "pin", "images": map[string]any{"orig": map[string]any{"url": fmt.Sprintf("https://i.pinimg.com/%d.jpg", i)}}})
	}
	b, _ := json.Marshal(map[string]any{"resource_response": map[string]any{"status": "success", "data": pins, "bookmark": next}})
	return string(b)
}

func TestPinterestRefreshesPreviewAndReusesHealthySession(t *testing.T) {
	primes, retries, requests := 0, 0, 0
	stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
		if !strings.Contains(r.URL.Path, "/resource/") {
			primes++
			return 200, http.Header{"Set-Cookie": {fmt.Sprintf("visitor=%d; Path=/", primes)}}, "html"
		}
		requests++
		cookie, _ := r.Cookie("visitor")
		if cookie == nil {
			t.Fatal("anonymous session cookie missing")
		}
		if cookie.Value == "1" {
			return 200, nil, pinPage(25, "-end-")
		}
		var data struct {
			Options map[string]json.RawMessage `json:"options"`
		}
		json.Unmarshal([]byte(r.URL.Query().Get("data")), &data)
		if data.Options["bookmarks"] == nil {
			return 200, nil, pinPage(150, "next")
		}
		return 200, nil, pinPage(30, "-end-")
	})
	var client *PinterestClient
	pages := []PinterestPage{}
	job := ImportJobRow{SourceBoardID: "board", SourceBoardURL: "/u/board/", ExpectedCount: 200}
	onPage := func(p PinterestPage) error { pages = append(pages, p); return nil }
	onRetry := func() error { retries++; return nil }
	if err := iteratePinterest(&client, context.Background(), job, onPage, onRetry); err != nil {
		t.Fatal(err)
	}
	if primes != 2 || retries != 1 || len(pages) != 3 || pages[0].Count != 25 || pages[1].Count != 150 || !pages[1].First || pages[2].First || pages[2].Next != "-end-" {
		t.Fatalf("primes=%d retries=%d pages=%+v", primes, retries, pages)
	}
	job.SourceBoardID = "another"
	if err := iteratePinterest(&client, context.Background(), job, onPage, onRetry); err != nil {
		t.Fatal(err)
	}
	if primes != 2 || requests != 5 {
		t.Fatalf("healthy session not reused: primes=%d requests=%d", primes, requests)
	}
}

func TestPinterestPreviewRetriesAreBoundedAndKeepImages(t *testing.T) {
	primes, pages := 0, 0
	stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
		if !strings.Contains(r.URL.Path, "/resource/") {
			primes++
			return 200, nil, "html"
		}
		return 200, nil, pinPage(24, "-end-")
	})
	var client *PinterestClient
	err := iteratePinterest(&client, context.Background(), ImportJobRow{SourceBoardURL: "/u/b/", ExpectedCount: 1000}, func(p PinterestPage) error {
		pages++
		if len(p.Pins) != 24 || p.Count != 24 || p.Unsupported != 0 {
			t.Fatalf("preview/module counting: %+v", p)
		}
		return nil
	}, func() error { return nil })
	if err == nil || primes != 5 || pages != 5 || client != nil {
		t.Fatalf("err=%v primes=%d pages=%d", err, primes, pages)
	}
}

func TestPinterestSmallBoardsAreNotRetried(t *testing.T) {
	primes := 0
	stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
		if !strings.Contains(r.URL.Path, "/resource/") {
			primes++
			return 200, nil, "html"
		}
		return 200, nil, pinPage(12, "-end-")
	})
	var client *PinterestClient
	err := iteratePinterest(&client, context.Background(), ImportJobRow{SourceBoardURL: "/u/b/", ExpectedCount: 12}, func(PinterestPage) error { return nil }, func() error { return nil })
	if err != nil || primes != 1 {
		t.Fatalf("small board err=%v primes=%d", err, primes)
	}
}

func TestPinterestRetriesTheSamePageAfterTemporaryFailure(t *testing.T) {
	second := 0
	stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
		if !strings.Contains(r.URL.Path, "/resource/") {
			return 200, nil, "html"
		}
		if !strings.Contains(r.URL.Query().Get("data"), "bookmarks") {
			return 200, nil, pinPage(150, "second")
		}
		second++
		if second == 1 {
			return 500, nil, "temporary"
		}
		return 200, nil, pinPage(40, "-end-")
	})
	var client *PinterestClient
	pages := 0
	err := iteratePinterest(&client, context.Background(), ImportJobRow{SourceBoardURL: "/u/b/", ExpectedCount: 200}, func(PinterestPage) error { pages++; return nil }, func() error { return nil })
	if err != nil || second != 2 || pages != 2 {
		t.Fatalf("err=%v second=%d committed pages=%d", err, second, pages)
	}
}

func TestPinterestChecksParentBeforeSectionOrResumedListing(t *testing.T) {
	for _, section := range []bool{true, false} {
		t.Run(fmt.Sprint(section), func(t *testing.T) {
			primes, probes, reads := 0, 0, 0
			stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
				if !strings.Contains(r.URL.Path, "/resource/") {
					primes++
					return 200, nil, "html"
				}
				if !strings.Contains(r.URL.Query().Get("data"), "bookmarks") && !strings.Contains(r.URL.Path, "BoardSectionPinsResource") {
					probes++
					if primes == 1 {
						return 200, nil, pinPage(25, "-end-")
					}
					return 200, nil, pinPage(100, "next")
				}
				reads++
				return 200, nil, pinPage(7, "-end-")
			})
			job := ImportJobRow{SourceBoardID: "root", SourceBoardURL: "/u/b/section/", BoardPinCount: 400}
			if section {
				job.SourceSectionID = "section"
			} else {
				job.ListCursor = "resume"
			}
			var client *PinterestClient
			// For a resumed job, refreshing the visitor restarts listing from the
			// beginning rather than trusting a cursor in a restricted session.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := iteratePinterest(&client, ctx, job, func(p PinterestPage) error {
				if !section {
					cancel()
					return context.Canceled
				}
				return nil
			}, func() error { return nil })
			if section && (err != nil || primes != 2 || probes != 2 || reads != 1) {
				t.Fatalf("err=%v primes=%d probes=%d reads=%d", err, primes, probes, reads)
			}
			if !section && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestPinterestStopsWhenPagePersistenceFails(t *testing.T) {
	requests := 0
	stubPinterest(t, func(r *http.Request) (int, http.Header, string) {
		if !strings.Contains(r.URL.Path, "/resource/") {
			return 200, nil, "html"
		}
		requests++
		return 200, nil, pinPage(250, "next")
	})
	want := errors.New("database unavailable")
	var client *PinterestClient
	err := iteratePinterest(&client, context.Background(), ImportJobRow{SourceBoardURL: "/u/b/", ExpectedCount: 1000}, func(PinterestPage) error { return want }, func() error { return nil })
	if !errors.Is(err, want) || requests != 1 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
}

func TestImportOutcomesDistinguishPinsImagesAndFailures(t *testing.T) {
	count := 100
	for _, tt := range []struct {
		name    string
		s       SessionJobStatus
		outcome string
		retry   bool
		missing int
	}{
		{"duplicate images", SessionJobStatus{Status: "done", Expected: 100, Listed: &count, Done: 80}, "complete", false, 0},
		{"legacy gap", SessionJobStatus{Status: "done", Expected: 1000, Done: 25}, "partial", true, 975},
		{"legacy unknown section", SessionJobStatus{Status: "done", Done: 12}, "complete", true, 0},
		{"unknown section", SessionJobStatus{Status: "done", Listed: &count, Done: 12}, "complete", false, 0},
		{"unsupported", SessionJobStatus{Status: "done", Expected: 100, Listed: &count, Done: 90, Unsupported: 10}, "partial", false, 0},
		{"upload failure", SessionJobStatus{Status: "done", Listed: &count, Done: 90, Failed: 10}, "partial", true, 0},
		{"interrupted", SessionJobStatus{Status: "done", Done: 12, ListingError: "failed"}, "partial", true, 0},
		{"empty failed", SessionJobStatus{Status: "done", ListingError: "failed"}, "failed", true, 0},
		{"still listing", SessionJobStatus{Status: "listing", Done: 12}, "", false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.s.summarize()
			if tt.s.Outcome != tt.outcome || tt.s.CanRetry != tt.retry || tt.s.Missing != tt.missing {
				t.Fatalf("%+v", tt.s)
			}
		})
	}
}
