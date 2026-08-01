package main

import (
	"encoding/json"
	"testing"
)

func TestBuildSaveContentRequiresContent(t *testing.T) {
	if _, err := buildSaveContent(nil); err == nil {
		t.Fatal("expected missing content error")
	}
}

func TestSaveContentNSIDRequiresType(t *testing.T) {
	_, err := saveContentNSID(json.RawMessage(`{"image":{}}`))
	if err == nil {
		t.Fatal("expected missing $type error")
	}
}

func TestDecodeSaveImageContent(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}}`)

	contentAny, err := buildSaveContent(contentRaw)
	if err != nil {
		t.Fatalf("buildSaveContent failed: %v", err)
	}
	contentMap, ok := contentAny.(map[string]any)
	if !ok {
		t.Fatalf("expected map content, got %T", contentAny)
	}
	if got := contentMap["$type"]; got != saveContentImageNSID {
		t.Fatalf("unexpected content type: %#v", got)
	}

	contentType, err := saveContentNSID(contentRaw)
	if err != nil {
		t.Fatalf("saveContentNSID failed: %v", err)
	}
	if contentType != saveContentImageNSID {
		t.Fatalf("unexpected content type: %q", contentType)
	}

	content, err := decodeSaveImageContent(contentRaw)
	if err != nil {
		t.Fatalf("decodeSaveImageContent failed: %v", err)
	}
	if content == nil {
		t.Fatal("expected image content")
	}
	if content.Image.Ref["$link"] != "bafkcid" {
		t.Fatalf("unexpected blob CID: %q", content.Image.Ref["$link"])
	}
	if content.Image.Type != "blob" {
		t.Fatalf("unexpected blob type: %q", content.Image.Type)
	}
}

func TestBuildSaveContentWithAttributionPreservesNestedAttribution(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}}`)

	contentAny, err := buildSaveContentWithAttribution(contentRaw, nil, false)
	if err != nil {
		t.Fatalf("buildSaveContentWithAttribution failed: %v", err)
	}
	content, ok := contentAny.(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent, got %T", contentAny)
	}
	if content.Attribution == nil {
		t.Fatal("expected nested attribution")
	}
	if content.Attribution.Credit != "nested" {
		t.Fatalf("unexpected preserved attribution: %q", content.Attribution.Credit)
	}
	if content.Image.Type != "blob" {
		t.Fatalf("expected blob type to be preserved, got %q", content.Image.Type)
	}
}

func TestBuildSaveContentWithAttributionOverridesNestedAttribution(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}}`)

	contentAny, err := buildSaveContentWithAttribution(contentRaw, &saveAttribution{Credit: "updated"}, false)
	if err != nil {
		t.Fatalf("buildSaveContentWithAttribution failed: %v", err)
	}
	content, ok := contentAny.(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent, got %T", contentAny)
	}
	if content.Attribution == nil {
		t.Fatal("expected nested attribution")
	}
	if content.Attribution.Credit != "updated" {
		t.Fatalf("unexpected overridden attribution: %q", content.Attribution.Credit)
	}
	if content.Image.Type != "blob" {
		t.Fatalf("expected blob type to be preserved, got %q", content.Image.Type)
	}
}

func TestBuildSaveContentWithAttributionRepairsMissingBlobType(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}}`)

	contentAny, err := buildSaveContentWithAttribution(contentRaw, nil, false)
	if err != nil {
		t.Fatalf("buildSaveContentWithAttribution failed: %v", err)
	}
	content, ok := contentAny.(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent, got %T", contentAny)
	}
	if content.Image.Type != "blob" {
		t.Fatalf("expected repaired blob type, got %q", content.Image.Type)
	}
}

func TestBuildSaveContentWithAttributionOverwriteClearsNestedAttribution(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}}`)

	contentAny, err := buildSaveContentWithAttribution(contentRaw, nil, true)
	if err != nil {
		t.Fatalf("buildSaveContentWithAttribution failed: %v", err)
	}
	content, ok := contentAny.(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent, got %T", contentAny)
	}
	if content.Attribution != nil {
		t.Fatalf("expected cleared attribution, got %#v", content.Attribution)
	}
}

func TestBuildSaveViewMovesAttributionIntoImageContent(t *testing.T) {
	view := buildSaveView(
		SaveRow{
			URI:               "at://did:plc:test/is.currents.feed.save/123",
			BlobCID:           "bafkcid",
			AuthorDID:         "did:plc:test",
			ContentNSID:       saveContentImageNSID,
			AttributionCredit: "nested",
		},
		profileView{DID: "did:plc:test", Handle: "tester"},
		false,
		"https://cdn.example.com",
	)

	content, ok := view.Content.(imageView)
	if !ok {
		t.Fatalf("expected imageView content, got %T", view.Content)
	}
	if content.Attribution == nil {
		t.Fatal("expected image attribution on content view")
	}
	if content.Attribution.Credit != "nested" {
		t.Fatalf("unexpected attribution credit: %q", content.Attribution.Credit)
	}
}

func TestDecodeSaveImageContentExtractsAlt(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"alt":"a cat on a mat"}`)
	content, err := decodeSaveImageContent(contentRaw)
	if err != nil {
		t.Fatalf("decodeSaveImageContent failed: %v", err)
	}
	if content == nil {
		t.Fatal("expected image content")
	}
	if content.Alt != "a cat on a mat" {
		t.Fatalf("unexpected alt: %q", content.Alt)
	}
}

