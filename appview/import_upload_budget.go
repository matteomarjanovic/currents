package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/jackc/pgx/v5"
)

const importUploadSpacing = 6 * time.Second

type importBlobWindow struct {
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
	PDSReset  int64     `json:"pdsReset"`
}

// This is an observation of the PDS, not a count of completed save records.
// Keep the raw rate headers for all providers; only the verified Bluesky
// daily, DID-scoped policy controls imports. Missing headers are not unlimited.
type importUploadBudget struct {
	Window      *importBlobWindow `json:"window,omitempty"`
	LastAttempt time.Time         `json:"lastAttempt"`
	RetryAt     time.Time         `json:"retryAt"`
	ManualUntil time.Time         `json:"manualUntil"`
	ObservedAt  time.Time         `json:"observedAt"`
	Status      int               `json:"status"`
	Headers     map[string]string `json:"headers,omitempty"`
}

func blueskyUploadHost(pds string) bool {
	u, err := url.Parse(pds)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" &&
		strings.HasSuffix(u.Hostname(), ".host.bsky.network") &&
		u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

// Linear release: 1000 at 24h, 200 at 1h, 50 at 5m. The last 50 never
// become import capacity in this window. Round up to preserve the reserve.
func importUploadReserve(left time.Duration) int {
	if left > time.Hour {
		return min(1000, 200+int(math.Ceil(800*float64(left-time.Hour)/float64(23*time.Hour))))
	}
	if left > 5*time.Minute {
		return 50 + int(math.Ceil(150*float64(left-5*time.Minute)/float64(55*time.Minute)))
	}
	return 50
}

func (b importUploadBudget) wait(now time.Time) time.Duration {
	// A host name alone is insufficient: learn the daily policy from a real
	// upload before changing the provider's existing import behavior.
	if b.Window == nil {
		return 0
	}
	wait := max(time.Duration(0), b.RetryAt.Sub(now), b.ManualUntil.Sub(now), b.LastAttempt.Add(importUploadSpacing).Sub(now))
	left := b.Window.Reset.Sub(now)
	if left <= 0 || b.Window.Remaining-1 >= importUploadReserve(left) {
		return wait
	}
	// Invert the reserve curve to find when one more upload fits. Polling is
	// capped by the worker so manual activity and account migration are noticed.
	remaining := b.Window.Remaining - 1
	var release time.Duration
	if remaining >= 200 {
		release = time.Hour + time.Duration(float64(remaining-200)*float64(23*time.Hour)/800)
	} else if remaining >= 50 {
		release = 5*time.Minute + time.Duration(float64(remaining-50)*float64(55*time.Minute)/150)
	}
	return max(wait, left-release+time.Second)
}

// RateLimit-Reset in the reference PDS is epoch seconds. Use the PDS Date
// header to tolerate host clock skew. Never mistake a shorter global limit
// for the blob window, even when it is returned by uploadBlob.
func parseImportBlobWindow(h http.Header, now time.Time) *importBlobWindow {
	if strings.ReplaceAll(h.Get("RateLimit-Policy"), " ", "") != "1000;w=86400" || h.Get("RateLimit-Limit") != "1000" {
		return nil
	}
	remaining, err := strconv.Atoi(h.Get("RateLimit-Remaining"))
	if err != nil || remaining < 0 || remaining > 1000 {
		return nil
	}
	reset, err := strconv.ParseInt(h.Get("RateLimit-Reset"), 10, 64)
	if err != nil {
		return nil
	}
	serverNow, err := http.ParseTime(h.Get("Date"))
	if err != nil {
		serverNow = now
	}
	left := time.Unix(reset, 0).Sub(serverNow)
	if left <= 0 || left > 24*time.Hour {
		return nil
	}
	return &importBlobWindow{Remaining: remaining, Reset: now.Add(left), PDSReset: reset}
}

func importUploadRetry(h http.Header, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(h.Get("Retry-After"), 10, 64); err == nil && seconds > 0 && seconds <= 86400 {
		return time.Duration(seconds) * time.Second
	}
	serverNow, err := http.ParseTime(h.Get("Date"))
	if err != nil {
		serverNow = now
	}
	if retry, err := http.ParseTime(h.Get("Retry-After")); err == nil && retry.After(serverNow) && retry.Sub(serverNow) <= 24*time.Hour {
		return retry.Sub(serverNow)
	}
	if reset, err := strconv.ParseInt(h.Get("RateLimit-Reset"), 10, 64); err == nil {
		if delay := time.Unix(reset, 0).Sub(serverNow); delay > 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	return importRateLimitWait
}

func (b *importUploadBudget) observe(pds string, status int, h http.Header, now time.Time) {
	b.ObservedAt, b.Status = now, status
	b.Headers = make(map[string]string)
	for _, key := range []string{"RateLimit-Policy", "RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "Retry-After", "Date"} {
		if value := h.Get(key); value != "" {
			b.Headers[key] = value[:min(len(value), 256)]
		}
	}
	if !blueskyUploadHost(pds) {
		return
	}
	// Authentication runs before the PDS route limiter. Its challenge headers
	// describe no blob admission, and the SDK may immediately retry with a
	// refreshed token/nonce, including on the first request after reset.
	if status == http.StatusUnauthorized {
		return
	}
	if window := parseImportBlobWindow(h, now); window != nil {
		if b.Window == nil || window.PDSReset > b.Window.PDSReset+2 {
			b.Window = window
		} else if delta := window.PDSReset - b.Window.PDSReset; delta >= -2 && delta <= 2 {
			// Concurrent/manual responses may arrive out of order. Never refund
			// a local reservation or increase capacity within the same window.
			b.Window.Remaining = min(b.Window.Remaining, window.Remaining)
		}
	} else if b.Window != nil && !b.Window.Reset.After(now) {
		// One real upload may discover a new window after reset. If the
		// headers disappear, do not turn that into a stream of probes.
		b.RetryAt = now.Add(importRateLimitWait)
	}
	if status == http.StatusTooManyRequests && b.Window != nil {
		b.RetryAt = now.Add(importUploadRetry(h, now) + time.Second)
	}
}

func (m *PgStore) changeImportUploadBudget(ctx context.Context, did, pds string, change func(*importUploadBudget)) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO import_upload_budget (owner_did,pds_url) VALUES ($1,$2) ON CONFLICT DO NOTHING`, did, pds); err != nil {
		return err
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT state FROM import_upload_budget WHERE owner_did=$1 AND pds_url=$2 FOR UPDATE`, did, pds).Scan(&raw); err != nil {
		return err
	}
	var budget importUploadBudget
	if err = json.Unmarshal(raw, &budget); err != nil {
		return err
	}
	change(&budget)
	raw, err = json.Marshal(budget)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE import_upload_budget SET state=$3 WHERE owner_did=$1 AND pds_url=$2`, did, pds, raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (m *PgStore) importUploadWait(ctx context.Context, did, pds string) (time.Duration, error) {
	if !blueskyUploadHost(pds) {
		return 0, nil
	}
	var raw []byte
	err := m.pool.QueryRow(ctx, `SELECT state FROM import_upload_budget WHERE owner_did=$1 AND pds_url=$2`, did, pds).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var budget importUploadBudget
	if err := json.Unmarshal(raw, &budget); err != nil {
		return 0, err
	}
	return budget.wait(time.Now()), nil
}

type importUploadWaitError struct{ delay time.Duration }

func (e *importUploadWaitError) Error() string { return "waiting for import upload capacity" }

// Observe at the HTTP boundary so the SDK retains its usual body/error parsing
// and OAuth behavior. Clone the client: never mutate the shared OAuth transport.
func withUploadBudget(c *atclient.APIClient, store *PgStore, did string, importing bool) *atclient.APIClient {
	if store == nil {
		return c
	}
	copy := *c
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	httpClient := *client
	next := client.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	httpClient.Transport = &uploadBudgetTransport{next: next, store: store, did: did, pds: strings.TrimRight(c.Host, "/"), importing: importing}
	copy.Client = &httpClient
	return &copy
}

type uploadBudgetTransport struct {
	next      http.RoundTripper
	store     *PgStore
	did, pds  string
	importing bool
}

func (t *uploadBudgetTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	upload := req.Method == "POST" && req.URL.Path == "/xrpc/com.atproto.repo.uploadBlob"
	token := req.URL.Path == "/xrpc/com.atproto.server.getServiceAuth" && req.URL.Query().Get("lxm") == "com.atproto.repo.uploadBlob"
	if req.URL.Scheme+"://"+req.URL.Host != t.pds || (!upload && !token) {
		return t.next.RoundTrip(req)
	}
	var charged bool
	var reservationReset int64
	var attempt, previousAttempt time.Time
	if blueskyUploadHost(t.pds) {
		var wait time.Duration
		err := t.store.changeImportUploadBudget(req.Context(), t.did, t.pds, func(b *importUploadBudget) {
			now := time.Now()
			if t.importing {
				wait = b.wait(now)
				if wait > 0 {
					return
				}
				previousAttempt, attempt = b.LastAttempt, now
				b.LastAttempt = attempt
			} else {
				// Let manual activity go first, including during token auth
				// retries. Charge a direct upload only after a token is issued.
				b.ManualUntil = now.Add(2 * time.Minute)
			}
			if upload && b.Window != nil && b.Window.Reset.After(now) {
				charged, reservationReset = b.Window.Remaining > 0, b.Window.PDSReset
				b.Window.Remaining = max(0, b.Window.Remaining-1)
			}
		})
		if err != nil {
			slog.Warn("reserving import upload capacity", "did", t.did, "err", err)
			if t.importing {
				if req.Body != nil {
					req.Body.Close()
				}
				return nil, &importUploadWaitError{importBudgetRecheck}
			}
		}
		if wait > 0 {
			if req.Body != nil {
				req.Body.Close()
			}
			return nil, &importUploadWaitError{wait}
		}
	}
	resp, err := t.next.RoundTrip(req)
	if resp != nil && token && resp.StatusCode >= 200 && resp.StatusCode < 300 && blueskyUploadHost(t.pds) {
		if saveErr := t.store.changeImportUploadBudget(req.Context(), t.did, t.pds, func(b *importUploadBudget) {
			if b.Window != nil && b.Window.Reset.After(time.Now()) {
				b.Window.Remaining = max(0, b.Window.Remaining-1)
			}
		}); saveErr != nil {
			slog.Warn("reserving direct upload capacity", "did", t.did, "err", saveErr)
		}
	}
	if resp != nil && upload {
		if saveErr := t.store.changeImportUploadBudget(req.Context(), t.did, t.pds, func(b *importUploadBudget) {
			if resp.StatusCode == http.StatusUnauthorized && blueskyUploadHost(t.pds) {
				// Refund only our reservation, preserving concurrent manual
				// activity. A nonce/token challenge never reached uploadBlob's
				// limiter; its immediate SDK retry belongs to this same attempt.
				if charged && b.Window != nil && b.Window.PDSReset == reservationReset {
					b.Window.Remaining = min(1000, b.Window.Remaining+1)
				}
				if t.importing && b.LastAttempt.Equal(attempt) {
					b.LastAttempt = previousAttempt
				}
			}
			b.observe(t.pds, resp.StatusCode, resp.Header, time.Now())
		}); saveErr != nil {
			slog.Warn("recording PDS upload limit", "did", t.did, "pds", t.pds, "err", saveErr)
		}
	}
	return resp, err
}
