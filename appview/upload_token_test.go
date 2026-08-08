package main

import (
	"encoding/json"
	"testing"
)

func TestExtractImageBlobRef(t *testing.T) {
	valid := `{
		"$type": "is.currents.feed.save",
		"content": {
			"$type": "is.currents.content.image",
			"image": {"$type":"blob","ref":{"$link":"bafyblobcid"},"mimeType":"image/jpeg","size":42},
			"alt": "hi"
		},
		"createdAt": "2026-01-01T00:00:00.000Z"
	}`
	ref, err := extractImageBlobRef(json.RawMessage(valid))
	if err != nil {
		t.Fatalf("extractImageBlobRef(valid): %v", err)
	}
	m, ok := ref.(map[string]any)
	if !ok {
		t.Fatalf("ref is %T, want map", ref)
	}
	if m["$type"] != "blob" {
		t.Errorf("$type = %v, want blob", m["$type"])
	}
	if link := m["ref"].(map[string]any)["$link"]; link != "bafyblobcid" {
		t.Errorf("ref.$link = %v, want bafyblobcid", link)
	}

	for name, value := range map[string]string{
		"no content": `{"$type":"is.currents.feed.save"}`,
		"no image":   `{"content":{"$type":"is.currents.content.image","alt":"x"}}`,
		"bad json":   `{not json`,
	} {
		if _, err := extractImageBlobRef(json.RawMessage(value)); err == nil {
			t.Errorf("extractImageBlobRef(%s): expected error, got nil", name)
		}
	}
}

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
