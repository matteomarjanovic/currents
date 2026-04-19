package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
)

const feedPersonalizedPoolCount = 3

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
		if c.Initialized || len(c.Collections) > 0 || len(c.Seeds) > 0 {
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
