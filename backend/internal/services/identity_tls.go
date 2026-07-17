package services

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

// TLSCertificate creates an ephemeral self-signed certificate backed by the
// installation's persistent Ed25519 identity. Peer clients pin that public key
// through discovery during pairing and through the durable trust record after
// pairing; public-CA trust and DNS names are intentionally not involved.
func (i Identity) TLSCertificate(now time.Time) (tls.Certificate, error) {
	if err := i.Validate(); err != nil {
		return tls.Certificate{}, err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS certificate serial: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "SyncSpace " + i.ID},
		NotBefore:    now.UTC().Add(-5 * time.Minute), NotAfter: now.UTC().Add(397 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, i.PrivateKey.Public(), i.PrivateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create identity TLS certificate: %w", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: i.PrivateKey, Leaf: template}, nil
}
