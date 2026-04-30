package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
)

// legacyCosignBundle represents the JSON schema produced by
// `cosign sign-blob --bundle` (cosign v2.x legacy format).
//
// Example:
//
//	{
//	  "base64Signature": "MEUCIQ...",
//	  "cert": "LS0tLS1C...",
//	  "rekorBundle": {
//	    "SignedEntryTimestamp": "MEUCIQ...",
//	    "Payload": {
//	      "body": "eyJhcGl...",
//	      "integratedTime": 1745984827,
//	      "logIndex": 1409144802,
//	      "logID": "c0d23d6ad406973f9559f3ba2d1ca01f84147d8ffc5b8445c224f98b9591801d"
//	    }
//	  }
//	}
type legacyCosignBundle struct {
	Base64Signature string             `json:"base64Signature"`
	Cert            string             `json:"cert"`
	RekorBundle     *legacyRekorBundle `json:"rekorBundle"`
}

type legacyRekorBundle struct {
	SignedEntryTimestamp string             `json:"SignedEntryTimestamp"`
	Payload              legacyRekorPayload `json:"Payload"`
}

type legacyRekorPayload struct {
	Body           string `json:"body"`
	IntegratedTime int64  `json:"integratedTime"`
	LogIndex       int64  `json:"logIndex"`
	LogID          string `json:"logID"`
}

// isLegacyCosignBundle returns true if the raw JSON contains the
// "base64Signature" key, indicating it's a legacy cosign bundle
// rather than a protobuf Sigstore bundle.
func isLegacyCosignBundle(data []byte) bool {
	var probe struct {
		Base64Signature *json.RawMessage `json:"base64Signature"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Base64Signature != nil
}

// convertLegacyBundle transforms a legacy cosign bundle JSON into a
// *bundle.Bundle that sigstore-go can verify.
//
// The resulting bundle uses:
//   - mediaType: v0.1 (inclusion promise required, no inclusion proof)
//   - verificationMaterial: X509CertificateChain with the signing cert
//   - tlogEntries[0]: transparency log entry with inclusion promise
//   - content: MessageSignature with SHA-256 digest of the artifact
func convertLegacyBundle(data []byte, artifactBytes []byte) (*bundle.Bundle, error) {
	var legacy legacyCosignBundle
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("unmarshal legacy bundle: %w", err)
	}

	if legacy.Base64Signature == "" {
		return nil, fmt.Errorf("legacy bundle missing base64Signature")
	}
	if legacy.Cert == "" {
		return nil, fmt.Errorf("legacy bundle missing cert")
	}
	if legacy.RekorBundle == nil {
		return nil, fmt.Errorf("legacy bundle missing rekorBundle")
	}

	// Decode signature.
	sigBytes, err := base64.StdEncoding.DecodeString(legacy.Base64Signature)
	if err != nil {
		return nil, fmt.Errorf("decode base64Signature: %w", err)
	}

	// Decode certificate: the "cert" field is base64-encoded PEM.
	certPEM, err := base64.StdEncoding.DecodeString(legacy.Cert)
	if err != nil {
		return nil, fmt.Errorf("decode cert base64: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("decode cert PEM: no PEM block found")
	}
	certDER := block.Bytes

	// Decode the Signed Entry Timestamp (inclusion promise).
	set, err := base64.StdEncoding.DecodeString(legacy.RekorBundle.SignedEntryTimestamp)
	if err != nil {
		return nil, fmt.Errorf("decode SignedEntryTimestamp: %w", err)
	}

	// Decode the canonicalized body (base64-encoded in the payload).
	canonBody, err := base64.StdEncoding.DecodeString(legacy.RekorBundle.Payload.Body)
	if err != nil {
		return nil, fmt.Errorf("decode rekor body: %w", err)
	}

	// Decode the log ID (hex-encoded SHA-256 of the log's public key).
	logIDBytes, err := hex.DecodeString(legacy.RekorBundle.Payload.LogID)
	if err != nil {
		return nil, fmt.Errorf("decode logID hex: %w", err)
	}

	// Compute SHA-256 digest of the artifact for MessageSignature.
	artifactDigest := sha256.Sum256(artifactBytes)

	// Construct the protobuf bundle.
	pb := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle+json;version=0.1",
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_X509CertificateChain{
				X509CertificateChain: &protocommon.X509CertificateChain{
					Certificates: []*protocommon.X509Certificate{
						{RawBytes: certDER},
					},
				},
			},
			TlogEntries: []*protorekor.TransparencyLogEntry{
				{
					LogIndex: legacy.RekorBundle.Payload.LogIndex,
					LogId: &protocommon.LogId{
						KeyId: logIDBytes,
					},
					KindVersion: &protorekor.KindVersion{
						Kind:    "hashedrekord",
						Version: "0.0.1",
					},
					IntegratedTime: legacy.RekorBundle.Payload.IntegratedTime,
					InclusionPromise: &protorekor.InclusionPromise{
						SignedEntryTimestamp: set,
					},
					CanonicalizedBody: canonBody,
				},
			},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    artifactDigest[:],
				},
				Signature: sigBytes,
			},
		},
	}

	b, err := bundle.NewBundle(pb)
	if err != nil {
		return nil, fmt.Errorf("construct bundle from legacy: %w", err)
	}
	return b, nil
}
