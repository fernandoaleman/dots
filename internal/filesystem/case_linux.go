package filesystem

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const importReadOnlyFlag = unix.ST_RDONLY

func directoryCaseSensitive(name string) (bool, error) {
	var volume syscall.Statfs_t
	if err := syscall.Statfs(name, &volume); err != nil {
		return false, err
	}
	// EXT4_SUPER_MAGIC, also shared by ext2/ext3.
	// https://github.com/torvalds/linux/blob/v6.12/include/uapi/linux/magic.h
	if volume.Type != 0xef53 {
		return false, fmt.Errorf("case-sensitivity query unavailable for filesystem type %#x", volume.Type)
	}
	directory, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer directory.Close()
	// FS_IOC_GETFLAGS = _IOR('f', 1, long); FS_CASEFOLD_FL marks
	// case-insensitive directories. This ioctl only reads metadata.
	// https://github.com/torvalds/linux/blob/v6.12/include/uapi/linux/fs.h
	const getFlags = 0x80000000 | (unsafe.Sizeof(uintptr(0)) << 16) | ('f' << 8) | 1
	var flags uint64
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, directory.Fd(), getFlags, uintptr(unsafe.Pointer(&flags)))
	if errno != 0 {
		return false, errno
	}
	return flags&0x40000000 == 0, nil
}

func directoryNameRules(name string) (nameRules, error) {
	sensitive, err := directoryCaseSensitive(name)
	if err != nil {
		return nameRules{}, err
	}
	return linuxNameRules(sensitive), nil
}

func directoryNameRulesFD(fd int) (nameRules, error) {
	var volume unix.Statfs_t
	if err := unix.Fstatfs(fd, &volume); err != nil {
		return nameRules{}, err
	}
	if volume.Type != 0xef53 {
		return nameRules{}, fmt.Errorf("case-sensitivity query unavailable for filesystem type %#x", volume.Type)
	}
	flags, err := unix.IoctlGetInt(fd, unix.FS_IOC_GETFLAGS)
	if err != nil {
		return nameRules{}, err
	}
	return linuxNameRules(flags&0x40000000 == 0), nil
}

func linuxNameRules(sensitive bool) nameRules {
	// ext4 casefold uses canonical decomposition; ordinary directories
	// preserve byte distinctions, including normalization variants.
	// https://docs.kernel.org/admin-guide/ext4.html#case-insensitive-file-name-lookups
	// Linux's NFDICF comparison also removes Default_Ignorable_Code_Point.
	// https://github.com/torvalds/linux/blob/v6.12/fs/unicode/mkutf8data.c
	return nameRules{
		caseSensitive: sensitive, normalizationSensitive: sensitive,
		ignoreDefaultIgnorables: !sensitive,
	}
}
