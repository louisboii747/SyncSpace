package pairing

import "time"

// EventType identifies a pairing or trust transition.
type EventType string

const (
	// EventPairingRequested is emitted when a new request awaits approval.
	EventPairingRequested EventType = "PairingRequested"
	// EventPairingAccepted is emitted after trust is durably stored.
	EventPairingAccepted EventType = "PairingAccepted"
	// EventPairingRejected is emitted after a pending request is rejected.
	EventPairingRejected EventType = "PairingRejected"
	// EventTrustedDeviceRemoved is emitted after local trust is removed.
	EventTrustedDeviceRemoved EventType = "TrustedDeviceRemoved"
)

// Event is the stable WebSocket envelope for pairing state changes.
type Event struct {
	Type          EventType      `json:"type"`
	Request       *Request       `json:"request,omitempty"`
	TrustedDevice *TrustedDevice `json:"trustedDevice,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
}

// EventPublisher receives pairing events after their state transition commits.
type EventPublisher interface {
	Publish(Event)
}
