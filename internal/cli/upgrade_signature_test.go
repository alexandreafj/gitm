package cli

import (
	"strings"
	"testing"
)

func TestSigstoreVerifierMalformedBundle(t *testing.T) {
	sv, err := newSigstoreVerifier()
	if err != nil {
		t.Fatalf("newSigstoreVerifier: %v", err)
	}

	tests := []struct {
		name        string
		bundle      []byte
		wantContain string
	}{
		{
			name:        "empty bytes",
			bundle:      []byte{},
			wantContain: "parse bundle",
		},
		{
			name:        "invalid JSON",
			bundle:      []byte("{not json at all"),
			wantContain: "parse bundle",
		},
		{
			name:        "truncated JSON",
			bundle:      []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"`),
			wantContain: "parse bundle",
		},
		{
			name:        "valid JSON but not a bundle",
			bundle:      []byte(`{"foo":"bar"}`),
			wantContain: "parse bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv.Verify([]byte("checksums content"), tt.bundle)
			if err == nil {
				t.Fatal("expected error for malformed bundle, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("expected error containing %q, got: %v", tt.wantContain, err)
			}
		})
	}
}

func TestNewSigstoreVerifier(t *testing.T) {
	sv, err := newSigstoreVerifier()
	if err != nil {
		t.Fatalf("newSigstoreVerifier should not fail: %v", err)
	}
	if sv == nil {
		t.Fatal("expected non-nil verifier")
	}
}

func TestSigstoreVerifierSatisfiesInterface(t *testing.T) {
	// Compile-time check is in upgrade_signature.go via var _ signatureVerifier = (*sigstoreVerifier)(nil).
	// This test exercises that newSigstoreVerifier returns a usable interface value.
	sv, err := newSigstoreVerifier()
	if err != nil {
		t.Fatalf("newSigstoreVerifier: %v", err)
	}
	var iface signatureVerifier = sv
	_ = iface // ensure assignment compiles
}
