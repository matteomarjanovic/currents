package main

import "testing"

func TestPDSServiceDID(t *testing.T) {
	cases := []struct {
		host    string
		want    string
		wantErr bool
	}{
		{"https://morel.us-east.host.bsky.network", "did:web:morel.us-east.host.bsky.network", false},
		{"https://morel.us-east.host.bsky.network/", "did:web:morel.us-east.host.bsky.network", false},
		{"https://pds.example.com:8443", "did:web:pds.example.com", false}, // port dropped
		{"http://localhost:3000", "did:web:localhost", false},
		{"", "", true},
		{"not a url", "", true},
	}
	for _, tc := range cases {
		got, err := pdsServiceDID(tc.host)
		if tc.wantErr {
			if err == nil {
				t.Errorf("pdsServiceDID(%q): expected error, got %q", tc.host, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("pdsServiceDID(%q): unexpected error: %v", tc.host, err)
			continue
		}
		if got != tc.want {
			t.Errorf("pdsServiceDID(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}
