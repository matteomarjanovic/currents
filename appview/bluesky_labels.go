package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/labeling"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/events"
	"github.com/gorilla/websocket"
)

const blueskyLabelerDID = "did:plc:ar7c4by46qjdydhdevvrndac"

type blueskyLabel struct {
	Src string     `json:"src"`
	URI string     `json:"uri"`
	CID string     `json:"cid"`
	Val string     `json:"val"`
	CTS time.Time  `json:"cts"`
	Exp *time.Time `json:"exp"`
	Neg bool       `json:"neg"`
}

func parseBlueskyLabel(raw *comatproto.LabelDefs_Label, key atcrypto.PublicKey) (blueskyLabel, bool) {
	if raw == nil || raw.Src != blueskyLabelerDID {
		return blueskyLabel{}, false
	}
	if strings.HasPrefix(raw.Uri, "did:") {
		if _, err := syntax.ParseDID(raw.Uri); err != nil || raw.Cid != nil {
			return blueskyLabel{}, false
		}
	} else if u, err := syntax.ParseATURI(raw.Uri); err != nil || u.Collection().String() != saveNSID || !u.Authority().IsDID() || u.RecordKey() == "" {
		return blueskyLabel{}, false
	}
	signed := labeling.FromLexicon(raw)
	if signed.VerifySyntax() != nil || signed.VerifySignature(key) != nil {
		return blueskyLabel{}, false
	}
	cts, err := time.Parse(time.RFC3339Nano, raw.Cts)
	if err != nil {
		return blueskyLabel{}, false
	}
	label := blueskyLabel{Src: raw.Src, URI: raw.Uri, Val: raw.Val, CTS: cts, Neg: raw.Neg != nil && *raw.Neg}
	if raw.Cid != nil {
		label.CID = *raw.Cid
	}
	if raw.Exp != nil {
		exp, err := time.Parse(time.RFC3339Nano, *raw.Exp)
		if err != nil {
			return blueskyLabel{}, false
		}
		label.Exp = &exp
	}
	return label, true
}

func runBlueskyLabelStream(ctx context.Context, dir identity.Directory, store *PgStore) {
	for ctx.Err() == nil {
		if err := tailBlueskyLabels(ctx, dir, store); err != nil && ctx.Err() == nil {
			slog.Warn("Bluesky label stream reconnecting", "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
		}
	}
}

func tailBlueskyLabels(ctx context.Context, dir identity.Directory, store *PgStore) error {
	ident, err := dir.LookupDID(ctx, blueskyLabelerDID)
	if err != nil {
		return err
	}
	key, err := ident.GetPublicKey("atproto_label")
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(ident.GetServiceEndpoint("atproto_labeler"))
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return fmt.Errorf("missing Bluesky HTTPS labeler endpoint")
	}
	endpoint.Scheme = "wss"
	endpoint.Path = "/xrpc/com.atproto.label.subscribeLabels"
	cursor, err := store.blueskyLabelCursor(ctx)
	if err != nil {
		return err
	}
	if cursor > 0 {
		endpoint.RawQuery = url.Values{"cursor": {strconv.FormatInt(cursor, 10)}}.Encode()
	}
	dialer := websocket.Dialer{NetDialContext: publicDialer.DialContext, HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.DialContext(ctx, endpoint.String(), nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	slog.Info("Bluesky label stream connected", "cursor", cursor)
	return consumeBlueskyLabels(ctx, conn, key, store, cursor)
}

func consumeBlueskyLabels(ctx context.Context, conn *websocket.Conn, key atcrypto.PublicKey, store *PgStore, cursor int64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn.SetReadLimit(4 << 20)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(time.Minute))
	})
	if err := conn.SetReadDeadline(time.Now().Add(time.Minute)); err != nil {
		return err
	}
	frames := make(chan events.XRPCStreamEvent, 32)
	fail := make(chan error, 1)
	go func() {
		defer close(frames)
		for {
			_, reader, err := conn.NextReader()
			if err != nil {
				fail <- err
				return
			}
			var frame events.XRPCStreamEvent
			if err := frame.Deserialize(reader); err != nil {
				fail <- err
				return
			}
			select {
			case frames <- frame:
			case <-ctx.Done():
				return
			}
		}
	}()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	var batch []blueskyLabel
	savedCursor := cursor
	flush := func() error {
		if cursor == savedCursor {
			return nil
		}
		if err := store.saveBlueskyLabelBatch(ctx, cursor, batch); err != nil {
			return err
		}
		savedCursor, batch = cursor, batch[:0]
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return err
			}
		case <-tick.C:
			if err := flush(); err != nil {
				return err
			}
		case frame, open := <-frames:
			if !open {
				if err := flush(); err != nil {
					return err
				}
				select {
				case err := <-fail:
					return err
				default:
					return ctx.Err()
				}
			}
			if frame.Error != nil {
				return fmt.Errorf("Bluesky label stream: %s", frame.Error.Error)
			}
			if frame.RepoInfo != nil && frame.RepoInfo.Name == "OutdatedCursor" {
				slog.Warn("Bluesky label history gap", "cursor", cursor)
			}
			if frame.LabelLabels == nil || frame.LabelLabels.Seq <= cursor {
				continue
			}
			cursor = frame.LabelLabels.Seq
			for _, raw := range frame.LabelLabels.Labels {
				if label, ok := parseBlueskyLabel(raw, key); ok {
					batch = append(batch, label)
				}
			}
			if len(batch) >= 128 {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
}

