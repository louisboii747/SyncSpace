// Package pairing implements explicit local trust decisions for discovered
// SyncSpace devices. Discovery remains the source of presence; pairing owns
// only pending requests and durable trusted-device state.
package pairing

import "time"

// TrustState is the persisted trust decision for a device.
type TrustState string

const (
	// TrustStateTrusted means the local user explicitly accepted this device.
	TrustStateTrusted TrustState = "trusted"
)

// RequestState describes the lifecycle of a pairing request.
type RequestState string

const (
	// RequestStatePending means the request still requires a local decision.
	RequestStatePending RequestState = "pending"
	// RequestStateRejected means the local user rejected the request.
	RequestStateRejected RequestState = "rejected"
)

// TrustedDevice is the durable local trust record used by future SyncSpace
// modules before exchanging sensitive data.
type TrustedDevice struct {
	DeviceID   string     `json:"deviceId"`
	DeviceName string     `json:"deviceName"`
	Platform   string     `json:"platform"`
	PairingKey string     `json:"pairingKey"`
	PairedAt   time.Time  `json:"pairedAt"`
	LastSeen   time.Time  `json:"lastSeen"`
	TrustState TrustState `json:"trustState"`
}

// Request is a short-lived, in-memory request awaiting explicit approval.
type Request struct {
	RequestID   string       `json:"requestId"`
	DeviceID    string       `json:"deviceId"`
	DeviceName  string       `json:"deviceName"`
	Platform    string       `json:"platform"`
	PairingKey  string       `json:"pairingKey"`
	RequestedAt time.Time    `json:"requestedAt"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	State       RequestState `json:"state"`
}
