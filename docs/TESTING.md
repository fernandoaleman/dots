# Acceptance testing

Run from a checkout with Python 3, make, and the Go version in go.mod available:

```sh
python3 scripts/acceptance.py
```

The shared macOS/Linux script runs `make build BINARY=<private-temp>/bin/dots`.
It then invokes `dots` by name through an isolated PATH, with that bin directory
first and only system command directories afterward. This is a disposable test
installation from the current checkout, not a system installation. Release
artifacts, package managers, and distribution installers belong to M5 and are
not validated here.

Each fixture has a private HOME, XDG config/state directories, temporary directory,
repository, and known input bytes. No command runs against the developer's real
HOME or ownership state. The build runs before replacing the child CLI's
environment so Go can use its normal toolchain and module cache.

The script checks actual exit codes and filesystem evidence, including file and
directory identity, permissions, timestamps, content hashes and symlink text.
Read-only commands, previews, and rejected operations compare complete fixture
snapshots (excluding access times). An unrelated sentinel is checked after every
invocation. A successful import must preserve the file inode and bytes, create
the expected link, and record the actual link identity in the v2 registry.
Registry JSON is inspected, never fabricated or edited.

Coverage includes fresh-state inspection and bootstrap; ordinary and nested
imports; an existing registry; configuration selection; duplicate config scopes;
argument/path refusals; root selection and precedence; repository collisions;
manual links; duplicate import; ownership history; ls/status; down preview,
down, repeated down, up and subsequent down. Recovery inspection and missing-state
settlement/no-op are tested without a live configuration requirement.

The top-level temporary directory uses the system `/tmp` base rather than the
caller's TMPDIR, keeping the Makefile build destination free of shell-sensitive
characters. Each CLI process uses its own fixture TMPDIR underneath it.
Fixtures and `results.json` command transcripts are retained at the printed
temporary path on success and failure. There is no automatic cleanup. Local
removal is a separate user decision. CI runner disposal handles its ephemeral
fixtures. Cross-filesystem refusal uses an existing writable Linux `/dev/shm`
only when it is a distinct device, never mounts or modifies runner disk layouts,
and prints the second retained fixture path. Unavailable cases print `SKIP`
with the reason; they are not reported as executed coverage.

## CI and lower-level boundaries

The existing CI workflow still runs all previous checks. Its separate Acceptance
job runs on both Ubuntu and macOS for pushes and pull requests, using the same
script. Any failed assertion fails the job. The script prints per-group results,
command counts and elapsed time; the retained transcript includes each command's
output, exit code and duration.

There is no supported deterministic interruption hook in the external CLI.
The script explicitly skips creation of interrupted states rather than inventing
registry data or adding production test hooks. The job also runs these existing
Go integration tests with verbose output:

```sh
go test -count=1 -v -timeout=5m ./internal/filesystem \
  -run '^(TestImportCLI|TestRecoveryCLI|TestInspectImportLinuxBindConfiguration|TestImportSelectedInputMountView)$'
```

`TestImportCLI` and `TestRecoveryCLI` create legitimate prepared/relocated states
using private fixture/fault-injection helpers and invoke a real compiled CLI for
inspection, previews, stale selections, retirement and separate settlement.
These remain lower-level integration coverage, not PATH-installation coverage.
Crash, persistence, capacity, registry-continuity, and hostile substitution tests
remain in the full Go suite; the acceptance script does not replace them.

Native Linux bind/mount-view tests use isolated child namespaces with existing
capabilities only. Verbose `PASS`/`SKIP` lines and reasons distinguish execution
from unavailable namespace, mount, or second-filesystem prerequisites. macOS
excludes Linux-only tests at build time; existing macOS tests cover its native
filesystem behavior. No privileged runner configuration is required.
