package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/polarsource/polar-go/models/components"
)

func signPolar(id, ts string, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s.%s.%s", id, ts, body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func polarHeaders(id, ts, sig string) http.Header {
	h := http.Header{}
	h.Set("webhook-id", id)
	h.Set("webhook-timestamp", ts)
	h.Set("webhook-signature", sig)
	return h
}

func TestVerifyPolarSignature(t *testing.T) {
	secret := "polar_whs_test_secret"
	body := []byte(`{"type":"subscription.created","data":{"id":"sub_1"}}`)
	id := "msg_2y4x"
	now := fmt.Sprintf("%d", time.Now().Unix())
	old := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	cases := []struct {
		name    string
		headers http.Header
		body    []byte
		want    bool
	}{
		{"valid", polarHeaders(id, now, "v1,"+signPolar(id, now, body, secret)), body, true},
		{"tampered body", polarHeaders(id, now, "v1,"+signPolar(id, now, body, secret)), []byte(`{"data":{}}`), false},
		{"wrong secret", polarHeaders(id, now, "v1,"+signPolar(id, now, body, "other")), body, false},
		{"wrong id", polarHeaders("msg_other", now, "v1,"+signPolar(id, now, body, secret)), body, false},
		{"expired timestamp", polarHeaders(id, old, "v1,"+signPolar(id, old, body, secret)), body, false},
		{"rotation: second sig valid", polarHeaders(id, now, "v1,"+signPolar(id, now, body, "other")+" v1,"+signPolar(id, now, body, secret)), body, true},
		{"unknown version ignored", polarHeaders(id, now, "v2,"+signPolar(id, now, body, secret)), body, false},
		{"missing signature", polarHeaders(id, now, ""), body, false},
		{"missing headers", http.Header{}, body, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := verifyPolarSignature(c.headers, c.body, secret); got != c.want {
				t.Errorf("verifyPolarSignature() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestColorTrialSpent(t *testing.T) {
	// Colors already spent by the viewer: a red, a teal and a near-black.
	spent := []string{"#e63946", "#2a9d8f", "#111111"}

	cases := []struct {
		name string
		hex  string
		want bool
	}{
		{"exact match", "#e63946", true},
		{"different case", "#E63946", true},
		{"one channel nudged", "#e63947", true},
		{"imperceptibly lighter", "#e73a47", true},
		{"visibly different red", "#c1121f", false},
		{"other spent color", "#2a9d8f", true},
		{"unrelated blue", "#457b9d", false},
		{"near-black vs black", "#000000", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lab, err := hexToLab(c.hex)
			if err != nil {
				t.Fatalf("hexToLab(%q): %v", c.hex, err)
			}
			if got := colorTrialSpent(spent, lab); got != c.want {
				t.Errorf("colorTrialSpent(%q) = %v, want %v", c.hex, got, c.want)
			}
		})
	}

	t.Run("no colors spent", func(t *testing.T) {
		lab, _ := hexToLab("#e63946")
		if colorTrialSpent(nil, lab) {
			t.Error("colorTrialSpent(nil, ...) = true, want false")
		}
	})
}

func TestNormalizeHex(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"#e63946", "#e63946"},
		{"E63946", "#e63946"},
		{"#E63946", "#e63946"},
	} {
		if got := normalizeHex(c.in); got != c.want {
			t.Errorf("normalizeHex(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestComplimentaryDiscount(t *testing.T) {
	full := components.CreateSubscriptionDiscountDiscountPercentageOnceForeverDurationBase(
		components.DiscountPercentageOnceForeverDurationBase{BasisPoints: 10000},
	)
	partial := components.CreateSubscriptionDiscountDiscountPercentageOnceForeverDurationBase(
		components.DiscountPercentageOnceForeverDurationBase{BasisPoints: 2500},
	)
	if !complimentaryDiscount(&full) {
		t.Error("100% discount should be complimentary")
	}
	if complimentaryDiscount(&partial) {
		t.Error("partial discount should not be complimentary")
	}
	if complimentaryDiscount(nil) {
		t.Error("no discount should not be complimentary")
	}
}

func TestPaidSupporterStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	for _, sub := range []PolarSubscription{
		{SubscriptionID: "sub_paid_monthly", DID: "did:plc:paid", Status: "active", ProductID: "monthly"},
		{SubscriptionID: "sub_paid_yearly", DID: "did:plc:paid", Status: "active", ProductID: "yearly"},
		{SubscriptionID: "sub_complimentary", DID: "did:plc:free", Status: "active", ProductID: "monthly", Complimentary: true},
	} {
		if err := store.UpsertPolarSubscription(ctx, sub); err != nil {
			t.Fatalf("UpsertPolarSubscription(%s): %v", sub.SubscriptionID, err)
		}
	}

	count, err := store.CountSupporters(ctx)
	if err != nil {
		t.Fatalf("CountSupporters: %v", err)
	}
	if count != 1 {
		t.Errorf("CountSupporters = %d, want 1 distinct paid supporter", count)
	}
	byProduct, err := store.CountSupportersByProduct(ctx)
	if err != nil {
		t.Fatalf("CountSupportersByProduct: %v", err)
	}
	if byProduct["monthly"] != 1 || byProduct["yearly"] != 1 {
		t.Errorf("CountSupportersByProduct = %#v, want one paid subscription for each plan", byProduct)
	}
}

// The gate itself, against a real ledger: five distinct colors are free, a
// sixth is not, and colors already spent stay free forever (which is what
// keeps pagination and text refinement off the meter). DB-backed; skips
// without TEST_DATABASE_URL.
func TestRequireColorSearch(t *testing.T) {
	store := newTestStore(t)
	// A webhook secret switches the gate on; without one everyone is a supporter.
	s := &Server{Store: store, PolarWebhookSecret: "polar_whs_test_secret"}
	const did = "did:plc:trialgate"

	call := func(t *testing.T, hex string) (bool, int) {
		t.Helper()
		lab, err := hexToLab(hex)
		if err != nil {
			t.Fatalf("hexToLab(%q): %v", hex, err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/xrpc/is.currents.feed.searchSavesByColor?color="+hex, nil)
		return s.requireColorSearch(rec, req, did, hex, lab), rec.Code
	}

	// The whole allowance, spent one distinct color at a time.
	for _, hex := range []string{"#e63946", "#2a9d8f", "#457b9d", "#e9c46a", "#7b2cbf"} {
		if ok, code := call(t, hex); !ok {
			t.Fatalf("requireColorSearch(%s) = false (status %d), want allowed", hex, code)
		}
	}

	// Paginating and refining that first search cost nothing, and neither does
	// a shade the eye can't tell apart from one already spent.
	for _, hex := range []string{"#e63946", "#E63946", "#e63947"} {
		if ok, _ := call(t, hex); !ok {
			t.Errorf("requireColorSearch(%s) = false, want free (already spent)", hex)
		}
	}

	// A sixth genuinely new color is where the paywall lands.
	ok, code := call(t, "#111111")
	if ok {
		t.Fatal("requireColorSearch(#111111) = true, want denied after the allowance")
	}
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", code, http.StatusForbidden)
	}

	// A denial must not have been charged: the ledger still holds exactly the
	// allowance, so the viewer can keep browsing the colors they did spend.
	spent, err := store.ColorTrialColors(context.Background(), did)
	if err != nil {
		t.Fatalf("ColorTrialColors: %v", err)
	}
	if len(spent) != colorTrialLimit {
		t.Errorf("spent %d colors, want %d (%v)", len(spent), colorTrialLimit, spent)
	}

	// Subscribing lifts the ceiling without touching the ledger.
	if err := store.UpsertPolarSubscription(context.Background(), PolarSubscription{
		SubscriptionID: "sub_trial", DID: did, CustomerID: "cus_1", Status: "active", ProductID: "prod_1",
	}); err != nil {
		t.Fatalf("UpsertPolarSubscription: %v", err)
	}
	if ok, code := call(t, "#111111"); !ok {
		t.Fatalf("requireColorSearch(#111111) as supporter = false (status %d), want allowed", code)
	}
}
