package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// labelerDIDHost extracts the host portion of a did:web DID.
// did:web:moderation.currents.is              → moderation.currents.is
// did:web:moderation.currents.is%3A8080       → moderation.currents.is:8080
// returns "" for non-did:web inputs.
func labelerDIDHost(did string) string {
	const prefix = "did:web:"
	if !strings.HasPrefix(did, prefix) {
		return ""
	}
	h := did[len(prefix):]
	// did:web percent-encodes ':' when a non-default port is part of the host.
	return strings.ReplaceAll(h, "%3A", ":")
}

// labelerDIDDoc returns the DID document for the labeler subdomain. It declares
// the secp256k1 verification method used to sign labels and the AtprotoLabeler
// service endpoint that hosts subscribeLabels / queryLabels / createReport.
func (s *Server) labelerDIDDoc() map[string]any {
	if s.Labeler == nil {
		return nil
	}
	did := s.Labeler.Signer.DID
	return map[string]any{
		"@context": []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
		},
		"id": did,
		"verificationMethod": []map[string]any{
			{
				"id":                 did + "#atproto_label",
				"type":               "Multikey",
				"controller":         did,
				"publicKeyMultibase": s.Labeler.Signer.PublicKeyMB,
			},
		},
		"service": []map[string]any{
			{
				"id":              "#atproto_labeler",
				"type":            "AtprotoLabeler",
				"serviceEndpoint": "https://" + s.LabelerHost,
			},
		},
	}
}

// LabelerWellKnownDID serves the labeler DID document. Only call this for
// requests whose Host matches s.LabelerHost; the routing decision is in
// WellKnownDID below.
func (s *Server) LabelerWellKnownDID(w http.ResponseWriter, r *http.Request) {
	doc := s.labelerDIDDoc()
	if doc == nil {
		http.Error(w, "labeler not configured", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/did+ld+json")
	_ = json.NewEncoder(w).Encode(doc)
}
