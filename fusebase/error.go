package fusebase

import (
	"bazil.org/fuse"
	"syscall"
)

const (
	EINVAL = fuse.Errno(syscall.EINVAL)
)
