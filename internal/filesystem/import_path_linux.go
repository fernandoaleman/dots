package filesystem

// Linux retains pinned namespace and known-object identity checks. Do not
// require procfs, mount privileges, or a global mount inventory for inspection.
func importDirectoryPath(int) (string, error) { return "", nil }
