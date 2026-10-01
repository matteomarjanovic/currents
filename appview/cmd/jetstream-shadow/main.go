// jetstream-shadow observes Jetstream v2 alongside TAP without writing to Currents.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type event struct {
	Type    string  `json:"$type"`
	Payload payload `json:"payload"`
}

type payload struct {
	Type        string `json:"$type"`
	Seq         int64  `json:"seq"`
	DID         string `json:"did"`
	WitnessedAt string `json:"witnessedAt"`
	Operation   string `json:"operation"`
	Collection  string `json:"collection"`
	Rkey        string `json:"rkey"`
	Rev         string `json:"rev"`
	CID         string `json:"cid"`
}

type report struct {
	Events    int            `json:"events"`
	Bytes     int            `json:"bytes"`
	Cursor    int64          `json:"cursor"`
	First     string         `json:"first,omitempty"`
	Last      string         `json:"last,omitempty"`
	Kinds     map[string]int `json:"kinds"`
	TAPChecks map[string]int `json:"tapChecks,omitempty"`
	pending   []pendingCheck
}

type pendingCheck struct {
	event payload
	due   time.Time
}

var errStream = errors.New("Jetstream stream error")

func streamURL(endpoint, did string, cursor int64) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return "", fmt.Errorf("Jetstream endpoint must use ws or wss")
	}
	q := u.Query()
	q.Set("collections", "is.currents.*")
	if did != "" {
		q.Set("dids", did)
	}
	if cursor != 0 {
		q.Set("cursor", strconv.FormatInt(cursor, 10))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// A newer TAP repo revision may contain a later change to the same record.
// Such a difference is inconclusive when replaying older Jetstream events.
func classify(p payload, state, tapRev, tapCID string) string {
	if state == "" {
		return "untracked"
	}
	if state != "active" {
		return "tap-" + state
	}
	if tapRev < p.Rev {
		return "tap-behind"
	}
	want := p.CID
	if p.Operation == "delete" {
		want = ""
	}
	if tapCID == want {
		return "match"
	}
	if tapRev == p.Rev {
		return "mismatch"
	}
	return "changed-later-or-missing"
}

func tapStatus(ctx context.Context, db *pgxpool.Pool, p payload) (string, error) {
	var state, rev, cid string
	err := db.QueryRow(ctx, `SELECT r.state, COALESCE(r.rev, ''), COALESCE(rr.cid, '')
		FROM repos r LEFT JOIN repo_records rr
		  ON rr.did = r.did AND rr.collection = $2 AND rr.rkey = $3
		WHERE r.did = $1`, p.DID, p.Collection, p.Rkey).Scan(&state, &rev, &cid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "untracked", nil
	}
	if err != nil {
		return "", err
	}
	return classify(p, state, rev, cid), nil
}

func checkPending(ctx context.Context, db *pgxpool.Pool, out *report) error {
	remaining := out.pending[:0]
	for i, check := range out.pending {
		if time.Now().Before(check.due) {
			remaining = append(remaining, check)
			continue
		}
		status, err := tapStatus(ctx, db, check.event)
		if err != nil {
			out.pending = append(remaining, out.pending[i:]...)
			return err
		}
		out.TAPChecks[status]++
		fmt.Printf("tap-check seq=%d uri=at://%s/%s/%s tap=%s\n",
			check.event.Seq, check.event.DID, check.event.Collection, check.event.Rkey, status)
	}
	out.pending = remaining
	return nil
}

func consume(ctx context.Context, conn *websocket.Conn, db *pgxpool.Pool, out *report, maxEvents int, settle time.Duration) error {
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	defer conn.Close()

	for {
		if err := conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return err
		}
		_, raw, err := conn.ReadMessage()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		var e event
		if err := json.Unmarshal(raw, &e); err != nil {
			return err
		}
		if e.Type == "error" || strings.HasSuffix(e.Payload.Type, "#info") {
			return fmt.Errorf("%w or cursor warning: %s", errStream, string(raw))
		}
		if db != nil {
			if err := checkPending(ctx, db, out); err != nil {
				return err
			}
		}
		p := e.Payload
		if p.Seq <= out.Cursor {
			continue // Cursors are inclusive after reconnect.
		}
		kind := strings.TrimPrefix(p.Type, "network.bsky.jetstream.subscribeEvents#")
		status := ""
		uri := ""
		witnessed, err := time.Parse(time.RFC3339Nano, p.WitnessedAt)
		if err != nil {
			return fmt.Errorf("invalid Jetstream witnessedAt: %w", err)
		}
		if kind == "commit" {
			uri = "at://" + p.DID + "/" + p.Collection + "/" + p.Rkey
			if db != nil {
				if time.Now().Before(witnessed.Add(settle)) {
					status = "pending"
					out.pending = append(out.pending, pendingCheck{p, witnessed.Add(settle)})
				} else {
					status, err = tapStatus(ctx, db, p)
					if err != nil {
						return err // Replay this event on reconnect.
					}
					out.TAPChecks[status]++
				}
			}
		}
		out.Events++
		out.Bytes += len(raw)
		out.Cursor = p.Seq
		out.Kinds[kind]++
		if out.First == "" {
			out.First = p.WitnessedAt
		}
		out.Last = p.WitnessedAt
		fmt.Printf("seq=%d kind=%s did=%s uri=%s lag=%s tap=%s\n",
			p.Seq, kind, p.DID, uri, time.Since(witnessed).Round(time.Millisecond), status)
		if maxEvents > 0 && out.Events >= maxEvents {
			return nil
		}
	}
}

