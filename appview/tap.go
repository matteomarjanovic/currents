package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/gorilla/websocket"
)

const (
	defaultBlobEnrichmentConcurrency = 2
	defaultCollectionEmbeddingDelay  = 30 * time.Second
)

type TapEvent struct {
	ID       int64             `json:"id"`
	Type     string            `json:"type"`
	Record   *TapRecordEvent   `json:"record"`
	Identity *TapIdentityEvent `json:"identity"`
}

type TapRecordEvent struct {
	Live       bool            `json:"live"`
	DID        string          `json:"did"`
	Collection string          `json:"collection"`
	Rkey       string          `json:"rkey"`
	Action     string          `json:"action"`
	CID        string          `json:"cid"`
	Record     json.RawMessage `json:"record"`
}

type TapIdentityEvent struct {
	DID      string `json:"did"`
	Handle   string `json:"handle"`
	IsActive bool   `json:"isActive"`
	Status   string `json:"status"`
}

type TapAck struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
}

type collectionRecord struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}

// TapHandler holds the dependencies for the TAP event listener.
type TapHandler struct {
	Context                     context.Context
	Store                       *PgStore
	Dir                         identity.Directory
	Inference                   *InferenceClient
	CDNBaseURL                  string
	CollectionEmbeddingDebounce time.Duration

	asyncMu          sync.Mutex
	collectionTimers map[string]*time.Timer
	inflightBlobCIDs map[string]struct{}
	blobTokens       chan struct{}
}

func runTapListener(ctx context.Context, tapURL string, handler *TapHandler) {
	for {
		slog.Info("connecting to TAP", "url", tapURL)
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, tapURL, nil)
		if err != nil {
			slog.Warn("TAP dial failed", "err", err)
		} else {
			slog.Info("connected to TAP")
			handleTapConn(ctx, conn, handler)
			conn.Close()
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func handleTapConn(ctx context.Context, conn *websocket.Conn, handler *TapHandler) {
	// Close the connection when ctx is cancelled to unblock ReadMessage.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("TAP read error", "err", err)
			}
			return
		}

		var evt TapEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			slog.Warn("TAP unmarshal error", "err", err)
			continue
		}

		switch evt.Type {
		case "record":
			if evt.Record == nil {
				continue
			}
			if err := handleTapRecord(ctx, handler, evt.Record); err != nil {
				slog.Error("TAP record handler error", "err", err, "collection", evt.Record.Collection, "action", evt.Record.Action)
				continue // don't ack; TAP will redeliver
			}
		case "identity":
			// no-op for now; fall through to ack
		default:
			continue // unknown type; don't ack
		}

		if err := conn.WriteJSON(TapAck{Type: "ack", ID: evt.ID}); err != nil {
			slog.Warn("TAP ack write error", "err", err)
			return
		}
	}
}

func handleTapRecord(ctx context.Context, handler *TapHandler, ev *TapRecordEvent) error {
	atURI := fmt.Sprintf("at://%s/%s/%s", ev.DID, ev.Collection, ev.Rkey)

	switch ev.Collection {
	case collectionNSID:
		if ev.Action == "delete" {
			return handler.Store.DeleteCollection(ctx, atURI)
		}
		var col collectionRecord
		if err := json.Unmarshal(ev.Record, &col); err != nil {
			return fmt.Errorf("unmarshal collection record: %w", err)
		}
		createdAt := parseTimestamp(col.CreatedAt)
		return handler.Store.UpsertCollection(ctx, atURI, ev.CID, ev.DID, col.Name, col.Description, createdAt)

	case saveNSID:
		if ev.Action == "delete" {
			return handler.Store.DeleteSave(ctx, atURI)
		}
		var s saveRecord
		if err := json.Unmarshal(ev.Record, &s); err != nil {
			return fmt.Errorf("unmarshal save record: %w", err)
		}
		contentNSID, err := saveContentNSID(s.Content)
		if err != nil {
			return err
		}
		pdsBlobCID := ""
		if contentNSID == saveContentImageNSID {
			content, err := decodeSaveImageContent(s.Content)
			if err != nil {
				return err
			}
			pdsBlobCID = content.Image.Ref["$link"]
			if pdsBlobCID == "" {
				return fmt.Errorf("save record missing image blob CID")
			}
		}
		createdAt := parseTimestamp(s.CreatedAt)
		if err := handleSaveUpsert(ctx, handler, ev, s, atURI, contentNSID, pdsBlobCID, createdAt); err != nil {
			return err
		}
		if contentNSID == saveContentImageNSID {
			handler.scheduleEmbeddingUpdate(s.Collection.URI)
		}
		return nil

	case "is.currents.actor.profile":
		if ev.Action == "delete" {
			return nil // profile deletion not meaningful; skip
		}
		var p currentsProfileRecord
		if err := json.Unmarshal(ev.Record, &p); err != nil {
			return fmt.Errorf("unmarshal profile record: %w", err)
		}
		ident, err := handler.Dir.LookupDID(ctx, syntax.DID(ev.DID))
		if err != nil {
			slog.Warn("resolving DID for profile upsert", "did", ev.DID, "err", err)
		}
		handle := ""
		if ident != nil {
			handle = ident.Handle.String()
		}
		return handler.Store.CreateUser(ctx, userRecordFromCurrentsProfile(
			ev.DID,
			handle,
			"",
			handler.CDNBaseURL,
			p,
			time.Now(),
		))
	}

	return nil
}

