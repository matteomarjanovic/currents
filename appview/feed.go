package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
)

const feedPersonalizedPoolCount = 3

type feedCursor struct {
	Version      int                    `json:"v"`
	Collections  []feedCursorCollection `json:"c,omitempty"`
	GlobalOffset int                    `json:"g,omitempty"`
}

type feedCursorCollection struct {
	URI    string `json:"u"`
	Offset int    `json:"o"`
}

type feedCandidatePool struct {
	URI      string
	Weight   float64
	Offset   int
	Items    []SaveRow
	More     bool
	consumed int
}

func decodeFeedCursor(raw string) (feedCursor, error) {
	if raw == "" {
		return feedCursor{Version: 1}, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return feedCursor{}, err
	}

	var cursor feedCursor
	if err := json.Unmarshal(decoded, &cursor); err == nil {
		if cursor.Version == 0 {
			cursor.Version = 1
		}
		if cursor.Version != 1 {
			return feedCursor{}, fmt.Errorf("unsupported feed cursor version")
		}
		if cursor.GlobalOffset < 0 {
			return feedCursor{}, fmt.Errorf("invalid global offset")
		}
		for _, col := range cursor.Collections {
			if col.URI == "" || col.Offset < 0 {
				return feedCursor{}, fmt.Errorf("invalid collection cursor")
			}
		}
		return cursor, nil
	}

	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return feedCursor{}, fmt.Errorf("invalid legacy feed cursor")
	}
	return feedCursor{Version: 1, GlobalOffset: offset}, nil
}

func encodeFeedCursor(cursor feedCursor) (string, error) {
	cursor.Version = 1
	if len(cursor.Collections) == 0 {
		cursor.Collections = nil
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
