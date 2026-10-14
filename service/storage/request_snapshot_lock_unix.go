//go:build !windows

package storage

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockSnapshotSpool(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func unlockSnapshotSpool(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
