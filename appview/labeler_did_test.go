package main

import "testing"

func TestLabelerDIDHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"did:web:moderation.currents.is", "moderation.currents.is"},
		{"did:web:moderation.currents.is%3A8080", "moderation.currents.is:8080"},
		{"did:web:localhost%3A8080", "localhost:8080"},
		{"did:plc:abc123", ""},
		{"", ""},
		{"did:web:", ""},
	}
	for _, c := range cases {
		if got := labelerDIDHost(c.in); got != c.want {
			t.Errorf("labelerDIDHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
