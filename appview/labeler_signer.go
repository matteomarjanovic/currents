package main

import (
	"fmt"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/labeling"
)

// LabelerSigner holds the labeler's identity: its DID, the secp256k1 signing
// private key, and the multibase public key advertised in the DID document under
// the #atproto_label verification method.
type LabelerSigner struct {
	DID         string
	PrivateKey  atcrypto.PrivateKeyExportable
	PublicKeyMB string
}

// NewLabelerSigner loads the signing key from a multibase-encoded string. Returns
// (nil, nil) when privKeyMultibase is empty so the appview can run without a
// configured labeler; call sites must nil-check the resulting signer.
func NewLabelerSigner(did, privKeyMultibase string) (*LabelerSigner, error) {
	if privKeyMultibase == "" {
		return nil, nil
	}
	if did == "" {
		return nil, fmt.Errorf("labeler DID required when signing key is set")
	}
	priv, err := atcrypto.ParsePrivateMultibase(privKeyMultibase)
	if err != nil {
		return nil, fmt.Errorf("parse labeler signing key: %w", err)
	}
	pub, err := priv.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("derive labeler public key: %w", err)
	}
	return &LabelerSigner{
		DID:         did,
		PrivateKey:  priv,
		PublicKeyMB: pub.Multibase(),
	}, nil
}

// Sign populates l.Sig with a secp256k1 signature over the canonical CBOR
// encoding of the label (excluding the Sig field itself).
func (s *LabelerSigner) Sign(l *labeling.Label) error {
	return l.Sign(s.PrivateKey)
}
