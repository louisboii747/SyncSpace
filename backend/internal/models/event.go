package models

import "time"

// DiscoveryEventType identifies a registry lifecycle transition.
type DiscoveryEventType string

const (
	// EventDeviceDiscovered is emitted for a newly observed device.
	EventDeviceDiscovered DiscoveryEventType = "DeviceDiscovered"
	// EventDeviceUpdated is emitted when device metadata or state changes.
	EventDeviceUpdated DiscoveryEventType = "DeviceUpdated"
	// EventDeviceOffline is emitted when a device exceeds the online timeout.
	EventDeviceOffline DiscoveryEventType = "DeviceOffline"
	// EventDeviceRemoved is emitted when an offline device is evicted.
	EventDeviceRemoved DiscoveryEventType = "DeviceRemoved"
)

// DiscoveryEvent is the stable event envelope delivered to connected clients.
type DiscoveryEvent struct {
	Type      DiscoveryEventType `json:"type"`
	Device    Device             `json:"device"`
	Timestamp time.Time          `json:"timestamp"`
}
