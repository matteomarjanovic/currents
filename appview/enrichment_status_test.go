package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestEnrichmentFailureKind(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"video", fmt.Errorf("%w: video/mp4", errUnsupportedMedia), "unsupported"},
		{"invalid blob bytes", fmt.Errorf("%w: unrecognized image", errInvalidImage), "invalid"},
		{"missing blob", &remoteHTTPError{operation: "blob fetch", status: 400}, "source_unavailable"},
		{"missing repo wrapped by source fallback", fmt.Errorf("fetching source: %w", &remoteHTTPError{operation: "blob fetch", status: 404}), "source_unavailable"},
		{"removed source", &remoteHTTPError{operation: "blob fetch", status: 410}, "source_unavailable"},
		{"PDS rate limit", &remoteHTTPError{operation: "blob fetch", status: 429}, "failed"},
		{"PDS unavailable", &remoteHTTPError{operation: "blob fetch", status: 503}, "failed"},
		{"decode error", &remoteHTTPError{operation: "inference server", status: 400}, "invalid"},
		{"oversized image", &remoteHTTPError{operation: "inference server", status: 413}, "invalid"},
		{"unsupported type", &remoteHTTPError{operation: "inference server", status: 415}, "unsupported"},
		{"inference queue full", &remoteHTTPError{operation: "inference server", status: 503}, "failed"},
		{"network failure", errors.New("connection refused"), "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := enrichmentFailureKind(test.err); got != test.want {
				t.Fatalf("kind = %q; want %q", got, test.want)
			}
		})
	}
}
