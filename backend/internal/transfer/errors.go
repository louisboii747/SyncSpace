package transfer

import "errors"

var (
	ErrNotFound            = errors.New("transfer not found")
	ErrInvalidRequest      = errors.New("invalid transfer request")
	ErrInvalidState        = errors.New("operation is not valid for the current transfer state")
	ErrUntrustedDevice     = errors.New("device is not trusted")
	ErrApprovalRequired    = errors.New("transfer requires approval")
	ErrUnauthorized        = errors.New("invalid transfer session token")
	ErrChecksumMismatch    = errors.New("checksum verification failed")
	ErrPathTraversal       = errors.New("unsafe transfer path")
	ErrConflictResolution  = errors.New("destination conflict requires a decision")
	ErrInsufficientStorage = errors.New("insufficient storage available at destination")
)
