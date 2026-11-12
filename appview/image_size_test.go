package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSaveImageSizeMatchesLexicon(t *testing.T) {
	raw, err := os.ReadFile("../lexicons/is/currents/content/image.json")
	if err != nil {
		t.Fatal(err)
	}
	var lex struct {
		Defs struct {
			Main struct {
				Properties struct {
					Image struct {
						MaxSize int `json:"maxSize"`
					} `json:"image"`
				} `json:"properties"`
			} `json:"main"`
		} `json:"defs"`
	}
	if err := json.Unmarshal(raw, &lex); err != nil {
		t.Fatal(err)
	}
	if maxBlobSize != lex.Defs.Main.Properties.Image.MaxSize || maxFetchedBlobSize != maxBlobSize {
		t.Fatal("upload, proxy and lexicon limits must agree")
	}
}

func TestSaveBlobSizeValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		size   any
		status int
	}{
		{"small", 1, 200}, {"exact limit", maxBlobSize, 200},
		{"one byte over", maxBlobSize + 1, 413}, {"reported GIF", 29_880_100, 413},
		{"missing", nil, 400}, {"zero", 0, 400}, {"negative", -1, 400},
		{"fraction", 1.5, 400}, {"string", "1", 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			blob := map[string]any{"size": tt.size, "mimeType": "image/gif"}
			res := httptest.NewRecorder()
			ok := checkSaveBlobSize(res, blob)
			if ok != (tt.status == 200) || res.Code != tt.status {
				t.Fatalf("got valid=%v status=%d body=%s", ok, res.Code, res.Body)
			}
		})
	}
}

func TestPrepareImagePreservesGIFAtLimitAndRejectsOversize(t *testing.T) {
	data := make([]byte, maxBlobSize+1)
	copy(data, "GIF89a")
	prepared, mime, err := prepareImageForUpload(context.Background(), nil, data[:maxBlobSize], "image/gif")
	if err != nil || mime != "image/gif" || !bytes.Equal(prepared, data[:maxBlobSize]) {
		t.Fatalf("in-limit GIF was changed: %s %v", mime, err)
	}
	for _, declared := range []string{"image/gif", "image/jpeg", "application/octet-stream"} {
		if _, _, err := prepareImageForUpload(context.Background(), nil, data, declared); !errors.Is(err, errImageTooLarge) {
			t.Fatalf("oversized GIF declared %q was not rejected: %v", declared, err)
		}
	}
}

func TestPrepareOversizedStillImage(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, maxBlobSize+1)
	copy(data, encoded.Bytes())
	prepared, mime, err := prepareImageForUpload(context.Background(), nil, data, "image/png")
	if err != nil || len(prepared) > maxBlobSize || mime != "image/jpeg" {
		t.Fatalf("existing still-image preparation failed: size=%d mime=%q err=%v", len(prepared), mime, err)
	}
}
