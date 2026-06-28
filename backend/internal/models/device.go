// Package models contains transport-neutral domain models shared by SyncSpace
// services and delivery adapters.
package models

import "time"

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
	ID              string          `json:"deviceId"`
	Name            string          `json:"deviceName"`
	Type            string          `json:"deviceType"`
	Platform        string          `json:"platform"`
	LocalIP         string          `json:"localIp"`
	Port            int             `json:"port"`
	AppVersion      string          `json:"appVersion"`
	LastSeen        time.Time       `json:"lastSeen"`
	Online          bool            `json:"online"`
	ConnectionState ConnectionState `json:"connectionState"`
}
