package main

import "testing"

func TestRewriteSaveRecordForContentFromLegacyImage(t *testing.T) {
	record := map[string]any{
		"$type": saveNSID,
		"image": map[string]any{
			"$type":    "blob",
			"mimeType": "image/jpeg",
		},
	}

	changed, err := rewriteSaveRecordForContent(record)
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite to change the record")
	}
	if _, ok := record["image"]; ok {
		t.Fatal("expected legacy image field to be removed")
	}
	content, ok := record["content"].(map[string]any)
	if !ok {
		t.Fatal("expected content object")
	}
	if got := content["$type"]; got != saveContentImageNSID {
		t.Fatalf("unexpected content type: %#v", got)
	}
	if _, ok := content["image"]; !ok {
		t.Fatal("expected content.image to be present")
	}
}

func TestRewriteSaveRecordForContentAddsMissingType(t *testing.T) {
	record := map[string]any{
		"$type": saveNSID,
		"content": map[string]any{
			"image": map[string]any{"mimeType": "image/jpeg"},
		},
	}

	changed, err := rewriteSaveRecordForContent(record)
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite to change the record")
	}
	content := record["content"].(map[string]any)
	if got := content["$type"]; got != saveContentImageNSID {
		t.Fatalf("unexpected content type: %#v", got)
	}
}

func TestRewriteSaveRecordForContentNoopWhenAlreadyMigrated(t *testing.T) {
	record := map[string]any{
		"$type": saveNSID,
		"content": map[string]any{
			"$type": saveContentImageNSID,
			"image": map[string]any{"mimeType": "image/jpeg"},
		},
	}

	changed, err := rewriteSaveRecordForContent(record)
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if changed {
		t.Fatal("expected rewrite to be a no-op")
	}
}

func TestRewriteSaveRecordResaveOf(t *testing.T) {
	record := map[string]any{
		"$type": saveNSID,
		"resaveOf": map[string]any{
			"uri": "at://did:plc:alice/is.currents.feed.save/abc",
			"cid": "bafkold",
		},
	}

	changed, err := rewriteSaveRecordResaveOf(record, map[string]any{
		"uri": "at://did:plc:alice/is.currents.feed.save/abc",
		"cid": "bafknew",
	})
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite to change the record")
	}
	resaveOf := record["resaveOf"].(map[string]any)
	if got := resaveOf["cid"]; got != "bafknew" {
		t.Fatalf("unexpected resaveOf cid: %#v", got)
	}
}
