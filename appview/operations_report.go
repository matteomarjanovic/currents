package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const operationsReportMaxAge = 5 * time.Minute

func (s *Server) APIOperationsHostSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.OpsReportingSecret == "" {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128<<10))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !validOperationsSignature(s.OpsReportingSecret, r.Header.Get("X-Currents-Ops-Timestamp"), r.Header.Get("X-Currents-Ops-Signature"), body) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var report struct {
		Host string `json:"host"`
	}
	if err := json.Unmarshal(body, &report); err != nil || (report.Host != "main" && report.Host != "inference") {
		http.Error(w, "invalid report", http.StatusBadRequest)
		return
	}
	if err := s.Store.UpsertOperationsHostSnapshot(r.Context(), report.Host, body); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validOperationsSignature(secret, timestamp, signature string, body []byte) bool {
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > operationsReportMaxAge {
		return false
	}
	encoded := strings.TrimPrefix(signature, "sha256=")
	provided, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte{'\n'})
	mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}
