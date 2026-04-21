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
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}}`)

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
}

func TestEffectiveSaveAttributionPrefersNestedContent(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}}`)
	legacy := &saveAttribution{Credit: "legacy"}

	attr, err := effectiveSaveAttribution(contentRaw, legacy)
	if err != nil {
		t.Fatalf("effectiveSaveAttribution failed: %v", err)
	}
	if attr == nil {
		t.Fatal("expected attribution")
	}
	if attr.Credit != "nested" {
		t.Fatalf("expected nested attribution, got %q", attr.Credit)
	}
}

func TestEffectiveSaveAttributionFallsBackToLegacy(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}}`)
	legacy := &saveAttribution{License: "CC BY 4.0"}

	attr, err := effectiveSaveAttribution(contentRaw, legacy)
	if err != nil {
		t.Fatalf("effectiveSaveAttribution failed: %v", err)
	}
	if attr == nil {
		t.Fatal("expected attribution")
	}
	if attr.License != "CC BY 4.0" {
		t.Fatalf("expected legacy attribution, got %q", attr.License)
	}
}

func TestBuildSaveContentWithAttributionMigratesLegacy(t *testing.T) {
	contentRaw := json.RawMessage(`{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}}`)
	legacy := &saveAttribution{Credit: "legacy"}

	contentAny, err := buildSaveContentWithAttribution(contentRaw, nil, legacy)
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
	if content.Attribution.Credit != "legacy" {
		t.Fatalf("unexpected migrated attribution: %q", content.Attribution.Credit)
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

func TestDecodeSaveImageContentSkipsNonImage(t *testing.T) {
	content, err := decodeSaveImageContent(json.RawMessage(`{"$type":"is.currents.content.note"}`))
	if err != nil {
		t.Fatalf("decodeSaveImageContent failed: %v", err)
	}
	if content != nil {
		t.Fatal("expected nil content for non-image record")
	}
}
