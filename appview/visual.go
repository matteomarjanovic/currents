package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/webp"
)

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

func (c *InferenceClient) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if mimeType == "" {
		mimeType = http.DetectContentType(imageBytes)
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
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed/image", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference server returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding inference response: %w", err)
	}
	return result.Embedding, nil
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

// ── Local image analysis ──────────────────────────────────────────────────────

type dominantColor struct {
	Hex      string  `json:"hex"`
	Fraction float64 `json:"fraction"`
}

// analyzeImageLocally decodes image bytes and returns the dominant color palette,
// image width, and image height. Palette extraction uses k-means (k=5) on a
// 64×64 thumbnail, matching the Python inference server's previous behavior.
func analyzeImageLocally(imageBytes []byte) (json.RawMessage, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decoding image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// Sample pixels into a 64×64 grid using nearest-neighbor.
	const thumbSize = 64
	pixels := make([][3]float64, 0, thumbSize*thumbSize)
	for y := 0; y < thumbSize; y++ {
		for x := 0; x < thumbSize; x++ {
			srcX := bounds.Min.X + x*width/thumbSize
			srcY := bounds.Min.Y + y*height/thumbSize
			r, g, b, _ := img.At(srcX, srcY).RGBA()
			pixels = append(pixels, [3]float64{
				float64(r >> 8),
				float64(g >> 8),
				float64(b >> 8),
			})
		}
	}

	palette := kmeans(pixels, 5, 10)

	// Count each pixel's nearest centroid.
	counts := make([]int, len(palette))
	for _, px := range pixels {
		counts[nearest(px, palette)]++
	}

	total := float64(len(pixels))
	colors := make([]dominantColor, len(palette))
	for i, c := range palette {
		colors[i] = dominantColor{
			Hex: fmt.Sprintf("#%02x%02x%02x",
				clamp(c[0]), clamp(c[1]), clamp(c[2])),
			Fraction: math.Round(float64(counts[i])/total*10000) / 10000,
		}
	}
	// Sort descending by fraction.
	for i := 1; i < len(colors); i++ {
		for j := i; j > 0 && colors[j].Fraction > colors[j-1].Fraction; j-- {
			colors[j], colors[j-1] = colors[j-1], colors[j]
		}
	}

	raw, err := json.Marshal(colors)
	if err != nil {
		return nil, 0, 0, err
	}
	return raw, width, height, nil
}

func kmeans(pixels [][3]float64, k, iters int) [][3]float64 {
	// Seed centroids from evenly spaced pixels.
	centroids := make([][3]float64, k)
	step := len(pixels) / k
	for i := range centroids {
		centroids[i] = pixels[rand.IntN(step)+i*step]
	}

	assign := make([]int, len(pixels))
	for iter := 0; iter < iters; iter++ {
		// Assignment step.
		for i, px := range pixels {
			assign[i] = nearest(px, centroids)
		}
		// Update step.
		sums := make([][3]float64, k)
		counts := make([]int, k)
		for i, px := range pixels {
			c := assign[i]
			sums[c][0] += px[0]
			sums[c][1] += px[1]
			sums[c][2] += px[2]
			counts[c]++
		}
		for i := range centroids {
			if counts[i] == 0 {
				centroids[i] = pixels[rand.IntN(len(pixels))]
				continue
			}
			n := float64(counts[i])
			centroids[i] = [3]float64{sums[i][0] / n, sums[i][1] / n, sums[i][2] / n}
		}
	}
	return centroids
}

func nearest(px [3]float64, centroids [][3]float64) int {
	best, bestDist := 0, math.MaxFloat64
	for i, c := range centroids {
		d := (px[0]-c[0])*(px[0]-c[0]) +
			(px[1]-c[1])*(px[1]-c[1]) +
			(px[2]-c[2])*(px[2]-c[2])
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

func clamp(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// ── Image fetching ────────────────────────────────────────────────────────────

// fetchBlobFromPDS downloads a blob from the author's PDS.
// It first tries the local user table for the PDS endpoint (fast path),
// then falls back to resolving the DID document via the identity directory.
func fetchBlobFromPDS(ctx context.Context, store *PgStore, dir identity.Directory, authorDID, blobCID string) ([]byte, string, error) {
	pdsEndpoint, err := store.GetUserPDSEndpoint(ctx, authorDID)
	if err != nil || pdsEndpoint == "" {
		ident, err := dir.LookupDID(ctx, syntax.DID(authorDID))
		if err != nil {
			return nil, "", fmt.Errorf("resolving DID %s: %w", authorDID, err)
		}
		pdsEndpoint = ident.PDSEndpoint()
		if pdsEndpoint == "" {
			return nil, "", fmt.Errorf("no PDS endpoint for DID %s", authorDID)
		}
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s", pdsEndpoint, authorDID, blobCID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := http.DefaultClient.Do(req)
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

	data, mimeType, err := fetchBlobFromPDS(r.Context(), s.Store, s.Dir, did, cid)
	if err != nil {
		http.Error(w, "could not fetch image", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(data)
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
