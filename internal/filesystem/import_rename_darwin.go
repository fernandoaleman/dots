package filesystem

import "golang.org/x/sys/unix"

func renameImport(from int, name string, to int, slot string) error {
	return unix.RenameatxNp(from, name, to, slot, unix.RENAME_EXCL)
}
