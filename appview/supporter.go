package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/polarsource/polar-go/models/components"
	"github.com/polarsource/polar-go/models/operations"
)

// Supporter tier: semantic library search and find-similar-in-library are
// gated behind a Polar subscription. Polar POSTs subscription lifecycle
// events to /api/polar/webhook, which mirrors them into polar_subscription;
// the XRPC handlers gate on that mirror. The whole gate is switched on by
// setting POLAR_WEBHOOK_SECRET — with it unset (dev, or pre-launch prod)
// every authenticated user counts as a supporter.

func (s *Server) isSupporter(ctx context.Context, did string) (bool, error) {
	if s.PolarWebhookSecret == "" {
		return true, nil
	}
	return s.Store.HasSupporterSubscription(ctx, did)
}

// Color search is the one supporter feature non-supporters can sample: a
// lifetime allowance of distinct query colors. The ledger keys on the color
// rather than on the request, so paginating a result set, adding a text query
// or narrowing the scope all reuse a color already spent instead of costing
// another one.
const colorTrialLimit = 5

// colorTrialSameDE groups near-identical query colors: two picks within this
// ΔE76 are the same color to the eye, so nudging the picker by a shade doesn't
// cost a second trial. Far below colorMaxDeltaE, which is what makes a stored
// palette color a *match* — this only decides what counts as the same query.
const colorTrialSameDE = 5.0

// colorTrialSpent reports whether the query color is close enough to one the
// viewer already spent to come for free.
func colorTrialSpent(spent []string, lab [3]float32) bool {
	for _, hex := range spent {
		other, err := hexToLab(hex)
		if err != nil {
			continue
		}
		dL := float64(lab[0] - other[0])
		dA := float64(lab[1] - other[1])
		dB := float64(lab[2] - other[2])
		if math.Sqrt(dL*dL+dA*dA+dB*dB) <= colorTrialSameDE {
			return true
		}
	}
	return false
}

