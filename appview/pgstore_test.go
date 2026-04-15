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
