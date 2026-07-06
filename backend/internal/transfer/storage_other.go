//go:build !windows && !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package transfer

// Unsupported future targets still advertise transfer support while reporting
// unknown free space. A native storage adapter can replace this conservative
// value without changing the discovery contract.
func availableStorage(string) int64 { return -1 }
