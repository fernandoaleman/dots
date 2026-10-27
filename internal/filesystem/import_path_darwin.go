package filesystem

import (
	"bytes"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Query only a pinned directory; this is additional comparison evidence, not
// an enumeration of every alias. F_GETPATH would hide firmlink ancestry.
func importDirectoryPath(fd int) (string, error) {
	// Darwin MAXPATHLEN is 1024 bytes, including the terminator.
	var buf [1024]byte
	_, err := importPathFcntl(uintptr(fd), unix.F_GETPATH_NOFIRMLINK, uintptr(unsafe.Pointer(&buf[0])))
	runtime.KeepAlive(&buf)
	if err != nil {
		return "", err
	}
	return importDarwinPath(buf[:])
}

func importDarwinPath(buf []byte) (string, error) {
	end := bytes.IndexByte(buf, 0)
	if end <= 0 || buf[0] != '/' {
		return "", fmt.Errorf("invalid native directory path")
	}
	return string(buf[:end]), nil
}

// Keep the buffer heap-backed and live across the integer-based x/sys wrapper.
// This is the same pointer-lifetime convention used by Go syscall wrappers.
//
//go:uintptrescapes
func importPathFcntl(fd uintptr, command int, buffer uintptr) (int, error) {
	return unix.FcntlInt(fd, command, int(buffer))
}
