package main

import "testing"

func TestValidSaveSuggestionMode(t *testing.T) {
	for _, mode := range []string{"last-used", "recommended", "recommended-then-last-used"} {
		if !validSaveSuggestionMode(mode) {
			t.Errorf("validSaveSuggestionMode(%q) = false", mode)
		}
	}
	for _, mode := range []string{"", "recent", "anything"} {
		if validSaveSuggestionMode(mode) {
			t.Errorf("validSaveSuggestionMode(%q) = true", mode)
		}
	}
}
