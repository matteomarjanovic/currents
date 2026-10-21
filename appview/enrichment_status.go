package main

import (
	"context"
	"errors"
	"fmt"
)

var errUnsupportedMedia = errors.New("unsupported media")

// Preserve the HTTP status so enrichment can distinguish invalid content from
// a failed request without parsing log messages.
type remoteHTTPError struct {
	operation string
	status    int
	body      string
}

func (e *remoteHTTPError) Error() string {
	message := fmt.Sprintf("%s returned %d", e.operation, e.status)
	if e.body != "" {
		message += ": " + e.body
	}
	return message
}

func enrichmentFailureKind(err error) string {
	if errors.Is(err, errUnsupportedMedia) {
		return "unsupported"
	}
	var response *remoteHTTPError
	if errors.As(err, &response) {
		switch response.operation {
		case "blob fetch":
			if response.status == 400 || response.status == 404 || response.status == 410 {
				return "source_unavailable"
			}
		case "inference server":
			if response.status == 400 || response.status == 413 {
				return "invalid"
			}
			if response.status == 415 {
				return "unsupported"
			}
		}
	}
	return "failed"
}

func (m *PgStore) RecordBlobEnrichmentFailure(ctx context.Context, blobCID, kind, message string) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO blob_enrichment_failure (blob_cid, kind, error)
		VALUES ($1, $2, $3)
		ON CONFLICT (blob_cid) DO UPDATE
		SET kind = EXCLUDED.kind, error = EXCLUDED.error, failed_at = now()
	`, blobCID, kind, message)
	return err
}

func (m *PgStore) ClearBlobEnrichmentFailure(ctx context.Context, blobCID string) error {
	_, err := m.pool.Exec(ctx, `DELETE FROM blob_enrichment_failure WHERE blob_cid = $1`, blobCID)
	return err
}
