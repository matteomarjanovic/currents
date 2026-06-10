package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/labeling"
)

// XRPCQueryLabels implements com.atproto.label.queryLabels.
//   ?uriPatterns=<pat>&uriPatterns=<pat>...  (required, at least one)
//   ?sources=<did>&sources=<did>...           (optional; default = all sources)
//   ?limit=50&cursor=<int>                    (optional)
func (s *Server) XRPCQueryLabels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	patterns := q["uriPatterns"]
	if len(patterns) == 0 {
		http.Error(w, "uriPatterns required", http.StatusBadRequest)
		return
	}
	sources := q["sources"]

	limit := 50
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 250 {
				n = 250
			}
			limit = n
		}
	}
	var cursor int64
	if c := q.Get("cursor"); c != "" {
		if n, err := strconv.ParseInt(c, 10, 64); err == nil && n >= 0 {
			cursor = n
		}
	}

	rows, next, err := s.Store.QueryLabels(r.Context(), patterns, sources, cursor, limit)
	if err != nil {
		slog.Error("QueryLabels", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	labels := make([]*comatproto.LabelDefs_Label, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, labelRowToLexicon(row))
	}
	out := &comatproto.LabelQueryLabels_Output{
		Labels: labels,
	}
	if next > 0 {
		c := strconv.FormatInt(next, 10)
		out.Cursor = &c
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// labelRowToLexicon converts a DB LabelRow to the indigo lexicon shape used
// in queryLabels and subscribeLabels responses.
func labelRowToLexicon(row LabelRow) *comatproto.LabelDefs_Label {
	l := labeling.Label{
		Version:   int64(row.Ver),
		SourceDID: row.Src,
		URI:       row.URI,
		Val:       row.Val,
		CreatedAt: row.CTS.UTC().Format(time.RFC3339Nano),
		Sig:       row.Sig,
	}
	if row.CID != "" {
		cid := row.CID
		l.CID = &cid
	}
	if row.Neg {
		t := true
		l.Negated = &t
	}
	if row.Exp != nil {
		exp := row.Exp.UTC().Format(time.RFC3339Nano)
		l.ExpiresAt = &exp
	}
	out := l.ToLexicon()
	return &out
}
