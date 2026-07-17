package pairing

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/hkdf"
)

func canonicalBeginRequest(value BeginRequest) []byte {
	value.Signature = ""
	encoded, _ := json.Marshal(value)
	return encoded
}

func canonicalBeginResponse(value BeginResponse) []byte {
	value.Signature = ""
	encoded, _ := json.Marshal(value)
	return encoded
}

func signRequest(value *BeginRequest, privateKey ed25519.PrivateKey) {
	value.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, canonicalBeginRequest(*value)))
}

func signResponse(value *BeginResponse, privateKey ed25519.PrivateKey) {
	value.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, canonicalBeginResponse(*value)))
}

func verifyRequest(value BeginRequest) (ed25519.PublicKey, error) {
	return verifySignature(value.PublicKey, value.Signature, canonicalBeginRequest(value))
}

func verifyResponse(value BeginResponse) (ed25519.PublicKey, error) {
	return verifySignature(value.PublicKey, value.Signature, canonicalBeginResponse(value))
}

func verifySignature(publicEncoded, signatureEncoded string, message []byte) (ed25519.PublicKey, error) {
	publicKey, err := base64.RawURLEncoding.DecodeString(publicEncoded)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid Ed25519 public key")
	}
	signature, err := base64.RawURLEncoding.DecodeString(signatureEncoded)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, errors.New("invalid Ed25519 signature")
	}
	if !ed25519.Verify(publicKey, message, signature) {
		return nil, errors.New("identity signature verification failed")
	}
	return ed25519.PublicKey(publicKey), nil
}

func newEphemeral() (*ecdh.PrivateKey, string, error) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	return privateKey, base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes()), nil
}

func derivePairing(privateKey *ecdh.PrivateKey, remoteEncoded string, request BeginRequest, response BeginResponse) ([]byte, string, error) {
	remoteBytes, err := base64.RawURLEncoding.DecodeString(remoteEncoded)
	if err != nil {
		return nil, "", errors.New("invalid X25519 public key")
	}
	remoteKey, err := ecdh.X25519().NewPublicKey(remoteBytes)
	if err != nil {
		return nil, "", fmt.Errorf("decode X25519 public key: %w", err)
	}
	shared, err := privateKey.ECDH(remoteKey)
	if err != nil {
		return nil, "", fmt.Errorf("derive pairing secret: %w", err)
	}
	transcript := sha256.Sum256(append(canonicalBeginRequest(request), canonicalBeginResponse(response)...))
	material := make([]byte, 40)
	reader := hkdf.New(sha256.New, shared, transcript[:], []byte("SyncSpace authenticated pairing v1"))
	if _, err := io.ReadFull(reader, material); err != nil {
		return nil, "", err
	}
	codeValue := binary.BigEndian.Uint64(material[32:]) % 1_000_000
	code := fmt.Sprintf("%03d %03d", codeValue/1000, codeValue%1000)
	return append([]byte(nil), material[:32]...), code, nil
}

func proofMAC(secret []byte, proof Proof) string {
	proof.MAC = ""
	encoded, _ := json.Marshal(proof)
	mac := hmac.New(sha256.New, secret)
	mac.Write(encoded)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyProof(secret []byte, proof Proof) bool {
	expected, err := base64.RawURLEncoding.DecodeString(proofMAC(secret, proof))
	if err != nil {
		return false
	}
	actual, err := base64.RawURLEncoding.DecodeString(proof.MAC)
	return err == nil && hmac.Equal(expected, actual)
}

func decisionMAC(secret []byte, decision PeerDecision) string {
	decision.MAC = ""
	encoded, _ := json.Marshal(decision)
	mac := hmac.New(sha256.New, secret)
	mac.Write(encoded)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyDecision(secret []byte, decision PeerDecision) bool {
	expected, err := base64.RawURLEncoding.DecodeString(decisionMAC(secret, decision))
	if err != nil {
		return false
	}
	actual, err := base64.RawURLEncoding.DecodeString(decision.MAC)
	return err == nil && hmac.Equal(expected, actual)
}

func decodeSharedKey(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("invalid shared pairing key")
	}
	return decoded, nil
}

func shortFingerprint(full string) string {
	parts := strings.Split(full, ":")
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, ":")
}

func validProtocolTimestamp(value string, nowUnix int64, maxSkewSeconds int64) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return false
	}
	difference := parsed - nowUnix
	if difference < 0 {
		difference = -difference
	}
	return difference <= maxSkewSeconds
}
