package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInferenceClientEmbedImageParsesMetadata(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/image" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"embedding":[1,2],"umap_embedding":[3],"width":640,"height":480,"dominant_colors":[{"hex":"#112233","fraction":0.75}]}`)
	}))
	defer srv.Close()

	client := NewInferenceClient(srv.URL)
	result, err := client.EmbedImage(context.Background(), []byte("img"), "image/jpeg")
	if err != nil {
		t.Fatalf("EmbedImage returned error: %v", err)
	}
	if result.Width != 640 || result.Height != 480 {
		t.Fatalf("unexpected dimensions: %dx%d", result.Width, result.Height)
	}
	if got := string(result.DominantColors); got != `[{"hex":"#112233","fraction":0.75}]` {
		t.Fatalf("unexpected dominant colors: %s", got)
	}
	if len(result.Embedding) != 2 || len(result.UMAPEmbedding) != 1 {
		t.Fatalf("unexpected embeddings: %#v", result)
	}
}

func TestInferenceClientPrepareImageReturnsBinaryPayload(t *testing.T) {
	t.Parallel()

	expected := []byte("prepared")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prepare/image" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm returned error: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if got := r.FormValue("max_bytes"); got != "123" {
			t.Errorf("unexpected max_bytes: %s", got)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(expected)
	}))
	defer srv.Close()

	client := NewInferenceClient(srv.URL)
	prepared, mimeType, err := client.PrepareImage(context.Background(), []byte("img"), "image/heic", 123)
	if err != nil {
		t.Fatalf("PrepareImage returned error: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("unexpected mime type: %s", mimeType)
	}
	if string(prepared) != string(expected) {
		t.Fatalf("unexpected payload: %q", prepared)
	}
}
