package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
	"time"
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
