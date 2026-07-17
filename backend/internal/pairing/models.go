// Package pairing implements authenticated, explicitly confirmed device trust.
// Discovery remains untrusted presence; only a completed verification-code
// ceremony creates durable credentials.
package pairing

import "time"

const ProtocolVersion = 1

type TrustState string

const (
	TrustStateTrusted TrustState = "trusted"
	TrustStateBlocked TrustState = "blocked"
)

type RequestState string

const (
	RequestStatePending    RequestState = "pending"
	RequestStateConfirming RequestState = "confirming"
	RequestStatePaired     RequestState = "paired"
	RequestStateRejected   RequestState = "rejected"
	RequestStateExpired    RequestState = "expired"
)

type RequestDirection string

const (
	DirectionIncoming RequestDirection = "incoming"
	DirectionOutgoing RequestDirection = "outgoing"
)

// TrustedDevice is a pinned long-term peer identity. PairingKey is deliberately
// excluded from JSON so local UI and diagnostics responses cannot leak the
// shared peer-authentication credential.
type TrustedDevice struct {
	DeviceID           string     `json:"deviceId"`
	DeviceName         string     `json:"deviceName"`
	LocalName          string     `json:"localName,omitempty"`
	Platform           string     `json:"platform"`
	PublicKey          string     `json:"publicKey"`
	Fingerprint        string     `json:"fingerprint"`
	PairingKey         string     `json:"-"`
	PairedAt           time.Time  `json:"pairedAt"`
	LastSeen           time.Time  `json:"lastSeen"`
	LastAuthenticated  time.Time  `json:"lastAuthenticatedAt,omitempty"`
	TrustState         TrustState `json:"trustState"`
	Blocked            bool       `json:"blocked"`
	IdentityKeyChanged bool       `json:"identityKeyChanged"`
	Notes              string     `json:"notes,omitempty"`
}

// Request is the safe UI projection of an authenticated key exchange. Secret
// material and ephemeral private keys stay in the in-memory session only.
type Request struct {
	RequestID        string           `json:"requestId"`
	DeviceID         string           `json:"deviceId"`
	DeviceName       string           `json:"deviceName"`
	Platform         string           `json:"platform"`
	Direction        RequestDirection `json:"direction"`
	Fingerprint      string           `json:"fingerprint"`
	VerificationCode string           `json:"verificationCode"`
	RequestedAt      time.Time        `json:"requestedAt"`
	ExpiresAt        time.Time        `json:"expiresAt"`
	State            RequestState     `json:"state"`
	LocalConfirmed   bool             `json:"localConfirmed"`
	RemoteConfirmed  bool             `json:"remoteConfirmed"`
}

type Decision struct {
	Request       Request        `json:"request"`
	TrustedDevice *TrustedDevice `json:"trustedDevice,omitempty"`
}

// BeginRequest and BeginResponse form a signed X25519 exchange. Their
// canonical signature payloads are defined in crypto.go.
type BeginRequest struct {
	ProtocolVersion int    `json:"protocolVersion"`
	RequestID       string `json:"requestId"`
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	Platform        string `json:"platform"`
	PublicKey       string `json:"publicKey"`
	EphemeralKey    string `json:"ephemeralKey"`
	Timestamp       string `json:"timestamp"`
	Nonce           string `json:"nonce"`
	Signature       string `json:"signature"`
}

type BeginResponse struct {
	ProtocolVersion int    `json:"protocolVersion"`
	RequestID       string `json:"requestId"`
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	Platform        string `json:"platform"`
	PublicKey       string `json:"publicKey"`
	EphemeralKey    string `json:"ephemeralKey"`
	Timestamp       string `json:"timestamp"`
	Nonce           string `json:"nonce"`
	Signature       string `json:"signature"`
}

type Proof struct {
	ProtocolVersion int    `json:"protocolVersion"`
	RequestID       string `json:"requestId"`
	DeviceID        string `json:"deviceId"`
	Action          string `json:"action"`
	Timestamp       string `json:"timestamp"`
	Nonce           string `json:"nonce"`
	MAC             string `json:"mac"`
}

type PeerDecision struct {
	RequestID       string       `json:"requestId"`
	State           RequestState `json:"state"`
	LocalConfirmed  bool         `json:"localConfirmed"`
	RemoteConfirmed bool         `json:"remoteConfirmed"`
	Timestamp       string       `json:"timestamp"`
	MAC             string       `json:"mac"`
}