func handleSaveUpsert(
	ctx context.Context,
	handler *TapHandler,
	ev *TapRecordEvent,
	s saveRecord,
	atURI, contentNSID, pdsBlobCID string,
	createdAt *time.Time,
) error {
	attr, err := effectiveSaveAttribution(s.Content, s.Attribution)
	if err != nil {
		return err
	}

	base := UpsertSaveParams{
		URI:           atURI,
		AuthorDID:     ev.DID,
		CollectionURI: s.Collection.URI,
		PdsBlobCID:    pdsBlobCID,
		ContentNSID:   contentNSID,
		Text:          s.Text,
		OriginURL:     s.OriginURL,
		ResaveOfURI:   s.ResaveOf.URI,
		ResaveOfCID:   s.ResaveOf.CID,
		CreatedAt:     createdAt,
	}
	if attr != nil {
		base.AttributionURL = attr.URL
		base.AttributionLicense = attr.License
		base.AttributionCredit = attr.Credit
	}

	if contentNSID != saveContentImageNSID || pdsBlobCID == "" {
		return handler.Store.UpsertSave(ctx, base)
	}

	// Case 1: Resave of a known save — reuse its visual identity and quality score.
	if s.ResaveOf.URI != "" {
		viID, qs, w, h, colors, err := handler.Store.GetSaveViIDAndQuality(ctx, s.ResaveOf.URI)
		if err == nil && viID != nil {
			base.VisualIdentityID = viID
			base.QualityScore = qs
			base.Width = w
			base.Height = h
			base.DominantColors = colors
			return handler.Store.UpsertSave(ctx, base)
		}
		// Original not in DB yet — fall through.
	}

	// Case 2: Same CID already linked to a visual identity.
	viID, qs, w, h, colors, err := handler.Store.GetViIDAndQualityByCID(ctx, pdsBlobCID)
	if err == nil && viID != nil {
		base.VisualIdentityID = viID
		base.QualityScore = qs
		base.Width = w
		base.Height = h
		base.DominantColors = colors
		return handler.Store.UpsertSave(ctx, base)
	}

	// Case 3: Novel image — persist immediately, then enrich asynchronously.
	if err := handler.Store.UpsertSave(ctx, base); err != nil {
		return err
	}
	handler.enqueueBlobEnrichment(pdsBlobCID)
	return nil
}

func (h *TapHandler) backgroundContext() context.Context {
	if h.Context != nil {
		return h.Context
	}
	return context.Background()
}

func (h *TapHandler) enqueueBlobEnrichment(blobCID string) {
	ctx := h.backgroundContext()

	h.asyncMu.Lock()
	if h.inflightBlobCIDs == nil {
		h.inflightBlobCIDs = make(map[string]struct{})
	}
	if _, exists := h.inflightBlobCIDs[blobCID]; exists {
		h.asyncMu.Unlock()
		return
	}
	if h.blobTokens == nil {
		h.blobTokens = make(chan struct{}, defaultBlobEnrichmentConcurrency)
	}
	h.inflightBlobCIDs[blobCID] = struct{}{}
	tokens := h.blobTokens
	h.asyncMu.Unlock()

	go func() {
		defer h.finishBlobEnrichment(blobCID)

		select {
		case tokens <- struct{}{}:
		case <-ctx.Done():
			return
		}
		defer func() { <-tokens }()

		if err := processBlobEnrichment(ctx, h, blobCID); err != nil && ctx.Err() == nil {
			slog.Warn("async blob enrichment failed", "blob_cid", blobCID, "err", err)
		}
	}()
}
func (h *TapHandler) finishBlobEnrichment(blobCID string) {
	h.asyncMu.Lock()
	delete(h.inflightBlobCIDs, blobCID)
	h.asyncMu.Unlock()
}

