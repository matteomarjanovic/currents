package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/atproto/labeling"
)

// LabelerIssuer mediates label issuance: constructs an atproto label, signs it
// via the Signer, persists it through PgStore.InsertLabel, appends an audit-log
// entry, and broadcasts the new row to any active subscribeLabels listeners.
//
// IssueLabel is safe to call concurrently. broadcast is non-blocking — a slow
// subscriber simply misses the live event and must catch up via the labelId
// cursor on reconnect.
type LabelerIssuer struct {
	Signer *LabelerSigner
	Store  *PgStore

	mu          sync.Mutex
	subscribers []chan<- LabelRow
}

// NewLabelerIssuer wires a signer to a store. Returns nil when signer is nil so
// the appview can run with the labeler disabled; callers must nil-check.
func NewLabelerIssuer(signer *LabelerSigner, store *PgStore) *LabelerIssuer {
	if signer == nil {
		return nil
	}
	return &LabelerIssuer{Signer: signer, Store: store}
}

// IssueLabelParams bundles the inputs for a label issuance. Actor and Action
// attribute the action in the moderation_event audit log; they don't affect the
// signed label itself (the labeler DID is always the src).
type IssueLabelParams struct {
	Actor     string // "auto" or moderator/author DID
	Action    string // ActionLabelAdd | ActionLabelNegate | ActionSelfLabel | ActionAIFlag
	URI       string // save URI being labeled
	RecordCID string // optional; empty = label applies to any version of the record
	BlobCID   string // denormalized for blob-keyed hydration; empty for URI-scoped labels (self-labels)
	Val       string // label value (e.g. "porn", "currents-nsfw-suspected")
	Neg       bool   // true = negation of a previous (src, uri, val)
}

// IssueLabel signs and persists a label, writes a moderation_event audit entry,
// and broadcasts the new row to live subscribers. Negation: clients honor the
// latest (src, uri, val) row, so issuing with Neg=true effectively retracts.
func (li *LabelerIssuer) IssueLabel(ctx context.Context, p IssueLabelParams) (LabelRow, error) {
	if li == nil {
		return LabelRow{}, fmt.Errorf("labeler not configured")
	}
	now := time.Now().UTC()
	label := &labeling.Label{
		Version:   labeling.ATPROTO_LABEL_VERSION,
		SourceDID: li.Signer.DID,
		URI:       p.URI,
		Val:       p.Val,
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	if p.RecordCID != "" {
		label.CID = &p.RecordCID
	}
	if p.Neg {
		t := true
		label.Negated = &t
	}
	if err := li.Signer.Sign(label); err != nil {
		return LabelRow{}, fmt.Errorf("sign label: %w", err)
	}

	row := LabelRow{
		Src:     label.SourceDID,
		URI:     label.URI,
		CID:     p.RecordCID,
		Val:     label.Val,
		Neg:     p.Neg,
		CTS:     now,
		Sig:     []byte(label.Sig),
		Ver:     int(label.Version),
		BlobCID: p.BlobCID,
	}
	id, err := li.Store.InsertLabel(ctx, row)
	if err != nil {
		return LabelRow{}, fmt.Errorf("insert label: %w", err)
	}
	row.ID = id

	actor := p.Actor
	if actor == "" {
		actor = "auto"
	}
	action := p.Action
	if action == "" {
		if p.Neg {
			action = ActionLabelNegate
		} else {
			action = ActionLabelAdd
		}
	}
	payload, _ := json.Marshal(map[string]any{"val": p.Val})
	_ = li.Store.InsertModerationEvent(ctx, actor, action, p.URI, p.RecordCID, p.BlobCID, payload)

	li.broadcast(row)
	return row, nil
}

// Subscribe registers ch to receive newly-issued labels. The caller must invoke
// Unsubscribe when done. Sends are non-blocking; a full channel is dropped.
func (li *LabelerIssuer) Subscribe(ch chan<- LabelRow) {
	li.mu.Lock()
	li.subscribers = append(li.subscribers, ch)
	li.mu.Unlock()
}

func (li *LabelerIssuer) Unsubscribe(ch chan<- LabelRow) {
	li.mu.Lock()
	defer li.mu.Unlock()
	for i, c := range li.subscribers {
		if c == ch {
			li.subscribers = append(li.subscribers[:i], li.subscribers[i+1:]...)
			return
		}
	}
}

func (li *LabelerIssuer) broadcast(row LabelRow) {
	li.mu.Lock()
	subs := append([]chan<- LabelRow(nil), li.subscribers...)
	li.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- row:
		default:
		}
	}
}
