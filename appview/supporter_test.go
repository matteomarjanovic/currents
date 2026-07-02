package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func signPaddle(ts string, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s:%s", ts, body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyPaddleSignature(t *testing.T) {
	secret := "pdl_ntfset_test_secret"
	body := []byte(`{"event_type":"subscription.created","data":{"id":"sub_1"}}`)
	now := fmt.Sprintf("%d", time.Now().Unix())
	old := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	cases := []struct {
		name   string
		header string
		body   []byte
		want   bool
	}{
		{"valid", "ts=" + now + ";h1=" + signPaddle(now, body, secret), body, true},
		{"tampered body", "ts=" + now + ";h1=" + signPaddle(now, body, secret), []byte(`{"data":{}}`), false},
		{"wrong secret", "ts=" + now + ";h1=" + signPaddle(now, body, "other"), body, false},
		{"expired timestamp", "ts=" + old + ";h1=" + signPaddle(old, body, secret), body, false},
		{"rotation: second h1 valid", "ts=" + now + ";h1=" + signPaddle(now, body, "other") + ";h1=" + signPaddle(now, body, secret), body, true},
		{"missing h1", "ts=" + now, body, false},
		{"empty header", "", body, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := verifyPaddleSignature(c.header, c.body, secret); got != c.want {
				t.Errorf("verifyPaddleSignature() = %v, want %v", got, c.want)
			}
		})
	}
}
