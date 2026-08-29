package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

type adminOverviewAppview struct {
	HeapBytes   uint64           `json:"heapBytes"`
	SystemBytes uint64           `json:"systemBytes"`
	Goroutines  int              `json:"goroutines"`
	Pool        AdminPoolMetrics `json:"pool"`
}

type adminOverviewInference struct {
	Available bool             `json:"available"`
	Health    *InferenceHealth `json:"health,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type adminOverviewBackground struct {
	MissingVisualIdentityCount       int64  `json:"missingVisualIdentityCount"`
	DistinctMissingBlobCIDCount      int64  `json:"distinctMissingBlobCidCount"`
	CollectionsMissingEmbeddingCount int64  `json:"collectionsMissingEmbeddingCount"`
	OldestMissingAgeSec              *int64 `json:"oldestMissingAgeSec,omitempty"`
}

type adminOverviewResponse struct {
	Now        time.Time                `json:"now"`
	Appview    adminOverviewAppview     `json:"appview"`
	Database   AdminDatabaseMetrics     `json:"database"`
	Inference  adminOverviewInference   `json:"inference"`
	Background adminOverviewBackground  `json:"background"`
	Jobs       []OperationsJobRun       `json:"jobs"`
	Hosts      []OperationsHostSnapshot `json:"hosts"`
}

func (s *Server) APIAdminOverview(w http.ResponseWriter, r *http.Request) {
	database, err := s.Store.AdminDatabaseMetrics(r.Context())
	if err != nil {
		slog.Error("AdminDatabaseMetrics", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	background, err := s.Store.GetBackgroundMetrics(r.Context())
	if err != nil {
		slog.Error("GetBackgroundMetrics", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	jobs, err := s.Store.LatestOperationsJobRuns(r.Context())
	if err != nil {
		slog.Error("LatestOperationsJobRuns", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	hosts, err := s.Store.LatestOperationsHostSnapshots(r.Context())
	if err != nil {
		slog.Error("LatestOperationsHostSnapshots", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	response := adminOverviewResponse{
		Now: time.Now().UTC(),
		Appview: adminOverviewAppview{
			HeapBytes:   mem.HeapAlloc,
			SystemBytes: mem.Sys,
			Goroutines:  runtime.NumGoroutine(),
			Pool:        s.Store.AdminPoolMetrics(),
		},
		Database: database,
		Background: adminOverviewBackground{
			MissingVisualIdentityCount:       background.Saves.MissingVisualIdentityCount,
			DistinctMissingBlobCIDCount:      background.Saves.DistinctMissingBlobCIDCount,
			CollectionsMissingEmbeddingCount: background.Saves.CollectionsMissingEmbeddingCount,
		},
		Jobs:  jobs,
		Hosts: hosts,
	}
	if background.Saves.OldestMissingCreatedAt != nil {
		age := int64(time.Since(*background.Saves.OldestMissingCreatedAt).Seconds())
		response.Background.OldestMissingAgeSec = &age
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if health, err := s.Inference.Health(ctx); err != nil {
		response.Inference.Error = err.Error()
	} else {
		response.Inference.Available = true
		response.Inference.Health = &health
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
