package services

import (
	"crypto/ed25519"
	"crypto/x509"
	"path/filepath"
	"testing"
	"time"
)

func TestIdentityTLSCertificateUsesPinnedIdentityKey(t *testing.T) {
	identity, err := NewFileIdentityStore(filepath.Join(t.TempDir(), "identity.json")).LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := identity.TLSCertificate(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := parsed.PublicKey.(ed25519.PublicKey)
	if !ok || IdentityFingerprint(publicKey) != identity.Fingerprint {
		t.Fatal("TLS certificate did not use the persistent identity key")
	}
}
