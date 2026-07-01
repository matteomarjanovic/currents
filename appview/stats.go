package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type registrationBucketView struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// APIAdminStats returns registration analytics for the /stats dashboard: the
// total number of indexed users plus per-UTC-day new-registration buckets. The
// client re-buckets the daily series to week/month granularities and derives
// the cumulative trend, so one query serves every view.
func (s *Server) APIAdminStats(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.Store.RegistrationsByDay(r.Context())
	if err != nil {
		slog.Error("RegistrationsByDay", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	total := 0
	views := make([]registrationBucketView, len(buckets))
	for i, b := range buckets {
		views[i] = registrationBucketView{
			Date:  b.Date.UTC().Format(time.RFC3339),
			Count: b.Count,
		}
		total += b.Count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"totalUsers":    total,
		"registrations": views,
	})
}
