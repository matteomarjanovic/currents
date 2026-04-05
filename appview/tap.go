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
	"golang.org/x/sync/errgroup"
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

type profileRecord struct {
	DisplayName string `json:"displayName"`
	Avatar      struct {
		Ref map[string]string `json:"ref"`
	} `json:"avatar"`
	CreatedAt string `json:"createdAt"`
}

type saveRecord struct {
	Collection struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"collection"`
	Image struct {
		Ref      map[string]string `json:"ref"`
		MimeType string            `json:"mimeType"`
	} `json:"image"`
	OriginURL   string `json:"originUrl"`
	Attribution struct {
		URL     string `json:"url"`
		License string `json:"license"`
		Credit  string `json:"credit"`
	} `json:"attribution"`
	Text     string `json:"text"`
	ResaveOf struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"resaveOf"`
	CreatedAt string `json:"createdAt"`
}

// TapHandler holds the dependencies for the TAP event listener.
type TapHandler struct {
	Store      *PgStore
	Dir        identity.Directory
	Inference  *InferenceClient
	CDNBaseURL string

	debounceMu  sync.Mutex
	debounceMap map[string]*time.Timer // keyed by collection_uri
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
		pdsBlobCID := s.Image.Ref["$link"]
		if pdsBlobCID == "" {
			return fmt.Errorf("save record missing image blob CID")
		}
		createdAt := parseTimestamp(s.CreatedAt)
		if err := handleSaveUpsert(ctx, handler, ev, s, atURI, pdsBlobCID, createdAt); err != nil {
			return err
		}
		handler.scheduleEmbeddingUpdate(s.Collection.URI)
		return nil

	case "is.currents.actor.profile":
		if ev.Action == "delete" {
			return nil // profile deletion not meaningful; skip
		}
		var p profileRecord
		if err := json.Unmarshal(ev.Record, &p); err != nil {
			return fmt.Errorf("unmarshal profile record: %w", err)
		}
		avatarURL := ""
		if cid := p.Avatar.Ref["$link"]; cid != "" {
			avatarURL = handler.CDNBaseURL + "/img/" + ev.DID + "/" + cid
		}
		ident, err := handler.Dir.LookupDID(ctx, syntax.DID(ev.DID))
		if err != nil {
			slog.Warn("resolving DID for profile upsert", "did", ev.DID, "err", err)
		}
		handle := ""
		if ident != nil {
			handle = ident.Handle.String()
		}
		createdAt := parseTimestamp(p.CreatedAt)
		var createdAtTime time.Time
		if createdAt != nil {
			createdAtTime = *createdAt
		} else {
			createdAtTime = time.Now()
		}
		return handler.Store.CreateUser(ctx, UserRecord{
			DID:         ev.DID,
			Handle:      handle,
			DisplayName: p.DisplayName,
			Avatar:      avatarURL,
			CreatedAt:   createdAtTime,
		})
	}

	return nil
}

