package transfer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

func offerSignature(key []byte, method, path, deviceID, timestamp, nonce string, body []byte) string {
	hash := sha256.Sum256(body)
	canonical := strings.Join([]string{method, path, deviceID, timestamp, nonce, hex.EncodeToString(hash[:])}, "\n")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validOfferSignature(key []byte, auth PeerAuthentication, method, path string, body []byte) bool {
	expected, err := base64.RawURLEncoding.DecodeString(offerSignature(key, method, path, auth.DeviceID, auth.Timestamp, auth.Nonce, body))
	if err != nil {
		return false
	}
	actual, err := base64.RawURLEncoding.DecodeString(auth.Signature)
	return err == nil && hmac.Equal(expected, actual)
}
