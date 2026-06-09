package main

import "testing"

func TestThresholdBoundaries(t *testing.T) {
	// Spot-check the three bands per harm axis so the constants don't drift silently.
	// All three classifiers (NSFW, violence, AI-gen) share the same threshold ladder.
	cases := []struct {
		name      string
		score     float32
		outcome   HarmOutcome
	}{
		{"below suspected", 0.50, HarmNone},
		{"suspected band", 0.80, HarmSuspected},
		{"auto-apply", 0.92, HarmAutoApply},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyHarmScore(c.score); got != c.outcome {
				t.Errorf("ClassifyHarmScore(%v) = %v, want %v", c.score, got, c.outcome)
			}
		})
	}
}
