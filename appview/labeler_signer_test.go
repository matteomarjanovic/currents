package main

import (
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/labeling"
)

func TestLabelerSigner_SignProducesVerifiableSignature(t *testing.T) {
	// Generate a fresh secp256k1 key and re-load through the multibase path,
	// proving the env-driven loading path works end-to-end.
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	mb := priv.Multibase()

	signer, err := NewLabelerSigner("did:web:moderation.test", mb)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	if signer == nil {
		t.Fatal("signer unexpectedly nil")
	}

	label := &labeling.Label{
		Version:   labeling.ATPROTO_LABEL_VERSION,
		SourceDID: signer.DID,
		URI:       "at://did:plc:alice/is.currents.feed.save/abc123",
		Val:       "currents-nsfw-suspected",
		CreatedAt: "2026-05-17T12:00:00Z",
	}
	if err := signer.Sign(label); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(label.Sig) == 0 {
		t.Fatal("signature empty after Sign")
	}

	pub, err := signer.PrivateKey.PublicKey()
	if err != nil {
		t.Fatalf("derive pub: %v", err)
	}
	if err := label.VerifySignature(pub); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
}

func TestNewLabelerSigner_DisabledWhenKeyEmpty(t *testing.T) {
	signer, err := NewLabelerSigner("did:web:moderation.test", "")
	if err != nil {
		t.Fatalf("expected nil error for empty key, got: %v", err)
	}
	if signer != nil {
		t.Fatal("expected nil signer for empty key")
	}
}

func TestNewLabelerSigner_RejectsKeyWithoutDID(t *testing.T) {
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if _, err := NewLabelerSigner("", priv.Multibase()); err == nil {
		t.Fatal("expected error when DID missing but key present")
	}
}
