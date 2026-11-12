package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

const budgetTestPDS = "https://fibercap.us-west.host.bsky.network"

func budgetHeaders(now time.Time, remaining int, left time.Duration) http.Header {
	h := make(http.Header)
	h.Set("Date", now.UTC().Format(http.TimeFormat))
	h.Set("RateLimit-Policy", "1000;w=86400")
	h.Set("RateLimit-Limit", "1000")
	h.Set("RateLimit-Remaining", fmt.Sprint(remaining))
	h.Set("RateLimit-Reset", fmt.Sprint(now.Add(left).Unix()))
	return h
}

func TestImportUploadReserve(t *testing.T) {
	for _, tt := range []struct {
		left time.Duration
		want int
	}{
		{25 * time.Hour, 1000}, {24 * time.Hour, 1000},
		{12 * time.Hour, 583}, {6 * time.Hour, 374},
		{time.Hour, 200}, {30 * time.Minute, 119},
		{5 * time.Minute, 50}, {time.Second, 50},
	} {
		if got := importUploadReserve(tt.left); got != tt.want {
			t.Errorf("%s: got %d, want %d", tt.left, got, tt.want)
		}
	}
}

func TestImportUploadWaitPreservesReserve(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for _, left := range []time.Duration{24 * time.Hour, 12 * time.Hour, time.Hour, 30 * time.Minute, 5 * time.Minute} {
		reserve := importUploadReserve(left)
		b := importUploadBudget{Window: &importBlobWindow{Remaining: reserve, Reset: now.Add(left)}}
		wait := b.wait(now)
		if wait <= 0 {
			t.Fatalf("%s: allowed spending the reserve", left)
		}
		if b.wait(now.Add(wait)) > 0 {
			t.Errorf("%s: next slot is still blocked after %s", left, wait)
		}
		b.Window.Remaining++
		if wait := b.wait(now); wait != 0 {
			t.Errorf("%s: capacity above reserve unnecessarily blocked for %s", left, wait)
		}
	}
	b := importUploadBudget{Window: &importBlobWindow{Remaining: 50, Reset: now.Add(5 * time.Minute)}}
	if wait := b.wait(now); wait < 5*time.Minute {
		t.Fatalf("final 50 released before reset: %s", wait)
	}
}

func TestImportUploadWaitPacingManualAndReset(t *testing.T) {
	now := time.Now()
	b := importUploadBudget{Window: &importBlobWindow{Remaining: 900, Reset: now.Add(time.Hour)}, LastAttempt: now}
	if got := b.wait(now); got != 6*time.Second {
		t.Fatalf("spacing = %s", got)
	}
	b.ManualUntil = now.Add(2 * time.Minute)
	if got := b.wait(now); got != 2*time.Minute {
		t.Fatalf("manual activity wait = %s", got)
	}
	b.RetryAt = now.Add(3 * time.Hour)
	if got := b.wait(now); got != 3*time.Hour {
		t.Fatalf("retry wait = %s", got)
	}
	if got := b.wait(now.Add(4 * time.Hour)); got != 0 {
		t.Fatalf("expired state prevented new-window discovery: %s", got)
	}
}

func TestImportUploadPolicyParsing(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct{ key, value string }{
		{"RateLimit-Policy", "3000;w=300"},
		{"RateLimit-Policy", "1000;w=3600"},
		{"RateLimit-Policy", ""},
		{"RateLimit-Limit", "999"},
		{"RateLimit-Remaining", "-1"},
		{"RateLimit-Remaining", "1001"},
		{"RateLimit-Remaining", ""},
		{"RateLimit-Reset", "86400"}, // Relative seconds are not a PDS epoch.
		{"RateLimit-Reset", fmt.Sprint(now.Add(25 * time.Hour).Unix())},
	} {
		h := budgetHeaders(now, 700, time.Hour)
		h.Set(tt.key, tt.value)
		if got := parseImportBlobWindow(h, now); got != nil {
			t.Errorf("accepted %s=%q: %+v", tt.key, tt.value, got)
		}
	}
	h := budgetHeaders(now.Add(-10*time.Minute), 700, time.Hour)
	got := parseImportBlobWindow(h, now)
	if got == nil || got.Remaining != 700 || !got.Reset.Equal(now.Add(time.Hour)) {
		t.Fatalf("PDS clock skew not respected: %+v", got)
	}
}

