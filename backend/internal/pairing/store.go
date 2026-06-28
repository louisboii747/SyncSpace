package pairing

import (
	"context"
	"errors"
)

var (
	// ErrTrustedDeviceNotFound means no durable trust record exists for an ID.
	ErrTrustedDeviceNotFound = errors.New("trusted device not found")
)

// TrustedDeviceStore persists explicit local trust decisions.
type TrustedDeviceStore interface {
	List(context.Context) ([]TrustedDevice, error)
	Get(context.Context, string) (TrustedDevice, error)
	Upsert(context.Context, TrustedDevice) error
	Delete(context.Context, string) (TrustedDevice, error)
}
