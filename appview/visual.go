package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"time"

	"log/slog"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

var blobHTTPClient = &http.Client{Timeout: 30 * time.Second}

// ── Inference client ──────────────────────────────────────────────────────────

type InferenceClient struct {
	baseURL string
	client  *http.Client
}

func NewInferenceClient(baseURL string) *InferenceClient {
	return &InferenceClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ImageEmbedding holds the result of an /embed/image call.
// UMAPEmbedding is nil when the inference server has no UMAP model loaded.
// SafetyScores is nil when the inference server has no moderation heads loaded;
// the appview treats safety scoring as optional so the backend can deploy ahead
// of the heads being trained.
type ImageEmbedding struct {
	Embedding      []float32       `json:"embedding"`
	UMAPEmbedding  []float32       `json:"umap_embedding"`
	Width          int             `json:"width"`
	Height         int             `json:"height"`
	DominantColors json.RawMessage `json:"dominant_colors"`
	SafetyScores   *SafetyScores   `json:"safety_scores,omitempty"`
	JunkScore      *float32        `json:"junk_score,omitempty"` // nil when no feed junk head is loaded
}

func (c *InferenceClient) doImageRequest(ctx context.Context, path string, imageBytes []byte, mimeType string, fields map[string]string) (*http.Response, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if mimeType == "" {
		mimeType = http.DetectContentType(imageBytes)
	}
	for key, value := range fields {
		if err := mw.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="image"`)
	h.Set("Content-Type", mimeType)
	fw, err := mw.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(imageBytes); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.client.Do(req)
}

func (c *InferenceClient) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) (ImageEmbedding, error) {
	resp, err := c.doImageRequest(ctx, "/embed/image", imageBytes, mimeType, nil)
	if err != nil {
		return ImageEmbedding{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ImageEmbedding{}, fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}

	var result ImageEmbedding
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ImageEmbedding{}, fmt.Errorf("decoding inference response: %w", err)
	}
	return result, nil
}

// Palette extracts the dominant-color palette without running the embedding
// model — used by the colors backfill.
func (c *InferenceClient) Palette(ctx context.Context, imageBytes []byte, mimeType string) (json.RawMessage, error) {
	resp, err := c.doImageRequest(ctx, "/palette", imageBytes, mimeType, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		DominantColors json.RawMessage `json:"dominant_colors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding inference response: %w", err)
	}
	return result.DominantColors, nil
}

func (c *InferenceClient) TranscodeImage(ctx context.Context, imageBytes []byte, mimeType string) ([]byte, string, error) {
	resp, err := c.doImageRequest(ctx, "/transcode/image", imageBytes, mimeType, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}

	transcoded, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading transcoded image response: %w", err)
	}
	outMime := resp.Header.Get("Content-Type")
	if outMime == "" {
		outMime = "image/jpeg"
	}
	return transcoded, outMime, nil
}

func (c *InferenceClient) PrepareImage(ctx context.Context, imageBytes []byte, mimeType string, maxBytes int) ([]byte, string, error) {
	resp, err := c.doImageRequest(ctx, "/prepare/image", imageBytes, mimeType, map[string]string{
		"max_bytes": strconv.Itoa(maxBytes),
	})
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}

	prepared, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading prepared image response: %w", err)
	}
	mimeType = resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(prepared)
	}
	return prepared, mimeType, nil
}

// ClassifySafetyEmbeddings posts a batch of pre-computed 768-d embeddings to
// the inference server's CPU-only backfill endpoint and returns one SafetyScores
// per row in input order. The server L2-normalizes server-side, so callers can
// pass raw vectors straight out of visual_identity.embedding.
//
// Returns an error if no safety heads are loaded server-side (503).
func (c *InferenceClient) ClassifySafetyEmbeddings(ctx context.Context, embeddings [][]float32) ([]SafetyScores, error) {
	if len(embeddings) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{"embeddings": embeddings})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify/safety/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Results []SafetyScores `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding inference response: %w", err)
	}
	return out.Results, nil
}

// ClassifyJunkEmbeddings posts a batch of pre-computed 768-d embeddings to the
// inference server's CPU-only backfill endpoint and returns one junk
// probability (1 = unsuitable for the global feed) per row in input order.
// The server L2-normalizes server-side, so callers can pass raw vectors
// straight out of visual_identity.embedding.
//
// Returns an error if no feed junk head is loaded server-side (503).
func (c *InferenceClient) ClassifyJunkEmbeddings(ctx context.Context, embeddings [][]float32) ([]float32, error) {
	if len(embeddings) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{"embeddings": embeddings})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify/junk/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Results []float32 `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding inference response: %w", err)
	}
	return out.Results, nil
}

func (c *InferenceClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed/text", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference server returned %d: %s", resp.StatusCode, b)
	}
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding inference response: %w", err)
	}
	return result.Embedding, nil
}

// ── Colors ────────────────────────────────────────────────────────────────────

type dominantColor struct {
	Hex      string  `json:"hex"`
	Fraction float64 `json:"fraction"`
}

