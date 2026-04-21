package main

import (
	"encoding/json"
	"testing"
)

func TestRewriteSaveRecordValueMigratesLegacyAttribution(t *testing.T) {
	raw := json.RawMessage(`{"$type":"is.currents.feed.save","collection":{"uri":"at://did:plc:test/is.currents.feed.collection/abc","cid":"bafycid"},"content":{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"}},"attribution":{"credit":"legacy"},"createdAt":"2026-04-21T00:00:00Z"}`)

	record, changed, err := rewriteSaveRecordValue(raw)
	if err != nil {
		t.Fatalf("rewriteSaveRecordValue failed: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite")
	}
	if _, ok := record["attribution"]; ok {
		t.Fatal("expected top-level attribution to be removed")
	}
	content, ok := record["content"].(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent content, got %T", record["content"])
	}
	if content.Attribution == nil || content.Attribution.Credit != "legacy" {
		t.Fatalf("expected nested legacy attribution, got %#v", content.Attribution)
	}
	if content.Image.Type != "blob" {
		t.Fatalf("expected blob type to be preserved, got %q", content.Image.Type)
	}
}

func TestRewriteSaveRecordValueSkipsAlreadyMigratedRecord(t *testing.T) {
	raw := json.RawMessage(`{"$type":"is.currents.feed.save","collection":{"uri":"at://did:plc:test/is.currents.feed.collection/abc","cid":"bafycid"},"content":{"$type":"is.currents.content.image","image":{"$type":"blob","ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}},"createdAt":"2026-04-21T00:00:00Z"}`)

	_, changed, err := rewriteSaveRecordValue(raw)
	if err != nil {
		t.Fatalf("rewriteSaveRecordValue failed: %v", err)
	}
	if changed {
		t.Fatal("expected already migrated record to be skipped")
	}
}

func TestRewriteSaveRecordValueRepairsMissingBlobType(t *testing.T) {
	raw := json.RawMessage(`{"$type":"is.currents.feed.save","collection":{"uri":"at://did:plc:test/is.currents.feed.collection/abc","cid":"bafycid"},"content":{"$type":"is.currents.content.image","image":{"ref":{"$link":"bafkcid"},"mimeType":"image/jpeg"},"attribution":{"credit":"nested"}},"createdAt":"2026-04-21T00:00:00Z"}`)

	record, changed, err := rewriteSaveRecordValue(raw)
	if err != nil {
		t.Fatalf("rewriteSaveRecordValue failed: %v", err)
	}
	if !changed {
		t.Fatal("expected missing blob type to be repaired")
	}
	content, ok := record["content"].(*saveImageContent)
	if !ok {
		t.Fatalf("expected *saveImageContent content, got %T", record["content"])
	}
	if content.Image.Type != "blob" {
		t.Fatalf("expected repaired blob type, got %q", content.Image.Type)
	}
}
