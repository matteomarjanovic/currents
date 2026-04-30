package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type backgroundMetricsResponse struct {
	Now         string         `json:"now"`
	ProcessMode string         `json:"processMode"`
	Saves       map[string]any `json:"saves"`
}

func (s *Server) BackgroundStatus(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.Store.GetBackgroundMetrics(r.Context())
	if err != nil {
		http.Error(w, "could not load background metrics", http.StatusInternalServerError)
		return
	}

	response := backgroundMetricsResponse{
		Now:         time.Now().UTC().Format(time.RFC3339),
		ProcessMode: s.ProcessMode,
		Saves: map[string]any{
			"missingVisualIdentityCount":       metrics.Saves.MissingVisualIdentityCount,
			"distinctMissingBlobCidCount":      metrics.Saves.DistinctMissingBlobCIDCount,
			"collectionsMissingEmbeddingCount": metrics.Saves.CollectionsMissingEmbeddingCount,
		},
	}
	if metrics.Saves.OldestMissingCreatedAt != nil {
		response.Saves["oldestMissingCreatedAt"] = metrics.Saves.OldestMissingCreatedAt.UTC().Format(time.RFC3339)
		age := int64(time.Since(*metrics.Saves.OldestMissingCreatedAt).Seconds())
		response.Saves["oldestMissingAgeSec"] = age
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func runRepairPass(ctx context.Context, handler *TapHandler) (RepairStats, error) {
	blobCIDs, err := handler.Store.ListMissingVisualIdentityBlobCIDs(ctx)
	if err != nil {
		return RepairStats{}, err
	}
	report := RepairStats{BlobCandidates: int64(len(blobCIDs))}
	for _, blobCID := range blobCIDs {
		if err := processBlobEnrichment(ctx, handler, blobCID); err != nil {
			slog.Warn("repair blob enrichment failed", "blob_cid", blobCID, "err", err)
			continue
		}
		report.BlobEnriched++
	}

	collections, err := handler.Store.ListCollectionsMissingEmbedding(ctx)
	if err != nil {
		return report, err
	}
	report.CollectionCandidates = int64(len(collections))
	for _, collectionURI := range collections {
		if err := recomputeCollectionEmbedding(ctx, handler.Store, collectionURI); err != nil {
			slog.Warn("repair collection embedding failed", "collection_uri", collectionURI, "err", err)
			continue
		}
		report.CollectionsRecomputed++
	}
	return report, nil
}
