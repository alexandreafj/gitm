package cli

import (
	"bytes"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// signatureVerifier verifies that a Sigstore bundle is a valid signature over
// the given checksums bytes, signed by this repo's release workflow.
type signatureVerifier interface {
	Verify(checksumsBytes, bundleBytes []byte) error
}

const (
	signerIssuer       = "https://token.actions.githubusercontent.com"
	signerSubjectRegex = `^https://github\.com/alexandreafj/gitm/\.github/workflows/release\.yml@refs/tags/v.*$`
)

// sigstoreVerifier is the production signatureVerifier backed by sigstore-go.
//
// Unit tests for this implementation are intentionally absent from this PR:
// exercising the full verification pipeline requires a real cosign-signed
// Sigstore bundle, which cannot be generated locally before the first signed
// release ships. The signatureVerifier interface itself is fully tested in
// the upgrade integration tests (T4) via a fake implementation. End-to-end
// correctness of sigstoreVerifier is validated by the smoke step in T7.
type sigstoreVerifier struct{}

func newSigstoreVerifier() (*sigstoreVerifier, error) {
	return &sigstoreVerifier{}, nil
}

// Verify checks that bundleBytes is a valid Sigstore bundle over checksumsBytes,
// produced by this repo's release workflow on a refs/tags/v* ref.
//
// Note: root.FetchTrustedRoot fetches the Sigstore public-good trust root over
// the network (via TUF). This is acceptable here because upgrade has network
// access by definition.
func (s *sigstoreVerifier) Verify(checksumsBytes, bundleBytes []byte) error {
	// Parse the Sigstore bundle from its JSON representation.
	// Releases v1.0.8 and v1.0.9 shipped with cosign's legacy bundle format
	// (top-level "base64Signature" key). Detect and convert before parsing.
	var b *bundle.Bundle
	if isLegacyCosignBundle(bundleBytes) {
		converted, err := convertLegacyBundle(bundleBytes, checksumsBytes)
		if err != nil {
			return fmt.Errorf("parse bundle: %w", err)
		}
		b = converted
	} else {
		b = new(bundle.Bundle)
		if err := b.UnmarshalJSON(bundleBytes); err != nil {
			return fmt.Errorf("parse bundle: %w", err)
		}
	}

	// Fetch the Sigstore public-good trust root. Requires network access.
	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return fmt.Errorf("load sigstore trust root: %w", err)
	}

	// NewVerifier is the non-deprecated successor to NewSignedEntityVerifier.
	verifier, err := verify.NewVerifier(
		trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("build verifier: %w", err)
	}

	// Pin the identity to this repo's release.yml on refs/tags/v*.
	// Args: issuer, issuerRegex, sanValue, sanRegex.
	identity, err := verify.NewShortCertificateIdentity(signerIssuer, "", "", signerSubjectRegex)
	if err != nil {
		return fmt.Errorf("build identity policy: %w", err)
	}

	policy := verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(checksumsBytes)),
		verify.WithCertificateIdentity(identity),
	)

	if _, err := verifier.Verify(b, policy); err != nil {
		return fmt.Errorf("signature verify: %w", err)
	}
	return nil
}

// Compile-time check that sigstoreVerifier satisfies signatureVerifier.
var _ signatureVerifier = (*sigstoreVerifier)(nil)
