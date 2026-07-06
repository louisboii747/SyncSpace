//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package transfer

import "golang.org/x/sys/unix"

func availableStorage(path string) int64 {
	var stat unix.Statfs_t
	if err := unix.Statfs(storageProbePath(path), &stat); err != nil {
		return -1
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}