// requireColorSearch gates color search. Supporters pass; everyone else spends
// from their trial allowance, which is charged here rather than in the handler
// so an unparseable color can't cost a trial. Denial reuses the same 403
// SupporterRequired the client already handles.
func (s *Server) requireColorSearch(w http.ResponseWriter, r *http.Request, did, hex string, lab [3]float32) bool {
	ok, err := s.isSupporter(r.Context(), did)
	if err != nil {
		slog.Error("isSupporter", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if ok {
		return true
	}

	spent, err := s.Store.ColorTrialColors(r.Context(), did)
	if err != nil {
		slog.Error("ColorTrialColors", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if colorTrialSpent(spent, lab) {
		return true
	}
	if len(spent) >= colorTrialLimit {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "SupporterRequired", "message": "you've used your free color searches"})
		return false
	}
	if err := s.Store.RecordColorTrial(r.Context(), did, normalizeHex(hex)); err != nil {
		slog.Error("RecordColorTrial", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	return true
}

// normalizeHex canonicalizes a validated hex color for storage, so the same
// color typed two ways is one row.
func normalizeHex(hex string) string {
	return "#" + strings.ToLower(strings.TrimPrefix(hex, "#"))
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
// Polar subscription state (drives the settings UI); `active` is what the gate
// enforces (everyone, while the gate is disabled). `colorTrialsLeft` is what's
// left of the color-search trial allowance — only meaningful, and only
// computed, when the gate would otherwise turn the viewer away.
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
	active := subscribed || s.PolarWebhookSecret == ""
	colorTrialsLeft := 0
	if !active {
		spent, err := s.Store.ColorTrialColors(r.Context(), did.String())
		if err != nil {
			slog.Error("ColorTrialColors", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		colorTrialsLeft = max(0, colorTrialLimit-len(spent))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"active":          active,
		"subscribed":      subscribed,
		"colorTrialsLeft": colorTrialsLeft,
	})
}

// APISupporterCheckout creates a Polar checkout session for the requested
// product and returns its URL for the embedded checkout. external_customer_id
// carries the viewer's DID, which Polar echoes back on every subscription
// webhook — that's the whole user ↔ subscription mapping.
func (s *Server) APISupporterCheckout(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	if s.Polar == nil {
		http.Error(w, "polar not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Product string `json:"product"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Product == "" {
		http.Error(w, "missing product", http.StatusBadRequest)
		return
	}
	externalID := did.String()
	embedOrigin := strings.TrimRight(s.FrontendURL, "/")
	res, err := s.Polar.Checkouts.Create(r.Context(), components.CheckoutCreate{
		Products:           []string{body.Product},
		ExternalCustomerID: &externalID,
		EmbedOrigin:        &embedOrigin,
	})
	if err != nil {
		slog.Error("polar checkout create", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": res.Checkout.URL})
}

// APISupporterPortal mints a Polar customer-portal session for the viewer and
// returns its URL — invoices, payment method, plan changes, and cancellation
// all happen there, so Currents needs no billing UI of its own.
func (s *Server) APISupporterPortal(w http.ResponseWriter, r *http.Request) {
	did, _, _ := s.currentSessionDID(r)
	if did == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	if s.Polar == nil {
		http.Error(w, "polar not configured", http.StatusServiceUnavailable)
		return
	}
	customerID, err := s.Store.GetPolarCustomerID(r.Context(), did.String())
	if err != nil {
		slog.Error("GetPolarCustomerID", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if customerID == "" {
		http.Error(w, "no subscription", http.StatusNotFound)
		return
	}
	res, err := s.Polar.CustomerSessions.Create(r.Context(),
		operations.CreateCustomerSessionsCreateCustomerSessionCreateCustomerSessionCustomerIDCreate(
			components.CustomerSessionCustomerIDCreate{CustomerID: customerID}))
	if err != nil {
		slog.Error("polar customer session create", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": res.CustomerSession.CustomerPortalURL})
}

// APISupporterStats returns the public transparency numbers for the support
// page: total indexed users and active supporter counts per product id. No
// auth — publishing these openly is the point.
func (s *Server) APISupporterStats(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.CountUsers(r.Context())
	if err != nil {
		slog.Error("CountUsers", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	byProduct, err := s.Store.CountSupportersByProduct(r.Context())
	if err != nil {
		slog.Error("CountSupportersByProduct", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	supporters := 0
	for _, n := range byProduct {
		supporters += n
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"totalUsers": users,
		"supporters": supporters,
		"byProduct":  byProduct,
	})
}

// polarWebhookEvent is the subset of a Polar webhook we consume. Every
// subscription.* event carries the full subscription entity, so one shape
// serves created/updated/active/canceled/uncanceled/revoked alike. The
// customer's external_id is the viewer's DID, set at checkout creation.
type polarWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		ID         string     `json:"id"`
		Status     string     `json:"status"`
		CustomerID string     `json:"customer_id"`
		ProductID  string     `json:"product_id"`
		EndsAt     *time.Time `json:"ends_at"`
		Customer   struct {
			ExternalID string `json:"external_id"`
		} `json:"customer"`
	} `json:"data"`
}

// PolarWebhook ingests Polar webhook deliveries. Polar treats any non-2xx as
// a failed delivery and retries with backoff (up to 10 attempts), so
// transient failures answer non-2xx and unusable-but-valid events are acked.
func (s *Server) PolarWebhook(w http.ResponseWriter, r *http.Request) {
	if s.PolarWebhookSecret == "" {
		http.Error(w, "polar not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	if !verifyPolarSignature(r.Header, body, s.PolarWebhookSecret) {
		// Non-2xx so Polar retries: a rotated secret then recovers on redeploy,
		// and a genuinely forged request retrying is harmless.
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var ev polarWebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(ev.Type, "subscription.") {
		w.WriteHeader(http.StatusOK)
		return
	}
	if ev.Data.Customer.ExternalID == "" {
		// Nothing to map the subscription to; retrying can't fix that.
		slog.Warn("polar subscription without external customer id", "subscription", ev.Data.ID, "event", ev.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	sub := PolarSubscription{
		SubscriptionID: ev.Data.ID,
		DID:            ev.Data.Customer.ExternalID,
		CustomerID:     ev.Data.CustomerID,
		Status:         ev.Data.Status,
		ProductID:      ev.Data.ProductID,
		EndsAt:         ev.Data.EndsAt,
	}
	if err := s.Store.UpsertPolarSubscription(r.Context(), sub); err != nil {
		slog.Error("UpsertPolarSubscription", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("polar subscription synced", "subscription", sub.SubscriptionID, "did", sub.DID, "status", sub.Status)
	w.WriteHeader(http.StatusOK)
}

// verifyPolarSignature checks a Standard Webhooks signature (what Polar
// uses): base64 HMAC-SHA256 over "<id>.<timestamp>.<raw body>" keyed with the
// endpoint secret as configured in the Polar dashboard. The webhook-signature
// header holds space-separated "v1,<sig>" entries — several during secret
// rotation, any match passes. The timestamp bound rejects replayed deliveries.
func verifyPolarSignature(h http.Header, body []byte, secret string) bool {
	id := h.Get("webhook-id")
	ts := h.Get("webhook-timestamp")
	if id == "" || ts == "" {
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
	fmt.Fprintf(mac, "%s.%s.%s", id, ts, body)
	expected := mac.Sum(nil)
	for _, part := range strings.Split(h.Get("webhook-signature"), " ") {
		v, ok := strings.CutPrefix(part, "v1,")
		if !ok {
			continue
		}
		if sig, err := base64.StdEncoding.DecodeString(v); err == nil && hmac.Equal(expected, sig) {
			return true
		}
	}
	return false
}
