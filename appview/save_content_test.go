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

func TestDecodeSaveImageContentSkipsNonImage(t *testing.T) {
	content, err := decodeSaveImageContent(json.RawMessage(`{"$type":"is.currents.content.note"}`))
	if err != nil {
		t.Fatalf("decodeSaveImageContent failed: %v", err)
	}
	if content != nil {
		t.Fatal("expected nil content for non-image record")
	}
}