package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/gorilla/websocket"
	cbg "github.com/whyrusleeping/cbor-gen"
)

// wsUpgrader allows cross-origin upgrades — atproto clients connect from many
// domains and a labeler service is meant to be publicly subscribable.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Number of rows fetched per backlog chunk; bounded to avoid OOM on first
// connect after long client outages. Streamed without buffering between chunks.
const backlogChunkSize = 1000

// liveBufferSize bounds the per-subscriber channel — IssueLabel.broadcast drops
// on full so a slow subscriber can't back-pressure label issuance. Slow clients
// reconnect with their cursor to catch up.
const liveBufferSize = 256

// XRPCSubscribeLabels implements com.atproto.label.subscribeLabels.
// Optional ?cursor=<int>: stream from id > cursor (default 0 = from the start).
//
// Sequencing: subscribe to the live channel BEFORE reading the backlog so we
// don't miss a label issued in the gap. Then drain backlog, then enter live mode
// skipping any live frames whose id is already covered by the backlog.
func (s *Server) XRPCSubscribeLabels(w http.ResponseWriter, r *http.Request) {
	if s.Labeler == nil {
		http.Error(w, "labeler not configured", http.StatusServiceUnavailable)
		return
	}

	var cursor int64
	if c := r.URL.Query().Get("cursor"); c != "" {
		if n, err := strconv.ParseInt(c, 10, 64); err == nil && n >= 0 {
			cursor = n
		}
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("subscribeLabels upgrade", "err", err)
		return
	}
	defer conn.Close()

	liveCh := make(chan LabelRow, liveBufferSize)
	s.Labeler.Subscribe(liveCh)
	defer s.Labeler.Unsubscribe(liveCh)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Reader goroutine: gorilla requires reads to detect close; we ignore client
	// messages but must drain.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	lastSent := cursor
	for {
		batch, err := s.Store.ListLabelsSince(ctx, lastSent, backlogChunkSize)
		if err != nil {
			slog.Warn("backlog read", "err", err)
			return
		}
		if len(batch) == 0 {
			break
		}
		for _, row := range batch {
			if err := writeLabelFrame(conn, row); err != nil {
				return
			}
			lastSent = row.ID
		}
		if len(batch) < backlogChunkSize {
			break
		}
	}

	// Drain liveCh of any rows that arrived during backlog read; skip dupes.
	conn.SetWriteDeadline(time.Time{})
	for {
		select {
		case <-ctx.Done():
			return
		case row := <-liveCh:
			if row.ID <= lastSent {
				continue
			}
			if err := writeLabelFrame(conn, row); err != nil {
				return
			}
			lastSent = row.ID
		}
	}
}

// writeLabelFrame serializes a label row as a #labels frame (CBOR header +
// CBOR body) and writes it as a single binary WS message.
func writeLabelFrame(conn *websocket.Conn, row LabelRow) error {
	header := events.EventHeader{Op: events.EvtKindMessage, MsgType: "#labels"}
	body := comatproto.LabelSubscribeLabels_Labels{
		Seq:    row.ID,
		Labels: []*comatproto.LabelDefs_Label{labelRowToLexicon(row)},
	}

	var buf bytes.Buffer
	cw := cbg.NewCborWriter(&buf)
	if err := header.MarshalCBOR(cw); err != nil {
		return err
	}
	if err := body.MarshalCBOR(cw); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}
