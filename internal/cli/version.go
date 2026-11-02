package cli

import "fmt"

// Set only by explicitly stamped builds, e.g. -ldflags '-X .../cli.version=v0.1.0'.
// A checkout's tags never turn an ordinary go build into an official release.
var version = "dev"
var commit = "unknown"

func versionText(v, revision string) string {
	if v == "" || v == "dev" {
		if revision == "" {
			revision = "unknown"
		}
		return fmt.Sprintf("dots dev (commit %s)\n", revision)
	}
	return fmt.Sprintf("dots %s\n", v)
}