func run(ctx context.Context, endpoint, did string, cursor int64, db *pgxpool.Pool, maxEvents int, settle time.Duration, out *report) error {
	dialer := websocket.Dialer{Subprotocols: []string{"xrpc.v1.json"}}
	for {
		target, err := streamURL(endpoint, did, cursor)
		if err != nil {
			return err
		}
		conn, resp, err := dialer.DialContext(ctx, target, nil)
		if err == nil {
			err = consume(ctx, conn, db, out, maxEvents, settle)
			if out.Cursor != 0 {
				cursor = out.Cursor
			}
			if err == nil || ctx.Err() != nil || maxEvents > 0 && out.Events >= maxEvents {
				return nil
			}
			if errors.Is(err, errStream) {
				return err
			}
			log.Printf("Jetstream disconnected at seq %d: %v", cursor, err)
		} else if resp != nil {
			resp.Body.Close()
			return fmt.Errorf("Jetstream HTTP %d: %w", resp.StatusCode, err)
		} else if ctx.Err() != nil {
			return nil
		} else {
			log.Printf("Jetstream dial failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
}

func main() {
	endpoint := flag.String("endpoint", "wss://jetstream.us-east.bsky.network/xrpc/network.bsky.jetstream.subscribeEvents", "Jetstream v2 WebSocket endpoint")
	did := flag.String("did", "", "limit to one DID")
	from := flag.String("from", "", "replay from an RFC3339 timestamp within the live lookback window")
	cursor := flag.Int64("cursor", 0, "resume from a Jetstream sequence number")
	duration := flag.Duration("duration", 2*time.Minute, "observation duration; 0 runs until interrupted")
	maxEvents := flag.Int("max-events", 0, "stop after this many events; 0 has no event limit")
	databaseURL := flag.String("database-url", "", "optional PostgreSQL URL for read-only TAP comparisons")
	settle := flag.Duration("settle", 15*time.Second, "wait this long before comparing live events with TAP")
	flag.Parse()
	if *from != "" && *cursor != 0 {
		log.Fatal("use either --from or --cursor")
	}
	if *from != "" {
		t, err := time.Parse(time.RFC3339, *from)
		if err != nil {
			log.Fatal(err)
		}
		*cursor = t.UnixMicro()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	var db *pgxpool.Pool
	if *databaseURL != "" {
		cfg, err := pgxpool.ParseConfig(*databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		if cfg.ConnConfig.RuntimeParams == nil {
			cfg.ConnConfig.RuntimeParams = make(map[string]string)
		}
		cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
		db, err = pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		if err := db.Ping(ctx); err != nil {
			log.Fatal(err)
		}
	}
	out := report{Kinds: make(map[string]int), TAPChecks: make(map[string]int)}
	if *from == "" {
		out.Cursor = *cursor // The sequence cursor is inclusive.
	}
	if err := run(ctx, *endpoint, *did, *cursor, db, *maxEvents, *settle, &out); err != nil {
		log.Print(err)
		os.Exit(1)
	}
	for len(out.pending) > 0 {
		next := out.pending[0].due
		for _, check := range out.pending[1:] {
			if check.due.Before(next) {
				next = check.due
			}
		}
		if wait := time.Until(next); wait > 0 {
			time.Sleep(wait)
		}
		checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := checkPending(checkCtx, db, &out)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		log.Fatal(err)
	}
}
