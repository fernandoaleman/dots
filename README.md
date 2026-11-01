# dots

A lightweight Go dotfiles manager with ordered personal and shared source
directories. dots is independent of thoughtbot/rcm and is not affiliated with
or supported by thoughtbot.

`dots up` installs missing symbolic links and necessary parent directories,
preserving existing content. `dots up --dry-run` previews the plan; `dots ls`
lists selected mappings, and `dots status` inspects current destinations.
`dots down` removes eligible recorded links; `dots down --dry-run` previews
removal. Both dry-runs, `dots ls`, and `dots status` are deterministic and
read-only. Legacy or manually created links are never adopted automatically.

## Development and usage

The application requires the Go version declared in `go.mod` or newer.
Development checks also require [prek](https://prek.j178.dev/installation/)
and Staticcheck.

Install Staticcheck using the version pinned by `STATICCHECK_VERSION` in
the Makefile, then ensure its installation directory is on PATH. Go installs
tools into `go env GOBIN` when set, otherwise into `$(go env GOPATH)/bin`.
The installer uses `GOTOOLCHAIN=auto` for that command and may download a
newer compiler to build the analyzer without changing the application's
Go requirement or module dependencies.

```sh
make install-staticcheck
make setup
```

`make setup` checks prerequisites and installs the pre-commit and
commit-message hooks. It reports installation and PATH guidance when a
required tool is missing.

```sh
go build -o dots .
./dots up --dry-run
go run . --config /path/to/config.toml up --dry-run
make fmt
make vet
make test
make lint
make check
```

`make lint` runs package-aware `staticcheck ./...` without modifying code.
`make hooks` runs the full prek suite, preserving the former hook-running
lint functionality. `make check` runs formatting, vet, tests, build,
Staticcheck, and all configured hooks.

Run `python3 scripts/acceptance.py` to build a disposable installation and test
the CLI through PATH with isolated HOME/XDG fixtures. CI runs it on macOS and
Linux. See [acceptance testing](docs/TESTING.md) for coverage, retained logs,
explicit skips, and the distinction from future release installation testing.

The native TOML `staticcheck.conf` retains recommended defaults and adds
package documentation, identifier naming, error formatting, and exported
comment-prefix checks. These checks validate specific conventions; they
do not establish complete API documentation coverage.

The Staticcheck pre-commit hook runs when staged Go files are present.
It receives no filenames and checks the whole module. It performs no fixes
and blocks the commit on diagnostics or a missing executable. The existing
gofmt and other hooks retain their behavior.

Dependency-only and lint-configuration-only commits do not trigger this
Go-file hook. `make lint` and CI still check the module unconditionally.

`--config` works before or after `up`, `down`, `ls`, `status`, and `add`. On `up`, `down`, and `add`, `-n`
aliases `--dry-run`. Use `dots --help` or a command's `--help` for usage.
`ls` and `status` are inherently read-only and reject `--dry-run`, `-n`,
and installation-only flags. JSON output is not available.

## Importing an existing dotfile

```sh
dots add --dry-run ~/.zshrc
dots add ~/.zshrc
dots add --root ~/dotfiles-local --dry-run ~/.config/nvim/init.lua
```

Exact syntax: `dots [--config FILE] add [--config FILE] [--root DIR] [--dry-run|-n] PATH`.
Flags precede the single PATH. Supply `--config` only once, before or after `add`.
The input must be an existing current-user-owned
regular file with one hard link and ordinary permissions, inside the configured
target namespace. Directories, symlinks (including already managed links),
special files and ambiguous path spellings refuse. Relative paths and leading
`./` work from that namespace; relative `..` and alternate target aliases are
not guessed. Configured symlink-plus-`..` paths retain their meaning.

The sole configured source root is selected by default; multiple roots require
`--root DIR`. Roots must already exist. There is no fallback to another root.
`~/.zshrc` normally imports to `ROOT/zshrc`, and `.config/nvim/init.lua` to
`ROOT/config/nvim/init.lua`. Nested names are preserved; source naming must
round-trip through the existing resolver. Precedence, literal exclusions,
all existing providers (including losers), and native collisions are checked.
Existing source content is never replaced, and existing symlinks are never adopted.

Preview displays ROOT, RELOCATE, SYMLINK and any missing PARENT entries. It
creates nothing, takes no exclusive lock and performs no synchronization.
It grants no authority: execution reloads TOML, resolves again, and repeats
filesystem and ownership checks before bootstrap and under the stable lock.

Execution preserves existing ownership history, creates private state and the
stable lock on first use, and records a durable intent before relocating the
file. It uses exclusive same-filesystem rename, then create-only symlink
installation and ownership recording. There is no copy/delete or overwriting
fallback. `IMPORTED` is reported only after successful completion. Exit codes
are 0 for completion/valid preview, 1 for conflicts or operational/output failure,
and 2 for invalid usage. Duplicate selection flags and false dry-run overrides
are rejected. Repeating add on an imported file refuses; use `status` to inspect it.

On failure, preserve both reported locations and registry evidence. The original
pathname may be absent after relocation; content may remain in the repository.
Confirmed step flags report successful operations, not proof that an uncertain
operation had no effect. No retry, rollback, adoption or parent cleanup occurs.
Safely created private state/lock files may remain after bootstrap failure;
later failures may retain source parents and an unfinished barrier. Inspect with
`dots recover` before selecting recovery. Output failure does not undo import.

`dots status` checks the installed link. `dots down --dry-run` previews its
eligibility; `dots down` removes only the verified owned link, retaining the
imported source and parent directories. `dots up` can install the retained source
later. Cross-filesystem imports and unsupported exclusive-rename semantics refuse.
The documented cooperative concurrency and bounded alias protections still apply;
see [import safety](docs/IMPORT.md).

## Inspecting and recovering interrupted imports

`dots recover` reads existing ownership state without TOML configuration,
exclusive locking, initialization, synchronization or writes. It displays the
registry location/revision, unfinished barrier, pending intent scope and paths,
observed file slots and evidence failures. A successful inspection can report
blocked recovery: it is an observation, not authorization to change state.

```sh
dots recover
dots recover --retire INTENT_ID --revision N --dry-run
dots recover --retire INTENT_ID --revision N
dots recover --settle --revision N --dry-run
dots recover --settle --revision N
```

Replace `INTENT_ID` with the displayed 32-character lowercase hexadecimal ID
and `N` with the exact displayed unsigned decimal registry revision. Recovery
uses `$XDG_STATE_HOME/dots/ownership.json`, falling back to
`~/.local/state/dots/ownership.json` for unset, empty or relative XDG values.
`--config` is rejected. Each mutation independently reloads and validates state
under the existing exclusive registry lock; stale selections refuse.

Retirement requires exactly one intact regular file at the recorded input or
source location and a verified absent counterpart. It verifies ancestry,
identity, metadata and content, invalidates prior owned/removing authority while
preserving history, and retires only the intent. It never moves or modifies user
files or symlinks. Success prints the surviving file's location and the new
revision, leaving the unfinished barrier in place.

Inspect again before separately selecting settlement at that new revision.
Settlement refuses pending imports and unfinished owned/removing history. It
preserves eligible records and the loaded registry version, clearing only the
barrier. It does not repair user paths or complete the interrupted import.
Missing state with revision zero and already-clean state at the exact revision
are no-ops that initialize, rewrite and synchronize nothing.

Both dry-runs acquire no exclusive lock and make zero writes. Their eligibility
reports grant no authority to a later invocation. Exit 0 means successful
inspection, eligible preview, completed action or no-op; exit 1 means blocked
selected recovery, operational uncertainty or output failure; exit 2 means
invalid usage. Duplicate, incomplete and conflicting action flags are rejected.
An output failure does not undo a completed registry action: inspect again.

If recovery refuses, preserve both displayed locations and registry evidence;
investigate the reported blockers rather than retrying blindly. Symlinks,
competing occupants, missing files or changed evidence can require manual
investigation. Manual editing of `ownership.json` is unsupported. No automatic
resume, rollback, adoption or cleanup occurs. The cooperative concurrency
contract still applies. See [recovery details](docs/IMPORT.md#failure-and-recovery).
Recovery never invokes `dots add` or resumes an interrupted import automatically.

The target platforms are macOS and Linux on arm64 and amd64. CI uses the
Go version declared in `go.mod` and checks formatting, vet, tests,
Staticcheck, build, and prek on macOS and Linux. CI uses the Makefile's
pinned Staticcheck installer and caches Go dependencies and hook environments.

## Try a disposable fixture

Run this from the repository root to preview two source directories without
using your own dotfiles. These setup commands create sample files in a new
temporary directory; the dry-run only reads them. The target remains absent
until the installation commands below are run.

```sh
dots_fixture="$(mktemp -d)"
mkdir -p "$dots_fixture/local" "$dots_fixture/shared/config/nvim"
printf 'personal\n' > "$dots_fixture/local/gitconfig"
printf 'shared\n' > "$dots_fixture/shared/.gitconfig"
printf 'set number\n' > "$dots_fixture/shared/config/nvim/init.lua"
export XDG_STATE_HOME="$dots_fixture/state"

cat > "$dots_fixture/config.toml" <<'EOF'
dotfiles_dirs = ["local", "shared"]
target_dir = "target"
EOF

go run . --config "$dots_fixture/config.toml" up --dry-run
```

With `<fixture>` standing for the absolute temporary directory, expect:

```text
LINK "<fixture>/target/.config/nvim/init.lua" -> "<fixture>/shared/config/nvim/init.lua"
LINK "<fixture>/target/.gitconfig" -> "<fixture>/local/gitconfig"
SUMMARY 2 planned, 0 correct, 0 conflicts, 0 warnings, 0 errors, 0 changes made
```

The first source's `gitconfig` wins over the shared `.gitconfig`, while the
shared Neovim file contributes a distinct destination. To install only into
this fixture and verify idempotency:

```sh
go run . --config "$dots_fixture/config.toml" up
go run . --config "$dots_fixture/config.toml" up
```

The fixture also isolates ownership state through `XDG_STATE_HOME`.
The first invocation creates two real symlinks and three parent directories.
The second reports two correct links and zero destination changes. Source contents are
never copied. The sample files remain in the temporary directory for
inspection and can be discarded afterward.

## Removing recorded links

Using the same disposable fixture and isolated state directory:

```sh
go run . --config "$dots_fixture/config.toml" down --dry-run
go run . --config "$dots_fixture/config.toml" down
```

The preview shows `REMOVE` for eligible links. Execution shows `REMOVED`;
`MISSING` describes absent recorded destinations, `PRESERVE` describes
unresolved/changed entries, and `ERROR`/`SKIP` report failures and unattempted
work. Output is sorted by destination. Preflight conflicts prevent all
removals; runtime failures retain partial success. Source referents and
parent directories are never cleaned up.

| Command | Exit 0 | Exit 1 | Exit 2 |
| --- | --- | --- | --- |
| `down`, `down --dry-run` | Successful recorded plan/removal, including no records or verified missing entries; warnings alone permitted | Preserved conflicts, state/configuration/inspection failures, partial failure, or output failure | Invalid usage |

Only newly created links are tracked in
`$XDG_STATE_HOME/dots/ownership.json`, defaulting to
`~/.local/state/dots/ownership.json` for unset, empty, or relative values.
Private state uses `0700` directories and `0600` files. Existing M1/M2 and
manual links remain untracked even when `up` reports `OK`: target equivalence
does not establish ownership. There is no adoption, force, or relinking.

A restrictive umask that removes owner permissions can make first-time state
creation fail, leaving incomplete private state while preserving managed paths.
dots does not automatically repair its permissions. See the
[bootstrap recovery procedure](docs/OWNERSHIP.md) before retrying.

Removal selects the current configuration and target. Historical targets
remain recorded without automatic removal. Root ordering, exclusions,
missing sources, and broken referents do not redefine ownership; `down`
loads configuration without scanning sources. Recorded broken destination
links remain eligible when their own identity and ancestry match.

**External processes must not modify affected destinations or ancestors
during `up` creation/registration or `down` validation/removal.** The stable
registry lock coordinates dots processes, not arbitrary filesystem writers.
Descriptor-relative operations and immediate identity checks cannot protect
against replacement after the final check.

Interrupted creation never grants ownership automatically. An unfinished
transaction conservatively invalidates prior removal authority; intact
links may therefore need manual recovery. Corrupt/unsupported state fails
without automatic reset. Uncertain removals retain records; completed calls
are counted even when later persistence fails. Dry-run never initializes or
locks state, performs recovery, or writes temporary files. Actual commands
can update bookkeeping while reporting zero destination changes.

See [ownership, transactions, and recovery](docs/OWNERSHIP.md) for state
transitions, permissions, filesystem limits, and the manual recovery policy.

## Listing and status

Using the disposable fixture above:

```sh
go run . --config "$dots_fixture/config.toml" ls
go run . --config "$dots_fixture/config.toml" status
```

`ls` shows the desired mappings, regardless of destination occupancy:

```text
SELECTED "<fixture>/target/.config/nvim/init.lua" -> "<fixture>/shared/config/nvim/init.lua"
SELECTED "<fixture>/target/.gitconfig" -> "<fixture>/local/gitconfig"
SUMMARY 2 selected, 0 conflicts, 0 warnings, 0 errors, 0 changes made
```

It uses the same configuration, exclusions, precedence, and source structural
checks as `up`. Source conflicts remain visible with `CONFLICT` rows, and
warnings and errors follow the mappings. Empty source directories contribute
structural evidence rather than mapping rows. `ls` does not inspect destination
occupancy, ancestors, or physical alias/name collisions; use `status` for those.

`status` inspects only the current resolver-selected destinations. After the
fixture installation, it reports `OK` for both links. Before installation,
it reports `MISSING`, including when parent directories are absent. Neither
command creates directories, links, configuration files, or other artifacts.

| Status row | Meaning |
| --- | --- |
| `OK` | Existing link points to the selected named source entry |
| `MISSING` | Destination or required parents are absent |
| `WRONG` | Usable symlink points to a different source entry |
| `BROKEN` | Destination symlink has a missing referent or a non-directory target component |
| `FILE`, `DIRECTORY`, `SPECIAL` | Destination is occupied by that entry type |
| `BLOCKED` | A required ancestor is obstructed or traversal cannot be resolved |
| `CONFLICT` | Source/destination identity or structural, alias, or native-name collision |
| `ERROR` | Source or filesystem inspection failed, including unavailable name capabilities |

Rows retain quoted destination and selected-source paths, reasons, and stable
destination ordering. Collision errors retain all applicable explanations;
inspection errors take priority over confirmed conflicts. Summaries count
each row once, with occupied types grouped together. The error total includes
error rows and standalone source diagnostics, which can concern the same path.

`OK` describes current target equivalence, not ownership by dots. `WRONG` does
not imply permission to replace a link. Status is a point-in-time observation;
it does not scan for historical links or repair, relink, or remove anything.

| Command | Exit 0 | Exit 1 | Exit 2 |
| --- | --- | --- | --- |
| `ls` | Successful or warning-only resolution, including an empty plan | Configuration/source errors, structural conflicts, or output failure | Invalid usage |
| `status` | Every selected link is correct, or a valid empty plan; warnings alone are permitted | Any missing, wrong, broken, occupied, blocked, or conflicting path; inspection/configuration/source errors or output failure | Invalid usage |

Unlike `status`, `up --dry-run` still succeeds when its only proposed actions
are missing links. Both new commands accept `--config` before or after their
name, reject positional arguments, and show help without loading configuration.

## Configuration

The canonical file is `$XDG_CONFIG_HOME/dots/config.toml` when
`XDG_CONFIG_HOME` is absolute. Otherwise it is `~/.config/dots/config.toml`.
An explicit `--config PATH` selects that file without fallback on error.
There is no automatic `.rcrc`, `.dots.toml`, or extensionless fallback.
Missing configuration produces an error; dots never generates it.

```toml
dotfiles_dirs = ["~/dotfiles-local", "~/dotfiles"]
target_dir = "~"
excludes = [".git", "README.md", "LICENSE"]
warn_duplicates = true
```

`dotfiles_dirs` is required and nonempty. `target_dir` defaults to home;
`excludes` defaults to an empty list; `warn_duplicates` defaults to true.
Malformed TOML, unknown keys, invalid types, and empty paths are errors.

Leading `~` expands in source, target, and explicit configuration paths.
Other shell interpolation is not performed. Relative `--config` paths use
the working directory; relative source and target paths use the selected
configuration file's directory.
Paths preserve filesystem traversal through symlinks and `..`. Traversal
through `..` beneath a missing ancestor is reported as a conflict.

Exclusions are literal, canonical paths relative to each source root,
matched before normalization and precedence. Excluding a directory prunes
its subtree. `.git` excludes only the root `.git`; `config/nvim` does not
exclude `.config/nvim`. Wildcards are literal: `*.tmp` excludes a file
named `*.tmp`, not all temporary files. Empty, absolute, escaping, and
noncanonical exclusions such as `config/` are rejected.

## Mapping and precedence

The first configured directory wins, regardless of filename spelling.
For example, local `gitconfig` overrides shared `.gitconfig`.
Within one source root, dotted variants win and produce an actionable
duplicate warning. Disabling warnings does not change the winner.
Ordinary overrides between roots do not warn.

| Source path | Destination beneath `target_dir` |
| --- | --- |
| `gitconfig` or `.gitconfig` | `.gitconfig` |
| `config/nvim/init.lua` or `.config/nvim/init.lua` | `.config/nvim/init.lua` |
| `config/git/config` | `.config/git/config` |

Only the top-level component gains a dot. Nested directories merge file
by file, retaining unique descendants from both roots and from dotted
and undotted trees.

Missing roots warn if another usable root exists; empty roots are usable.
No usable roots, unreadable roots, and non-directory roots are errors.
Configured roots may be symlinks to accessible directories. Nested source
symlinks are opaque entries and are never recursively traversed; dangling
links and unsupported source types are errors.

## Plans and safety

Output uses quoted absolute paths, ordered by destination:

```text
LINK "/home/example/.gitconfig" -> "/home/example/dotfiles-local/gitconfig"
SUMMARY 1 planned, 0 correct, 0 conflicts, 0 warnings, 0 errors, 0 changes made
```

`LINK` means missing, `OK` means already linked to the selected source,
and `CONFLICT` means blocked. Conflicts distinguish existing regular files,
directories, wrong or broken links, blocked ancestors, self-reference,
and structural file/directory collisions. Existing ancestor directory
symlinks are inspected, including collisions they introduce between plans.
Destination occupancy never changes source selection.
Case and Unicode-normalization collisions use read-only directory capability
queries on macOS APFS/HFS+ and Linux ext4. The pinned `golang.org/x/text`
dependency supplies canonical normalization and full Unicode folding, which
the standard library does not provide. Comparisons are conservative because
filesystem Unicode tables can differ by version: ambiguous pairs may be
blocked even when a particular filesystem distinguishes them. If capabilities
are unavailable, potentially ambiguous entries are conflicts; unrelated
entries remain visible.

The comparison also accounts for ignorable Unicode characters in ext4
casefold directories and case-insensitive HFS+ volumes. APFS preserves
those distinctions.

Source directories, including empty ones, remain structural collision
evidence even though they do not produce link actions. Directory trees may
merge, but a file cannot occupy a required directory's destination.

Warnings and source diagnostics follow the entries. Structural conflicts
retain their participants and do not hide unrelated entries. No links
are automatically replaced or removed by `up`, including broken links.

The executable exits 0 for clean or warning-only plans, 1 for runtime
errors or conflicts, and 2 for invalid usage. `--force` is unavailable.

## Installing safely

Review `dots up --dry-run`, then run `dots up` with the same configuration.
Both commands use the same resolver-selected sources. Existing correct
absolute or relative links are left unchanged. Files, directories, wrong
links, and broken links are conflicts; nothing is automatically replaced.

Installation validates the entire plan before writing. Any source error,
conflict, or preparation failure prevents all destination changes, including parent
creation. Warnings alone permit installation. Dry-run retains its read-only
inspection path; it cannot guarantee later write permissions or resources.

The filesystem package prepares directory handles, then creates parents and
symlinks relative to those handles. The pinned `golang.org/x/sys v0.35.0`
dependency supplies macOS/Linux primitives while retaining Go 1.23 support.
The filesystem package also maintains the private ownership registry;
no filesystem framework is used.

Existing directory symlinks are accepted, including those resolving outside
`target_dir`. Their resolved directory identities are pinned. Newly
substituted parent symlinks are never followed during creation. Observed
ancestor or source-entry changes abort installation; source contents may
still be edited in place. New links store the selected absolute source path,
preserving configured symlink-plus-`..` traversal.

New parents use `0700`, subject to the existing umask. Existing permissions
and process umask are unchanged. If an unexpected parent or destination
appears after preflight, installation stops, even for a concurrent correct
link. A fresh invocation can recognize that link as satisfied.

On a runtime failure, installation stops at the first failing entry. Completed
links and directories remain, and later entries are unattempted. There is no
automatic rollback or cleanup. Fix the reported problem, review a new dry-run,
then rerun; completed correct links will be skipped. An output failure also
does not undo writes; stderr reports the already completed counts.

Installation reports `LINKED`, `OK`, `CONFLICT`, `ERROR`, and `SKIP` rows in
destination order. Its summary counts successful link and directory creation
calls; total changes are their sum. Counts describe operations completed by
dots, not a permanently frozen filesystem snapshot.

Directory handles resist symlink redirection but cannot freeze names. A
concurrent rename after final validation can move the pinned directory;
creation remains attached to that directory rather than a replacement path.
An ordinary directory substituted before the first post-mkdir identity
observation cannot always be detected. Real symlinks also cannot freeze source
referents after their final validation. These limits are tested explicitly;
installation is create-only, not a filesystem transaction.

Prepared plans are single-use and own deduplicated directory handles. Handles
are released after application, failure, or explicit abandonment. Resource
exhaustion fails cleanly; dots does not raise limits or fall back to unchecked
pathname writes.

See [the handoff](docs/HANDOFF.md) for requirements and
[migration from rcm](docs/MIGRATING_FROM_RCM.md) for manual migration.
See [ownership and removal safety](docs/OWNERSHIP.md) for the registry-backed
installation/removal contract. Replacement, broad cleanup, and
releases remain outside this milestone.

## License

[MIT](LICENSE). The Go module path is `github.com/fernandoaleman/dots`.
No release installation URL has been assigned.
