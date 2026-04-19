package main

import "testing"

func TestSearchSavesEFSearch(t *testing.T) {
	tests := []struct {
		name       string
		offset     int
		fetchLimit int
		want       int
	}{
		{name: "minimum floor", offset: 0, fetchLimit: 50, want: 100},
		{name: "deep page scales", offset: 150, fetchLimit: 200, want: 350},
		{name: "deep page with exclude fetch", offset: 400, fetchLimit: 300, want: 700},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchSavesEFSearch(tt.offset, tt.fetchLimit); got != tt.want {
				t.Fatalf("searchSavesEFSearch(%d, %d) = %d, want %d", tt.offset, tt.fetchLimit, got, tt.want)
			}
		})
	}
}

func TestSearchSavesQueryLimit(t *testing.T) {
	tests := []struct {
		name               string
		limit              int
		excludeViewerSaves bool
		want               int
	}{
		{name: "default page fetches one extra", limit: 50, want: 51},
		{name: "exclude viewer saves keeps extra headroom", limit: 50, excludeViewerSaves: true, want: 100},
		{name: "small page still overfetches", limit: 1, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchSavesQueryLimit(tt.limit, tt.excludeViewerSaves); got != tt.want {
				t.Fatalf("searchSavesQueryLimit(%d, %t) = %d, want %d", tt.limit, tt.excludeViewerSaves, got, tt.want)
			}
		})
	}
}

func TestSearchSavesMaxScanTuples(t *testing.T) {
	tests := []struct {
		name       string
		offset     int
		fetchLimit int
		want       int
	}{
		{name: "minimum floor", offset: 0, fetchLimit: 50, want: 20000},
		{name: "moderate depth still uses floor", offset: 150, fetchLimit: 200, want: 20000},
		{name: "deep page grows scan budget", offset: 900, fetchLimit: 200, want: 22000},
		{name: "very deep page scales further", offset: 4000, fetchLimit: 300, want: 86000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchSavesMaxScanTuples(tt.offset, tt.fetchLimit); got != tt.want {
				t.Fatalf("searchSavesMaxScanTuples(%d, %d) = %d, want %d", tt.offset, tt.fetchLimit, got, tt.want)
			}
		})
	}
}

func TestTrimANNPage(t *testing.T) {
	rows := []SaveRow{{URI: "a"}, {URI: "b"}, {URI: "c"}}

	page := trimANNPage(rows, 2)
	if !page.HasMore {
		t.Fatal("trimANNPage should report hasMore when an extra row is present")
	}
	if len(page.Rows) != 2 {
		t.Fatalf("len(page.Rows) = %d, want 2", len(page.Rows))
	}
	if page.Rows[0].URI != "a" || page.Rows[1].URI != "b" {
		t.Fatalf("unexpected trimmed rows: %#v", page.Rows)
	}

	page = trimANNPage(rows[:2], 2)
	if page.HasMore {
		t.Fatal("trimANNPage unexpectedly reported hasMore for an exact page")
	}
	if len(page.Rows) != 2 {
		t.Fatalf("len(page.Rows) = %d, want 2", len(page.Rows))
	}

	page = trimANNPage(rows[:1], 2)
	if page.HasMore {
		t.Fatal("trimANNPage unexpectedly reported hasMore for a short page")
	}
	if len(page.Rows) != 1 || page.Rows[0].URI != "a" {
		t.Fatalf("unexpected short page rows: %#v", page.Rows)
	}
}
