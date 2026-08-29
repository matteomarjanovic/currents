package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

func TestValidOperationsSignature(t *testing.T) {
	t.Parallel()
	secret := "test-secret"
	body := []byte(`{"host":"main"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n"))
	mac.Write(body)
	signature := "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !validOperationsSignature(secret, timestamp, signature, body) {
		t.Fatal("expected valid signature")
	}
	if validOperationsSignature(secret, timestamp, signature, []byte(`{"host":"inference"}`)) {
		t.Fatal("signature must bind the body")
	}
	if validOperationsSignature(secret, "0", signature, body) {
		t.Fatal("stale timestamp must fail")
	}
}