// hexToLab converts a #rrggbb color to CIELab (D65). It is the single
// definition of the color space behind visual_identity_color: both stored
// palettes and query colors go through it, so ΔE comparisons are consistent.
func hexToLab(hex string) ([3]float32, error) {
	if len(hex) == 7 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return [3]float32{}, fmt.Errorf("invalid hex color %q", hex)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return [3]float32{}, fmt.Errorf("invalid hex color %q", hex)
	}

	var lin [3]float64
	for i, ch := range [3]uint64{v >> 16 & 0xff, v >> 8 & 0xff, v & 0xff} {
		c := float64(ch) / 255.0
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	x := (0.4124564*lin[0] + 0.3575761*lin[1] + 0.1804375*lin[2]) / 0.95047
	y := 0.2126729*lin[0] + 0.7151522*lin[1] + 0.0721750*lin[2]
	z := (0.0193339*lin[0] + 0.1191920*lin[1] + 0.9503041*lin[2]) / 1.08883

	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return t*7.787 + 16.0/116.0
	}
	fx, fy, fz := f(x), f(y), f(z)
	return [3]float32{
		float32(116.0*fy - 16.0),
		float32(500.0 * (fx - fy)),
		float32(200.0 * (fy - fz)),
	}, nil
}

// ── Image fetching ────────────────────────────────────────────────────────────

// fetchBlobFromPDS downloads a blob from the author's PDS.
// Tries the cached endpoint first; on failure falls back to DID document
// resolution to handle PDS migrations where the cached endpoint is stale.
// On a successful fallback, updates the cached endpoint in the DB.
func fetchBlobFromPDS(ctx context.Context, store *PgStore, dir identity.Directory, authorDID, blobCID string) ([]byte, string, error) {
	if cached, _ := store.GetUserPDSEndpoint(ctx, authorDID); cached != "" {
		if data, mime, err := getBlobFromEndpoint(ctx, cached, authorDID, blobCID); err == nil {
			return data, mime, nil
		}
		slog.Warn("cached PDS endpoint failed, falling back to DID resolution", "did", authorDID, "cached", cached)
	}

	ident, err := dir.LookupDID(ctx, syntax.DID(authorDID))
	if err != nil {
		return nil, "", fmt.Errorf("resolving DID %s: %w", authorDID, err)
	}
	pdsEndpoint := ident.PDSEndpoint()
	if pdsEndpoint == "" {
		return nil, "", fmt.Errorf("no PDS endpoint for DID %s", authorDID)
	}

	data, mime, err := getBlobFromEndpoint(ctx, pdsEndpoint, authorDID, blobCID)
	if err != nil {
		return nil, "", err
	}

	go func() {
		if err := store.UpdateUserPDSEndpoint(context.Background(), authorDID, pdsEndpoint); err != nil {
			slog.Error("failed to update PDS endpoint in DB", "did", authorDID, "endpoint", pdsEndpoint, "err", err)
		}
	}()

	return data, mime, nil
}

func getBlobFromEndpoint(ctx context.Context, pdsEndpoint, authorDID, blobCID string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s", pdsEndpoint, authorDID, blobCID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := blobHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching blob: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("blob fetch returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading blob: %w", err)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// ── Image proxy ──────────────────────────────────────────────────────────────

func (s *Server) ImageProxy(w http.ResponseWriter, r *http.Request) {
	did := r.PathValue("did")
	cid := r.PathValue("cid")

	// Public images: allow any origin to fetch() them (used by the organize context
	// menu's copy/download). Overrides the credentialed CORS the global middleware sets
	// so the ACAO is a plain "*" — valid without credentials and cacheable as one entry
	// for every caller (the response is Cache-Control: immutable). No Origin echo, so an
	// <img> load that fills the cache and a later fetch() share the same permissive ACAO.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Del("Access-Control-Allow-Credentials")
	w.Header().Del("Vary")

	data, mimeType, err := fetchBlobFromPDS(r.Context(), s.Store, s.Dir, did, cid)
	if err != nil {
		slog.Error("image proxy failed", "did", did, "cid", cid, "err", err)
		http.Error(w, "could not fetch image", http.StatusBadGateway)
		return
	}

	if isHEIC(mimeType) {
		transcoded, transcodedMime, err := s.Inference.TranscodeImage(r.Context(), data, mimeType)
		if err != nil {
			slog.Error("image transcode failed", "did", did, "cid", cid, "err", err)
			http.Error(w, "could not transcode image", http.StatusBadGateway)
			return
		}
		data = transcoded
		mimeType = transcodedMime
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(data)
}

func isHEIC(mimeType string) bool {
	switch mimeType {
	case "image/heic", "image/heif", "image/heic-sequence", "image/heif-sequence":
		return true
	}
	return false
}

// ── Quality score ─────────────────────────────────────────────────────────────

func qualityScore(width, height int) float64 {
	shortSide := math.Min(float64(width), float64(height))
	aspect := float64(width) / float64(height)

	resScore := math.Max(0.0, math.Min(1.0, (shortSide-200)/(600-200)))

	idealMin, idealMax := 0.5, 2.0
	var arScore float64
	if aspect >= idealMin && aspect <= idealMax {
		arScore = 1.0
	} else {
		distance := math.Max(idealMin-aspect, aspect-idealMax)
		arScore = math.Max(0.0, 1.0-distance)
	}

	return math.Round(((resScore+arScore)/2)*1000) / 1000
}
