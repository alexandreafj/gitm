package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// buildTestLegacyBundle constructs a syntactically valid legacy cosign bundle
// JSON using a self-signed certificate. The signature is real (ECDSA P-256
// over artifactBytes) but verifying against Sigstore's trust root would fail
// because the cert is not issued by Fulcio. This is sufficient for testing the
// conversion layer.
func buildTestLegacyBundle(t *testing.T, artifactBytes []byte) []byte {
	t.Helper()

	// Generate a throwaway ECDSA P-256 key and self-signed cert.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Sign the artifact.
	h := sha256.Sum256(artifactBytes)
	sigBytes, err := ecdsa.SignASN1(rand.Reader, privKey, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Construct a fake rekor body (just needs to be valid base64).
	fakeBody := base64.StdEncoding.EncodeToString([]byte(`{"apiVersion":"0.0.1","kind":"hashedrekord"}`))

	// Construct a fake log ID (32 bytes hex).
	logIDBytes := sha256.Sum256([]byte("fake-log-public-key"))
	logID := hex.EncodeToString(logIDBytes[:])

	// Construct a fake SET (just needs to be valid base64).
	fakeSET := base64.StdEncoding.EncodeToString([]byte("fake-signed-entry-timestamp"))

	legacy := legacyCosignBundle{
		Base64Signature: base64.StdEncoding.EncodeToString(sigBytes),
		Cert:            base64.StdEncoding.EncodeToString(certPEM),
		RekorBundle: &legacyRekorBundle{
			SignedEntryTimestamp: fakeSET,
			Payload: legacyRekorPayload{
				Body:           fakeBody,
				IntegratedTime: time.Now().Unix(),
				LogIndex:       1409144802,
				LogID:          logID,
			},
		},
	}

	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy bundle: %v", err)
	}
	return data
}

// mustMarshal is a test helper that marshals v to JSON or fails the test.
func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustMarshal: %v", err)
	}
	return data
}