func (s *PgStore) blueskyLabelCursor(ctx context.Context) (int64, error) {
	var cursor int64
	err := s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT seq FROM external_label_cursor WHERE src = $1), 0)`, blueskyLabelerDID).Scan(&cursor)
	return cursor, err
}

func (s *PgStore) saveBlueskyLabelBatch(ctx context.Context, cursor int64, labels []blueskyLabel) error {
	raw, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		WITH received AS (
			SELECT DISTINCT ON (src, uri, cid, val) src, uri, cid, val, cts, exp, neg
			FROM jsonb_to_recordset(COALESCE(NULLIF($1::jsonb, 'null'), '[]'))
				AS l(src text, uri text, cid text, val text, cts timestamptz, exp timestamptz, neg boolean)
			WHERE uri LIKE 'at://%' OR EXISTS (SELECT 1 FROM "user" WHERE did = l.uri)
			ORDER BY src, uri, cid, val, cts DESC, neg DESC
		), saved AS (
			INSERT INTO external_label (src, uri, cid, val, cts, exp, neg)
				SELECT src, uri, cid, val, cts, exp, neg FROM received
			ON CONFLICT (src, uri, cid, val) DO UPDATE
				SET cts = EXCLUDED.cts, exp = EXCLUDED.exp, neg = EXCLUDED.neg
				WHERE (EXCLUDED.cts, EXCLUDED.neg) > (external_label.cts, external_label.neg)
		)
		INSERT INTO external_label_cursor (src, seq) VALUES ($2, $3)
		ON CONFLICT (src) DO UPDATE SET seq = GREATEST(EXCLUDED.seq, external_label_cursor.seq)
	`, string(raw), blueskyLabelerDID, cursor)
	return err
}

// A CID-bound event applies only to its record version. URI-wide events,
// including negations, apply to every version of that URI.
func activeBlueskyLabels(labels []blueskyLabel, cid string, now time.Time) []labelView {
	latest := make(map[string]blueskyLabel)
	for _, label := range labels {
		if label.CID != "" && label.CID != cid {
			continue
		}
		prev, ok := latest[label.Val]
		if !ok || label.CTS.After(prev.CTS) || (label.CTS.Equal(prev.CTS) && label.Neg) {
			latest[label.Val] = label
		}
	}
	var active []labelView
	for _, label := range latest {
		if !label.Neg && (label.Exp == nil || now.Before(*label.Exp)) {
			view := labelView{Src: label.Src, URI: label.URI, Val: label.Val, CTS: label.CTS.UTC().Format(time.RFC3339Nano)}
			if label.Exp != nil {
				view.Exp = label.Exp.UTC().Format(time.RFC3339Nano)
			}
			active = append(active, view)
		}
	}
	slices.SortFunc(active, func(a, b labelView) int { return strings.Compare(a.Val, b.Val) })
	return active
}

func (s *PgStore) getBlueskyLabels(ctx context.Context, targets map[string]string) (map[string][]labelView, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	subjects := make([]string, 0, len(targets)*2)
	accounts := make(map[string]bool)
	for uri, did := range targets {
		subjects = append(subjects, uri)
		if !accounts[did] {
			subjects = append(subjects, did)
			accounts[did] = true
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT l.src, l.uri, l.cid, l.val, l.cts, l.exp, l.neg, COALESCE(s.record_cid, '')
		FROM external_label l LEFT JOIN save s ON s.uri = l.uri AND l.cid <> ''
		WHERE l.src = $1 AND l.uri = ANY($2)
	`, blueskyLabelerDID, subjects)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySubject := make(map[string][]blueskyLabel)
	versions := make(map[string]string)
	for rows.Next() {
		var l blueskyLabel
		var version string
		if err := rows.Scan(&l.Src, &l.URI, &l.CID, &l.Val, &l.CTS, &l.Exp, &l.Neg, &version); err != nil {
			return nil, err
		}
		bySubject[l.URI] = append(bySubject[l.URI], l)
		if version != "" {
			versions[l.URI] = version
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	out := make(map[string][]labelView)
	byDID := make(map[string][]labelView, len(accounts))
	for did := range accounts {
		byDID[did] = activeBlueskyLabels(bySubject[did], "", now)
	}
	for uri, did := range targets {
		record := activeBlueskyLabels(bySubject[uri], versions[uri], now)
		if account := byDID[did]; len(account)+len(record) > 0 {
			out[uri] = append(record, account...)
		}
	}
	return out, nil
}