func TestBuildSaveViewSurfacesAlt(t *testing.T) {
	view := buildSaveView(
		SaveRow{
			URI:         "at://did:plc:test/is.currents.feed.save/123",
			BlobCID:     "bafkcid",
			AuthorDID:   "did:plc:test",
			ContentNSID: saveContentImageNSID,
			AltText:     "a cat on a mat",
		},
		profileView{DID: "did:plc:test", Handle: "tester"},
		false,
		"https://cdn.example.com",
	)
	content, ok := view.Content.(imageView)
	if !ok {
		t.Fatalf("expected imageView content, got %T", view.Content)
	}
	if content.Alt != "a cat on a mat" {
		t.Fatalf("unexpected alt on content view: %q", content.Alt)
	}
}

// Editing a save (e.g. changing its collection or attribution) rebuilds the
// content via buildSaveContentWithAttribution; alt text must survive the round-trip.
func TestBuildSaveContentWithAttributionPreservesAlt(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"alt":"keep me"}`)

	contentAny, err := buildSaveContentWithAttribution(contentRaw, &saveAttribution{Credit: "updated"}, false)
	if err != nil {
		t.Fatalf("buildSaveContentWithAttribution failed: %v", err)
	}
	content, ok := contentAny.(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent, got %T", contentAny)
	}
	if content.Alt != "keep me" {
		t.Fatalf("expected alt preserved through edit, got %q", content.Alt)
	}
}

func TestDecodeSaveImageContentSkipsNonImage(t *testing.T) {
	content, err := decodeSaveImageContent(json.RawMessage(`{"$type":"is.currents.content.note"}`))
	if err != nil {
		t.Fatalf("decodeSaveImageContent failed: %v", err)
	}
	if content != nil {
		t.Fatal("expected nil content for non-image record")
	}
}

func TestBuildSaveContentWithAlt(t *testing.T) {
	const blob = `"image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}`
	tests := []struct {
		name     string
		content  string
		alt      string
		wantAlt  string
		wantAttr string
	}{
		{
			name:    "sets alt on content that had none",
			content: `{"$type":"is.currents.content.image",` + blob + `}`,
			alt:     "a red bicycle",
			wantAlt: "a red bicycle",
		},
		{
			name:    "replaces existing alt",
			content: `{"$type":"is.currents.content.image",` + blob + `,"alt":"old"}`,
			alt:     "new",
			wantAlt: "new",
		},
		{
			name:    "empty alt clears the field",
			content: `{"$type":"is.currents.content.image",` + blob + `,"alt":"old"}`,
			alt:     "",
			wantAlt: "",
		},
		{
			name:     "leaves attribution untouched",
			content:  `{"$type":"is.currents.content.image",` + blob + `,"attribution":{"credit":"nested"}}`,
			alt:      "described",
			wantAlt:  "described",
			wantAttr: "nested",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentAny, err := buildSaveContentWithAlt(json.RawMessage(tt.content), tt.alt)
			if err != nil {
				t.Fatalf("buildSaveContentWithAlt failed: %v", err)
			}
			content, ok := contentAny.(*saveImageContent)
			if !ok {
				t.Fatalf("expected *saveImageContent, got %T", contentAny)
			}
			if content.Alt != tt.wantAlt {
				t.Fatalf("alt = %q, want %q", content.Alt, tt.wantAlt)
			}
			if tt.wantAttr == "" {
				if content.Attribution != nil {
					t.Fatalf("expected no attribution, got %+v", content.Attribution)
				}
			} else if content.Attribution == nil || content.Attribution.Credit != tt.wantAttr {
				t.Fatalf("attribution = %+v, want credit %q", content.Attribution, tt.wantAttr)
			}
			// The blob ref must survive — a lost ref would orphan the image.
			if content.Image.Type != "blob" || content.Image.Ref["$link"] != "bafkcid" {
				t.Fatalf("blob ref not preserved: %+v", content.Image)
			}
		})
	}
}

func TestBuildSaveContentWithAltRepairsMissingBlobType(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}}`)

	contentAny, err := buildSaveContentWithAlt(contentRaw, "described")
	if err != nil {
		t.Fatalf("buildSaveContentWithAlt failed: %v", err)
	}
	content, ok := contentAny.(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent, got %T", contentAny)
	}
	if content.Image.Type != "blob" {
		t.Fatalf("expected blob type to be repaired, got %q", content.Image.Type)
	}
}
