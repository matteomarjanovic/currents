package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
)

const feedPersonalizedPoolCount = 3

// feedCollectionCandidatePool is how many of the viewer's top collections (by
// importance) are considered when building the personalized/serendipity pools.
// feedPersonalizedPoolCount are then sampled from this wider set, weighted by
// importance and driven by the per-request seed — that sampling is what rotates
// the feed between refreshes instead of always drawing the same top collections.
const feedCollectionCandidatePool = 5

// feedVarietyWindow bounds the seeded start offset applied to each personalized
// ANN pool: a fresh load starts somewhere within the first feedVarietyWindow
// nearest neighbours instead of always at 0, so the images inside each pool
// rotate too. This is the lever that gives per-refresh variety even to viewers
// with feedPersonalizedPoolCount or fewer collections (where collection sampling
// can't vary the selection).
const feedVarietyWindow = 60

type feedCursorMode string

const (
	feedCursorModeGlobal   feedCursorMode = "global"
	feedCursorModePositive feedCursorMode = "positive"
	feedCursorModeNegative feedCursorMode = "negative"
)

type feedCursor struct {
	Version      int                    `json:"v"`
	Mode         feedCursorMode         `json:"m"`
	Initialized  bool                   `json:"i,omitempty"`
	Collections  []feedCursorCollection `json:"c,omitempty"`
	Seeds        []feedCursorSeed       `json:"s,omitempty"`
	GlobalOffset int                    `json:"g,omitempty"`
	// Seed drives per-request variety in the personalized/serendipity feeds
	// (collection sampling + per-pool start offsets + pool interleaving). Minted
	// on the first page and carried across pages so pagination stays stable within
	// a scroll session while a fresh load (new seed) reshuffles. Unused (0) for the
	// global feed, which keeps its own daily jitter.
	Seed int64 `json:"r,omitempty"`
}

type feedCursorCollection struct {
	URI    string `json:"u"`
	Offset int    `json:"o"`
}

type feedCursorSeed struct {
	VisualIdentityID string `json:"i"`
	Offset           int    `json:"o"`
}

type feedCandidatePool struct {
	Key      string
	Weight   float64
	Offset   int
	Items    []SaveRow
	More     bool
	consumed int
}

func requestedFeedCursorMode(alpha float64) feedCursorMode {
	switch {
	case alpha > 0:
		return feedCursorModePositive
	case alpha < 0:
		return feedCursorModeNegative
	default:
		return feedCursorModeGlobal
	}
}

func (m feedCursorMode) valid() bool {
	switch m {
	case feedCursorModeGlobal, feedCursorModePositive, feedCursorModeNegative:
		return true
	default:
		return false
	}
}

func (c feedCursor) validate() error {
	if !c.Mode.valid() {
		return fmt.Errorf("unsupported feed cursor mode")
	}
	if c.Version != 1 {
		return fmt.Errorf("unsupported feed cursor version")
	}
	if c.GlobalOffset < 0 {
		return fmt.Errorf("invalid global offset")
	}
	for _, col := range c.Collections {
		if col.URI == "" || col.Offset < 0 {
			return fmt.Errorf("invalid collection cursor")
		}
	}
	for _, seed := range c.Seeds {
		if seed.VisualIdentityID == "" || seed.Offset < 0 {
			return fmt.Errorf("invalid seed cursor")
		}
	}

	switch c.Mode {
	case feedCursorModeGlobal:
		if c.Initialized || len(c.Collections) > 0 || len(c.Seeds) > 0 || c.Seed != 0 {
			return fmt.Errorf("invalid global cursor")
		}
	case feedCursorModePositive:
		if !c.Initialized || len(c.Seeds) > 0 {
			return fmt.Errorf("invalid positive cursor")
		}
	case feedCursorModeNegative:
		if !c.Initialized || len(c.Collections) > 0 {
			return fmt.Errorf("invalid negative cursor")
		}
	}

	return nil
}

func (c feedCursor) validateForMode(mode feedCursorMode) error {
	if err := c.validate(); err != nil {
		return err
	}
	if c.Mode != mode {
		return fmt.Errorf("cursor mode mismatch")
	}
	return nil
}

func decodeFeedCursor(raw string) (feedCursor, error) {
	if raw == "" {
		return feedCursor{Version: 1, Mode: feedCursorModeGlobal}, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return feedCursor{}, err
	}

	var cursor feedCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return feedCursor{}, err
	}
	if err := cursor.validate(); err != nil {
		return feedCursor{}, err
	}
	return cursor, nil
}