func handleSaveUpsert(
	ctx context.Context,
	handler *TapHandler,
	ev *TapRecordEvent,
	s saveRecord,
	atURI, pdsBlobCID string,
	createdAt *time.Time,
) error {
	base := UpsertSaveParams{
		URI:           atURI,
		AuthorDID:     ev.DID,
		CollectionURI: s.Collection.URI,
		PdsBlobCID:    pdsBlobCID,
		Text:               s.Text,
		OriginURL:          s.OriginURL,
		AttributionURL:     s.Attribution.URL,
		AttributionLicense: s.Attribution.License,
		AttributionCredit:  s.Attribution.Credit,
		ResaveOfURI:        s.ResaveOf.URI,
		CreatedAt:     createdAt,
	}

	// Persist the save immediately so it's visible even if VI enrichment
	// fails or hangs (blob fetch, inference, etc.). The ON CONFLICT UPDATE
	// below will enrich the row once visual identity is resolved.
	if err := handler.Store.UpsertSave(ctx, base); err != nil {
		return err
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

	// Case 3: Novel image — fetch blob, analyze, resolve or create visual identity.
	imageBytes, mimeType, err := fetchBlobFromPDS(ctx, handler.Store, handler.Dir, ev.DID, pdsBlobCID)
	if err != nil {
		slog.Warn("failed to fetch blob for visual identity", "uri", atURI, "err", err)
		return nil // save already persisted; backfill VI later
	}

	var (
		embedding      []float32
		dominantColors json.RawMessage
		imgWidth       int
		imgHeight      int
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		embedding, err = handler.Inference.EmbedImage(gctx, imageBytes, mimeType)
		return err
	})
	g.Go(func() error {
		var err error
		dominantColors, imgWidth, imgHeight, err = analyzeImageLocally(imageBytes)
		return err
	})
	if err := g.Wait(); err != nil {
		slog.Warn("image analysis failed for visual identity", "uri", atURI, "err", err)
		return nil // save already persisted; backfill VI later
	}

	qs32 := float32(qualityScore(imgWidth, imgHeight))
	base.QualityScore = &qs32
	base.Width = &imgWidth
	base.Height = &imgHeight
	base.DominantColors = dominantColors

	nearestVI, err := handler.Store.FindNearestVI(ctx, embedding, 0.02)
	if err != nil {
		slog.Warn("nearest VI search failed", "uri", atURI, "err", err)
		return nil // save already persisted; backfill VI later
	}

	if nearestVI != nil {
		// Near-duplicate: link to existing VI, maybe promote canonical.
		if err := handler.Store.MaybePromoteCanonical(ctx, *nearestVI, ev.DID, pdsBlobCID, atURI, qs32); err != nil {
			slog.Warn("canonical promotion failed", "vi", *nearestVI, "err", err)
		}
		base.VisualIdentityID = nearestVI
		return handler.Store.UpsertSave(ctx, base)
	}

	// Novel: create a new visual identity. This save IS the initial canonical.
	newVI, err := handler.Store.CreateVI(ctx, ev.DID, pdsBlobCID, embedding)
	if err != nil {
		slog.Warn("failed to create visual identity", "uri", atURI, "err", err)
		return nil // save already persisted; backfill VI later
	}
	base.VisualIdentityID = &newVI
	if err := handler.Store.UpsertSave(ctx, base); err != nil {
		return err
	}
	if err := handler.Store.SetVICanonicalSave(ctx, newVI, atURI); err != nil {
		slog.Warn("failed to set canonical save on VI", "vi", newVI, "uri", atURI, "err", err)
	}
	return nil
}

// scheduleEmbeddingUpdate debounces recomputation of a collection's canonical embedding.
// If a timer is already pending for the collection it is reset; otherwise a new one is created.
func (h *TapHandler) scheduleEmbeddingUpdate(collectionURI string) {
	const delay = 30 * time.Second
	h.debounceMu.Lock()
	defer h.debounceMu.Unlock()
	if h.debounceMap == nil {
		h.debounceMap = make(map[string]*time.Timer)
	}
	if t, ok := h.debounceMap[collectionURI]; ok {
		t.Reset(delay)
		return
	}
	h.debounceMap[collectionURI] = time.AfterFunc(delay, func() {
		h.debounceMu.Lock()
		delete(h.debounceMap, collectionURI)
		h.debounceMu.Unlock()
		h.recomputeCollectionEmbedding(collectionURI)
	})
}

// recomputeCollectionEmbedding loads all VI embeddings for a collection, computes the medoid,
// and writes it back to collection.canonical_embedding.
func (h *TapHandler) recomputeCollectionEmbedding(collectionURI string) {
	ctx := context.Background()
	embeddings, err := h.Store.GetCollectionEmbeddings(ctx, collectionURI)
	if err != nil {
		slog.Warn("GetCollectionEmbeddings failed", "collection", collectionURI, "err", err)
		return
	}
	if len(embeddings) == 0 {
		return
	}
	medoid := computeMedoid(embeddings)
	if err := h.Store.UpdateCollectionEmbedding(ctx, collectionURI, medoid); err != nil {
		slog.Warn("UpdateCollectionEmbedding failed", "collection", collectionURI, "err", err)
	}
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
