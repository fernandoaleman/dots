package filesystem

import (
	"fmt"
	"syscall"
)

func directoryCaseSensitive(name string) (bool, error) {
	// _PC_CASE_SENSITIVE from Apple's sys/unistd.h.
	// https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/unistd.h
	value, err := syscall.Pathconf(name, 11)
	if err != nil {
		return false, err
	}
	if value != 0 && value != 1 {
		return false, fmt.Errorf("unknown case-sensitivity capability for %q", name)
	}
	return value == 1, nil
}

func directoryNameRules(name string) (nameRules, error) {
	sensitive, err := directoryCaseSensitive(name)
	if err != nil {
		return nameRules{}, err
	}
	var volume syscall.Statfs_t
	if err := syscall.Statfs(name, &volume); err != nil {
		return nameRules{}, err
	}
	var filesystem string
	for _, value := range volume.Fstypename {
		if value == 0 {
			break
		}
		filesystem += string(byte(value))
	}
	// APFS and HFS+ normalize names independently of case sensitivity.
	// https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/FAQ/FAQ.html
	if filesystem != "apfs" && filesystem != "hfs" {
		return nameRules{}, fmt.Errorf("name-equivalence query unavailable for filesystem %q", filesystem)
	}
	// Case-insensitive HFS+ also ignores formatting characters; APFS does
	// not. Use a conservative superset of the older HFS+ Unicode tables.
	// https://developer.apple.com/library/archive/technotes/tn/tn1150.html
	return nameRules{
		caseSensitive:           sensitive,
		ignoreDefaultIgnorables: filesystem == "hfs" && !sensitive,
	}, nil
}