func (h *TapHandler) scheduleEmbeddingUpdate(collectionURI string) {
	if collectionURI == "" {
		return
	}
	delay := h.CollectionEmbeddingDebounce
	if delay <= 0 {
		delay = defaultCollectionEmbeddingDelay
	}
	ctx := h.backgroundContext()

	h.asyncMu.Lock()
	if h.collectionTimers == nil {
		h.collectionTimers = make(map[string]*time.Timer)
	}
	if timer, ok := h.collectionTimers[collectionURI]; ok {
		timer.Reset(delay)
		h.asyncMu.Unlock()
		return
	}
	h.collectionTimers[collectionURI] = time.AfterFunc(delay, func() {
		h.asyncMu.Lock()
		delete(h.collectionTimers, collectionURI)
		h.asyncMu.Unlock()

		if err := recomputeCollectionEmbedding(ctx, h.Store, collectionURI); err != nil && ctx.Err() == nil {
			slog.Warn("collection embedding update failed", "collection_uri", collectionURI, "err", err)
		}
	})
	h.asyncMu.Unlock()
}

func processBlobEnrichment(ctx context.Context, handler *TapHandler, blobCID string) error {
	candidates, err := handler.Store.ListBlobSourceCandidates(ctx, blobCID)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		slog.Info("blob enrichment skipped; no saves remain", "blob_cid", blobCID)
		return nil
	}

	source, imageBytes, mimeType, err := fetchBlobFromCandidates(ctx, handler.Store, handler.Dir, candidates, blobCID)
	if err != nil {
		return err
	}

	inferResult, err := handler.Inference.EmbedImage(ctx, imageBytes, mimeType)
	if err != nil {
		return err
	}

	quality := float32(qualityScore(inferResult.Width, inferResult.Height))
	nearestVI, err := handler.Store.FindNearestVI(ctx, inferResult.Embedding, 0.02)
	if err != nil {
		return err
	}

	if nearestVI != nil {
		if err := handler.Store.ApplyBlobVisualIdentity(ctx, blobCID, *nearestVI, quality, inferResult.Width, inferResult.Height, inferResult.DominantColors); err != nil {
			return err
		}
		if err := handler.Store.MaybePromoteCanonical(ctx, *nearestVI, source.AuthorDID, blobCID, source.URI, quality); err != nil {
			return err
		}
	} else {
		newVI, err := handler.Store.CreateVI(ctx, source.AuthorDID, blobCID, inferResult.Embedding, inferResult.UMAPEmbedding)
		if err != nil {
			return err
		}
		if err := handler.Store.ApplyBlobVisualIdentity(ctx, blobCID, newVI, quality, inferResult.Width, inferResult.Height, inferResult.DominantColors); err != nil {
			return err
		}
		if err := handler.Store.SetVICanonicalSave(ctx, newVI, source.URI); err != nil {
			return err
		}
	}

	collections, err := handler.Store.ListCollectionsByBlobCID(ctx, blobCID)
	if err != nil {
		return err
	}
	for _, collectionURI := range collections {
		handler.scheduleEmbeddingUpdate(collectionURI)
	}
	return nil
}

func recomputeCollectionEmbedding(ctx context.Context, store *PgStore, collectionURI string) error {
	embeddings, err := store.GetCollectionEmbeddings(ctx, collectionURI)
	if err != nil {
		return err
	}
	if len(embeddings) == 0 {
		return nil
	}
	medoid := computeMedoid(embeddings)
	return store.UpdateCollectionEmbedding(ctx, collectionURI, medoid)
}

func fetchBlobFromCandidates(ctx context.Context, store *PgStore, dir identity.Directory, candidates []BlobSourceCandidate, blobCID string) (BlobSourceCandidate, []byte, string, error) {
	var lastErr error
	for _, candidate := range candidates {
		imageBytes, mimeType, err := fetchBlobFromPDS(ctx, store, dir, candidate.AuthorDID, blobCID)
		if err == nil {
			return candidate, imageBytes, mimeType, nil
		}
		lastErr = fmt.Errorf("fetching blob from %s: %w", candidate.AuthorDID, err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no blob source candidates for %s", blobCID)
	}
	return BlobSourceCandidate{}, nil, "", lastErr
}

// computeMedoid returns the embedding with minimum total cosine distance to all others.
// O(n²) — fine for typical collection sizes.
func computeMedoid(embeddings [][]float32) []float32 {
	if len(embeddings) == 1 {
		return embeddings[0]
	}
	bestIdx := 0
	bestTotal := float32(math.MaxFloat32)
	for i, a := range embeddings {
		var total float32
		for j, b := range embeddings {
			if i != j {
				total += cosineDistance(a, b)
			}
		}
		if total < bestTotal {
			bestTotal = total
			bestIdx = i
		}
	}
	return embeddings[bestIdx]
}

func cosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1
	}
	return 1 - dot/float32(math.Sqrt(float64(normA)*float64(normB)))
}

func parseTimestamp(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
