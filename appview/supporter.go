package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Supporter tier: semantic library search and find-similar-in-library are
// gated behind a Paddle subscription. Paddle POSTs subscription lifecycle
// events to /api/paddle/webhook, which mirrors them into paddle_subscription;
// the XRPC handlers gate on that mirror. The whole gate is switched on by
// setting PADDLE_WEBHOOK_SECRET — with it unset (dev, or pre-launch prod)
// every authenticated user counts as a supporter.

func (s *Server) isSupporter(ctx context.Context, did string) (bool, error) {
	if s.PaddleWebhookSecret == "" {
		return true, nil
	}
	return s.Store.HasSupporterSubscription(ctx, did)
}

// requireSupporter gates an XRPC handler: when the viewer isn't a supporter it
// writes a 403 SupporterRequired response and returns false.
func (s *Server) requireSupporter(w http.ResponseWriter, r *http.Request, did string) bool {
	ok, err := s.isSupporter(r.Context(), did)
	if err != nil {
		slog.Error("isSupporter", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "SupporterRequired", "message": "this feature is available to supporters"})
		return false
	}
	return true
}

// APISupporterStatus reports the viewer's entitlement. `subscribed` is the real
// Paddle subscription state (drives the settings UI); `active` is what the gate
// enforces (everyone, while the gate is disabled).
func (s *Server) APISupporterStatus(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	subscribed, err := s.Store.HasSupporterSubscription(r.Context(), did.String())
	if err != nil {
		slog.Error("HasSupporterSubscription", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	active := subscribed || s.PaddleWebhookSecret == ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"active": active, "subscribed": subscribed})
}

// APISupporterPortal mints a Paddle customer-portal session for the viewer and
// returns its overview URL — invoices, payment method, plan changes, and
// cancellation all happen there, so Currents needs no billing UI of its own.
func (s *Server) APISupporterPortal(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	if s.PaddleAPIKey == "" {
		http.Error(w, "paddle not configured", http.StatusServiceUnavailable)
		return
	}
	customerID, err := s.Store.GetPaddleCustomerID(r.Context(), did.String())
	if err != nil {
		slog.Error("GetPaddleCustomerID", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if customerID == "" {
		http.Error(w, "no subscription", http.StatusNotFound)
		return
	}
	portalURL, err := s.createPaddlePortalSession(r.Context(), customerID)
	if err != nil {
		slog.Error("createPaddlePortalSession", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": portalURL})
}

var paddleHTTP = &http.Client{Timeout: 10 * time.Second}

// createPaddlePortalSession calls the Paddle REST API. The environment is
// inferred from the API key prefix (pdl_sdbx_... = sandbox), so no separate
// environment setting exists to drift out of sync.
func (s *Server) createPaddlePortalSession(ctx context.Context, customerID string) (string, error) {
	base := "https://api.paddle.com"
	if strings.HasPrefix(s.PaddleAPIKey, "pdl_sdbx_") {
		base = "https://sandbox-api.paddle.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/customers/"+url.PathEscape(customerID)+"/portal-sessions", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.PaddleAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := paddleHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("paddle portal session: status %d: %.200s", resp.StatusCode, body)
	}
	var out struct {
		Data struct {
			URLs struct {
				General struct {
					Overview string `json:"overview"`
				} `json:"general"`
			} `json:"urls"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Data.URLs.General.Overview == "" {
		return "", fmt.Errorf("paddle portal session: no overview url in response")
	}
	return out.Data.URLs.General.Overview, nil
}

// paddleWebhookEvent is the subset of a Paddle notification we consume. Every
// subscription.* event carries the full subscription entity, so one shape
// serves created/updated/canceled/paused/resumed alike. custom_data.did is set
// by the frontend when it opens checkout and maps the subscription to a user.
type paddleWebhookEvent struct {
	EventType string `json:"event_type"`
	Data      struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		CustomerID string `json:"customer_id"`
		CustomData struct {
			DID string `json:"did"`
		} `json:"custom_data"`
		Items []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"items"`
		ScheduledChange *struct {
			EffectiveAt time.Time `json:"effective_at"`
		} `json:"scheduled_change"`
	} `json:"data"`
}

// PaddleWebhook ingests Paddle notifications. Paddle treats any non-2xx as a
// failed delivery and retries with the same payload for up to ~3 days, so
// transient failures answer non-2xx and unusable-but-valid events are acked.
func (s *Server) PaddleWebhook(w http.ResponseWriter, r *http.Request) {
	if s.PaddleWebhookSecret == "" {
		http.Error(w, "paddle not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	if !verifyPaddleSignature(r.Header.Get("Paddle-Signature"), body, s.PaddleWebhookSecret) {
		// Non-2xx so Paddle retries: a rotated secret then recovers on redeploy,
		// and a genuinely forged request retrying is harmless.
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var ev paddleWebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(ev.EventType, "subscription.") {
		w.WriteHeader(http.StatusOK)
		return
	}
	if ev.Data.CustomData.DID == "" {
		// Nothing to map the subscription to; retrying can't fix that.
		slog.Warn("paddle subscription without did in custom_data", "subscription", ev.Data.ID, "event", ev.EventType)
		w.WriteHeader(http.StatusOK)
		return
	}

	sub := PaddleSubscription{
		SubscriptionID: ev.Data.ID,
		DID:            ev.Data.CustomData.DID,
		CustomerID:     ev.Data.CustomerID,
		Status:         ev.Data.Status,
	}
	if len(ev.Data.Items) > 0 {
		sub.PriceID = ev.Data.Items[0].Price.ID
	}
	if ev.Data.ScheduledChange != nil {
		sub.ScheduledChange = &ev.Data.ScheduledChange.EffectiveAt
	}
	if err := s.Store.UpsertPaddleSubscription(r.Context(), sub); err != nil {
		slog.Error("UpsertPaddleSubscription", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("paddle subscription synced", "subscription", sub.SubscriptionID, "did", sub.DID, "status", sub.Status)
	w.WriteHeader(http.StatusOK)
}

// verifyPaddleSignature checks a Paddle-Signature header ("ts=...;h1=..."):
// HMAC-SHA256 over "<ts>:<raw body>" keyed with the notification destination
// secret. Multiple h1 values appear during secret rotation — any match passes.
// The timestamp bound rejects replayed deliveries.
func verifyPaddleSignature(header string, body []byte, secret string) bool {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ";") {
		if v, ok := strings.CutPrefix(part, "ts="); ok {
			ts = v
		} else if v, ok := strings.CutPrefix(part, "h1="); ok {
			sigs = append(sigs, v)
		}
	}
	if ts == "" || len(sigs) == 0 {
		return false
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	if d := time.Since(time.Unix(sec, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte(":"))
	mac.Write(body)
	expected := mac.Sum(nil)
	for _, s := range sigs {
		if sig, err := hex.DecodeString(s); err == nil && hmac.Equal(expected, sig) {
			return true
		}
	}
	return false
}