func TestImportUploadProviderGate(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	for _, host := range []string{
		"https://eurosky.social", "https://blacksky.app", "https://selfhosted.social",
		"https://fibercap.us-west.host.bsky.network.evil.example",
		"http://fibercap.us-west.host.bsky.network", "https://host.bsky.network",
		"https://user@fibercap.us-west.host.bsky.network", budgetTestPDS + ":8443",
	} {
		b := importUploadBudget{}
		b.observe(host, 429, budgetHeaders(now, 0, time.Hour), now)
		if b.Window != nil || b.wait(now) != 0 || b.Headers["RateLimit-Policy"] == "" {
			t.Errorf("%s: changed behavior or lost observation: %+v", host, b)
		}
	}
}

func TestImportUploadObservations(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	b := importUploadBudget{}
	b.observe(budgetTestPDS, 200, budgetHeaders(now, 900, time.Hour), now)
	if b.Window == nil || b.Window.Remaining != 900 {
		t.Fatal("successful response did not establish the budget")
	}
	b.observe(budgetTestPDS, 200, budgetHeaders(now, 850, time.Hour), now)
	b.observe(budgetTestPDS, 200, budgetHeaders(now, 899, time.Hour), now)
	if b.Window.Remaining != 850 {
		t.Fatal("out-of-order response refunded capacity")
	}
	h := budgetHeaders(now, 2999, 5*time.Minute)
	h.Set("RateLimit-Policy", "3000;w=300")
	h.Set("RateLimit-Limit", "3000")
	b.observe(budgetTestPDS, 200, h, now)
	b.observe(budgetTestPDS, 200, http.Header{}, now)
	if b.Window.Remaining != 850 || !b.Window.Reset.Equal(now.Add(time.Hour)) {
		t.Fatal("missing/global headers replaced daily budget")
	}
	h.Set("Retry-After", "600")
	b.observe(budgetTestPDS, 429, h, now)
	if b.RetryAt.Sub(now) != 601*time.Second {
		t.Fatal("global throttle was not honored separately")
	}
	// An upload after reset may discover the new budget, including manual
	// usage elsewhere. Never assume the next window still has 1000 available.
	now = now.Add(2 * time.Hour)
	b.observe(budgetTestPDS, 200, budgetHeaders(now, 650, 23*time.Hour), now)
	if b.Window.Remaining != 650 || b.wait(now) <= 0 {
		t.Fatal("new-window observation or reserve was lost")
	}
	b.Window.Reset = now.Add(-time.Second)
	b.observe(budgetTestPDS, 200, http.Header{}, now)
	if b.wait(now) < importRateLimitWait {
		t.Fatal("missing new-window headers allowed repeated discovery uploads")
	}
}

func TestImportUploadRetryHeaders(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	for _, retry := range []string{"120", now.Add(2 * time.Minute).UTC().Format(http.TimeFormat)} {
		h := http.Header{"Retry-After": {retry}}
		if got := importUploadRetry(h, now); got != 2*time.Minute {
			t.Errorf("%q: %s", retry, got)
		}
	}
	if got := importUploadRetry(http.Header{}, now); got != importRateLimitWait {
		t.Errorf("fallback: %s", got)
	}
}

func TestImportUploadWindowIdentityUsesPDSClock(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	b := importUploadBudget{}
	h := budgetHeaders(now, 100, time.Hour)
	b.observe(budgetTestPDS, 200, h, now)
	// A delayed response must not look like a new window and refund capacity.
	h.Set("RateLimit-Remaining", "900")
	b.observe(budgetTestPDS, 200, h, now.Add(10*time.Second))
	if b.Window.Remaining != 100 {
		t.Fatalf("response delay refunded budget: %d", b.Window.Remaining)
	}
}
