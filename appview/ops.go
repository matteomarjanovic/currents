package main

import (
	"context"
	"log/slog"
)

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
