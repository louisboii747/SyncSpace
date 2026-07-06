// Package models contains transport-neutral domain models shared by SyncSpace
// services and delivery adapters.
package models

import "time"

// TransferCapabilities are reusable discovery metadata for negotiation.
type TransferCapabilities struct {
	AvailableStorage   int64 `json:"availableStorage"`
	ProtocolVersion    int   `json:"protocolVersion"`
	MaxChunkSize       int64 `json:"maximumChunkSize"`
	CompressionSupport bool  `json:"compressionSupport"`
	TransferSupport    bool  `json:"transferCapability"`
}

// ConnectionState describes whether a device is currently reachable through
// LAN discovery. Additional states can be added later without changing the
// device model consumed by feature modules.
type ConnectionState string

const (
	// ConnectionOnline means the device has been observed recently.
	ConnectionOnline ConnectionState = "online"
	// ConnectionOffline means the device has exceeded the discovery timeout.
	ConnectionOffline ConnectionState = "offline"
)

// Device is the canonical representation of a SyncSpace device.
type Device struct {
	ID                       string          `json:"deviceId"`
	Name                     string          `json:"deviceName"`
	Type                     string          `json:"deviceType"`
	Platform                 string          `json:"platform"`
	LocalIP                  string          `json:"localIp"`
	Port                     int             `json:"port"`
	AppVersion               string          `json:"appVersion"`
	LastSeen                 time.Time       `json:"lastSeen"`
	Online                   bool            `json:"online"`
	ConnectionState          ConnectionState `json:"connectionState"`
	AvailableStorage         int64           `json:"availableStorage"`
	TransferCapability       bool            `json:"transferCapability"`
	SupportedProtocolVersion int             `json:"supportedProtocolVersion"`
	MaximumChunkSize         int64           `json:"maximumChunkSize"`
	CompressionSupport       bool            `json:"compressionSupport"`
}
