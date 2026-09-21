package main

import "testing"

func TestNormalizePinterestUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "username", input: " giova_merlo ", want: "giova_merlo"},
		{name: "at username", input: "@giova_merlo", want: "giova_merlo"},
		{name: "profile URL", input: "https://www.pinterest.com/giova_merlo/", want: "giova_merlo"},
		{name: "localized board URL", input: "https://it.pinterest.com/giova_merlo/consolle/.", want: "giova_merlo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePinterestUsername(tt.input)
			if err != nil {
				t.Fatalf("normalizePinterestUsername() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizePinterestUsername() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePinterestUsernameRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "https://example.com/giova_merlo/", "giova_merlo/consolle"} {
		t.Run(input, func(t *testing.T) {
			if _, err := normalizePinterestUsername(input); err == nil {
				t.Fatalf("normalizePinterestUsername(%q) unexpectedly succeeded", input)
			}
		})
	}
}