func encodeFeedCursor(cursor feedCursor) (string, error) {
	cursor.Version = 1
	if cursor.Mode == "" {
		cursor.Mode = feedCursorModeGlobal
	}
	if len(cursor.Collections) == 0 {
		cursor.Collections = nil
	}
	if len(cursor.Seeds) == 0 {
		cursor.Seeds = nil
	}
	if err := cursor.validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func feedPoolFetchLimit(limit int) int {
	return max(limit*3, limit+25)
}

// feedMixSeedSalt derives the pool-interleaving RNG seed from the cursor seed so
// buildFeedPage's mixing is independent of the collection-sampling RNG draws.
const feedMixSeedSalt int64 = -0x61c8864680b583eb // golden-ratio constant

// newFeedSeed returns a non-zero random seed for a personalized/serendipity feed
// request (0 is reserved for "no seed" / the global feed).
func newFeedSeed() int64 {
	s := rand.Int63()
	if s == 0 {
		s = 1
	}
	return s
}

// sampleCollections picks up to n collections from cands without replacement,
// each draw weighted by its importance Score, using rng. It's deterministic for a
// given rng seed, so a cursor's stored seed reproduces the same selection across
// pages; the returned slice is in draw order. Sampling a few from a wider set
// (rather than taking a hard top-n) is what rotates the feed between refreshes.
func sampleCollections(cands []CollectionImportance, n int, rng *rand.Rand) []CollectionImportance {
	pool := make([]CollectionImportance, len(cands))
	copy(pool, cands)
	picked := make([]CollectionImportance, 0, n)
	for len(picked) < n && len(pool) > 0 {
		total := 0.0
		for _, c := range pool {
			if c.Score > 0 {
				total += c.Score
			}
		}
		idx := len(pool) - 1
		if total <= 0 {
			// No positive weights left: fall back to a uniform pick.
			idx = rng.Intn(len(pool))
		} else {
			pick := rng.Float64() * total
			acc := 0.0
			for i, c := range pool {
				if c.Score <= 0 {
					continue
				}
				acc += c.Score
				if pick <= acc {
					idx = i
					break
				}
			}
		}
		picked = append(picked, pool[idx])
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	return picked
}

// seededStartOffset maps (seed, key) to a stable offset in [0, window), so each
// personalized pool starts at a different slice of its nearest neighbours per
// fresh load while staying fixed within a cursor session.
func seededStartOffset(seed int64, key string, window int) int {
	if window <= 0 {
		return 0
	}
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(seed))
	_, _ = h.Write(b[:])
	_, _ = h.Write([]byte(key))
	return int(h.Sum64() % uint64(window))
}

func (p *feedCandidatePool) hasRemaining() bool {
	return p.consumed < len(p.Items)
}

func (p *feedCandidatePool) nextOffset() int {
	return p.Offset + p.consumed
}

func (p *feedCandidatePool) hasMoreAfterPage() bool {
	return p.consumed < len(p.Items) || p.More
}

func (p *feedCandidatePool) consumeNextUnique(seen map[string]bool) (SaveRow, bool) {
	for p.consumed < len(p.Items) {
		row := p.Items[p.consumed]
		p.consumed++
		if seen[row.URI] {
			continue
		}
		return row, true
	}
	return SaveRow{}, false
}

func buildFeedPage(rng *rand.Rand, pools []*feedCandidatePool, limit int) []SaveRow {
	rows := make([]SaveRow, 0, limit)
	seen := make(map[string]bool, limit)

	for len(rows) < limit {
		totalWeight := 0.0
		for _, pool := range pools {
			if pool.Weight > 0 && pool.hasRemaining() {
				totalWeight += pool.Weight
			}
		}
		if totalWeight == 0 {
			break
		}

		pick := rand.Float64() * totalWeight
		if rng != nil {
			pick = rng.Float64() * totalWeight
		}

		acc := 0.0
		selected := -1
		for i, pool := range pools {
			if pool.Weight <= 0 || !pool.hasRemaining() {
				continue
			}
			acc += pool.Weight
			if pick <= acc {
				selected = i
				break
			}
		}
		if selected < 0 {
			break
		}

		row, ok := pools[selected].consumeNextUnique(seen)
		if !ok {
			continue
		}
		seen[row.URI] = true
		rows = append(rows, row)
	}

	for _, pool := range pools {
		for len(rows) < limit {
			row, ok := pool.consumeNextUnique(seen)
			if !ok {
				break
			}
			seen[row.URI] = true
			rows = append(rows, row)
		}
		if len(rows) >= limit {
			break
		}
	}

	return rows
}
