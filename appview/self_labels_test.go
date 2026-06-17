package main

import (
	"slices"
	"testing"
)

// parseSelfLabels is the form-input gate that keeps self-label propagation from
// becoming an arbitrary-label-injection vector: only canonical content-warning
// vals survive, deduped, in order.
func TestParseSelfLabels(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single valid", "nudity", []string{"nudity"}},
		{"multiple valid", "porn,graphic-media", []string{"porn", "graphic-media"}},
		{"allows ai provenance", "currents-ai-generated", []string{"currents-ai-generated"}},
		{"drops unknown", "nudity,malware,porn", []string{"nudity", "porn"}},
		{"dedupes", "nudity,nudity", []string{"nudity"}},
		{"trims whitespace", " nudity , porn ", []string{"nudity", "porn"}},
		{"all unknown -> nil", "foo,bar", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseSelfLabels(c.in); !slices.Equal(got, c.want) {
				t.Fatalf("parseSelfLabels(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// The backfill isolates self-labels from moderator/auto labels by val, so its
// val list must stay in lockstep with the write-path gate.
func TestSelfLabelValsListMatchesAllowed(t *testing.T) {
	list := selfLabelValsList()
	if len(list) != len(allowedSelfLabelVals) {
		t.Fatalf("selfLabelValsList len = %d, want %d", len(list), len(allowedSelfLabelVals))
	}
	for _, v := range list {
		if _, ok := allowedSelfLabelVals[v]; !ok {
			t.Fatalf("selfLabelValsList returned %q not in allowedSelfLabelVals", v)
		}
	}
}

// labelRowsHaveVal drives the backfill's idempotency: a sibling already carrying
// the val is skipped on re-run.
func TestLabelRowsHaveVal(t *testing.T) {
	rows := []LabelRow{{Val: "porn"}, {Val: "nudity"}}
	if !labelRowsHaveVal(rows, "nudity") {
		t.Fatal("expected nudity to be present")
	}
	if labelRowsHaveVal(rows, "graphic-media") {
		t.Fatal("did not expect graphic-media")
	}
	if labelRowsHaveVal(nil, "porn") {
		t.Fatal("did not expect any val in empty rows")
	}
}