func TestIsLegacyCosignBundle(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "legacy bundle with base64Signature",
			data: []byte(`{"base64Signature":"abc","cert":"def","rekorBundle":{}}`),
			want: true,
		},
		{
			name: "new protobuf bundle",
			data: []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle+json;version=0.1","verificationMaterial":{}}`),
			want: false,
		},
		{
			name: "empty JSON object",
			data: []byte(`{}`),
			want: false,
		},
		{
			name: "invalid JSON",
			data: []byte(`not json`),
			want: false,
		},
		{
			name: "empty bytes",
			data: []byte{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLegacyCosignBundle(tt.data)
			if got != tt.want {
				t.Errorf("isLegacyCosignBundle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertLegacyBundle_Success(t *testing.T) {
	artifact := []byte("fake checksums content\n")
	bundleJSON := buildTestLegacyBundle(t, artifact)

	b, err := convertLegacyBundle(bundleJSON, artifact)
	if err != nil {
		t.Fatalf("convertLegacyBundle: %v", err)
	}

	// Verify the resulting bundle has the expected structure.
	if b == nil {
		t.Fatal("expected non-nil bundle")
	}

	// Check media type.
	version, err := b.Version()
	if err != nil {
		t.Fatalf("bundle.Version: %v", err)
	}
	if version != "v0.1" {
		t.Errorf("expected bundle version v0.1, got %s", version)
	}

	// Check that verification content (cert) is present.
	vc, err := b.VerificationContent()
	if err != nil {
		t.Fatalf("VerificationContent: %v", err)
	}
	if vc == nil {
		t.Fatal("expected non-nil verification content")
	}

	// Check tlog entries.
	entries, err := b.TlogEntries()
	if err != nil {
		t.Fatalf("TlogEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 tlog entry, got %d", len(entries))
	}

	// Check that HasInclusionPromise is true (required for v0.1).
	if !b.HasInclusionPromise() {
		t.Error("expected HasInclusionPromise() = true for v0.1 bundle")
	}
}

func TestConvertLegacyBundle_ErrorCases(t *testing.T) {
	artifact := []byte("test artifact")

	tests := []struct {
		name        string
		data        []byte
		wantContain string
	}{
		{
			name:        "invalid JSON",
			data:        []byte(`{not json`),
			wantContain: "unmarshal legacy bundle",
		},
		{
			name:        "missing base64Signature",
			data:        []byte(`{"base64Signature":"","cert":"abc","rekorBundle":{"SignedEntryTimestamp":"abc","Payload":{"body":"abc","integratedTime":1,"logIndex":1,"logID":"aa"}}}`),
			wantContain: "missing base64Signature",
		},
		{
			name:        "missing cert",
			data:        []byte(`{"base64Signature":"abc","cert":"","rekorBundle":{"SignedEntryTimestamp":"abc","Payload":{"body":"abc","integratedTime":1,"logIndex":1,"logID":"aa"}}}`),
			wantContain: "missing cert",
		},
		{
			name:        "missing rekorBundle",
			data:        []byte(`{"base64Signature":"abc","cert":"abc"}`),
			wantContain: "missing rekorBundle",
		},
		{
			name:        "invalid base64Signature",
			data:        []byte(`{"base64Signature":"!!!invalid!!!","cert":"abc","rekorBundle":{"SignedEntryTimestamp":"abc","Payload":{"body":"abc","integratedTime":1,"logIndex":1,"logID":"aa"}}}`),
			wantContain: "decode base64Signature",
		},
		{
			name: "invalid cert base64",
			data: mustMarshal(t, legacyCosignBundle{
				Base64Signature: base64.StdEncoding.EncodeToString([]byte("sig")),
				Cert:            "!!!not-base64!!!",
				RekorBundle: &legacyRekorBundle{
					SignedEntryTimestamp: base64.StdEncoding.EncodeToString([]byte("set")),
					Payload: legacyRekorPayload{
						Body:           base64.StdEncoding.EncodeToString([]byte("body")),
						IntegratedTime: 1,
						LogIndex:       1,
						LogID:          "aa",
					},
				},
			}),
			wantContain: "decode cert base64",
		},
		{
			name: "cert is base64 but not PEM",
			data: mustMarshal(t, legacyCosignBundle{
				Base64Signature: base64.StdEncoding.EncodeToString([]byte("sig")),
				Cert:            base64.StdEncoding.EncodeToString([]byte("not a PEM block")),
				RekorBundle: &legacyRekorBundle{
					SignedEntryTimestamp: base64.StdEncoding.EncodeToString([]byte("set")),
					Payload: legacyRekorPayload{
						Body:           base64.StdEncoding.EncodeToString([]byte("body")),
						IntegratedTime: 1,
						LogIndex:       1,
						LogID:          "aa",
					},
				},
			}),
			wantContain: "decode cert PEM",
		},
		{
			name: "invalid logID hex",
			data: mustMarshal(t, legacyCosignBundle{
				Base64Signature: base64.StdEncoding.EncodeToString([]byte("sig")),
				Cert:            base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("fake-der")})),
				RekorBundle: &legacyRekorBundle{
					SignedEntryTimestamp: base64.StdEncoding.EncodeToString([]byte("set")),
					Payload: legacyRekorPayload{
						Body:           base64.StdEncoding.EncodeToString([]byte("body")),
						IntegratedTime: 1,
						LogIndex:       1,
						LogID:          "not-hex!!",
					},
				},
			}),
			wantContain: "decode logID hex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertLegacyBundle(tt.data, artifact)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("expected error containing %q, got: %v", tt.wantContain, err)
			}
		})
	}
}

// TestSigstoreVerifierLegacyBundleNoLongerFailsOnParse verifies that the
// "unknown field base64Signature" parse error no longer occurs. The verifier
// will still fail (at trust root fetch or identity verification) but the
// error message should NOT contain "unknown field".
func TestSigstoreVerifierLegacyBundleNoLongerFailsOnParse(t *testing.T) {
	artifact := []byte("checksums content for regression test\n")
	bundleJSON := buildTestLegacyBundle(t, artifact)

	sv, err := newSigstoreVerifier()
	if err != nil {
		t.Fatalf("newSigstoreVerifier: %v", err)
	}

	err = sv.Verify(artifact, bundleJSON)
	// We expect an error (trust root fetch will fail in CI/unit tests without
	// network, or identity won't match), but it should NOT be the old parse error.
	if err == nil {
		// If this passes with no error, that's fine too (means full verification
		// somehow succeeded, which is unlikely in unit tests).
		return
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "unknown field") {
		t.Fatalf("regression: legacy bundle still causes proto parse error: %v", err)
	}
	if strings.Contains(errMsg, `unknown field "base64Signature"`) {
		t.Fatalf("regression: got the exact old error: %v", err)
	}

	// The error should be from a later stage (trust root, identity, etc.)
	t.Logf("expected non-parse error from later verification stage: %v", err)
}
