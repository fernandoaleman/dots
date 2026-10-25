# dots — Project Handoff for Codex

**Status:** Architectural reference and milestone history; active GitHub issues define current feature scope

**Project name:** `dots`  
**CLI executable:** `dots`  
**Implementation:** Go  
**Purpose:** A lightweight, predictable, cross-platform dotfiles manager inspired by thoughtbot/rcm, not affiliated with or supported by thoughtbot.

## 1. Project vision

Build a simple, modern, single-binary dotfiles manager that keeps the best part of rcm: **ordered, layered dotfiles directories**, allowing an organization or team to maintain shared defaults while each person provides local overrides. Avoid the complexity of templating frameworks such as chezmoi, forced `dot_` filenames, or elaborate configuration management.

Core principles:

- **One Git repository, one Go module, one `dots` executable** with subcommands; not separate `rcup`, `rcdn`, `mkrc`, and `lsrc` binaries.
- **Minimal dependencies**, idiomatic Go, simple readable code, predictable operations.
- **Safety first:** never silently replace an unmanaged file or directory; start with read-only dry runs.
- **Portable:** macOS (Apple Silicon and Intel), Linux (amd64 and arm64), including Arch and Ubuntu. Linux distributions of the same architecture generally share a binary.
- **Configuration uses XDG**, but do not force XDG conventions on managed applications or user dotfiles.
- **No mandatory templates or renaming:** support source names both with and without leading dots.
- **Repeatable operations:** running `dots up` twice should not damage or unnecessarily change correct links.
- **Compatibility goal:** preserve practical rcm source naming and directory-precedence behavior, but this is a distinct project, not a claim of full rcm CLI/config compatibility.

## 2. Configuration: TOML and XDG

**Canonical configuration:** `$XDG_CONFIG_HOME/dots/config.toml`, or `$HOME/.config/dots/config.toml` when `XDG_CONFIG_HOME` is unset or empty. Follow XDG conventions: only use an absolute `XDG_CONFIG_HOME` value; ignore an invalid relative value and use the home-directory default.

**Config lookup order:**

1. Explicit `--config PATH`, when provided. If missing, unreadable, or invalid, report an error; **never silently fall back**.
2. The single XDG configuration path described above.
3. No automatic fallback. If the file is absent, explain where to create it and show a minimal example (or a future explicit initialization command).

**Do not automatically search or parse:** `$HOME/.dots.toml`, `$HOME/.rcrc`, `$HOME/rcrc`, `$HOME/.config/dots/config`, or arbitrary `*.conf` files. In particular, `~/.rcrc` is **not** supported as a configuration fallback. A user may still choose to *manage* a `.rcrc` dotfile as ordinary content; that does not make it a dots configuration.

**Why no `~/.dots.toml` fallback?** A single XDG location and `--config` give predictable behavior and avoid split-brain configuration. A home-directory fallback may be reconsidered after real-world demand, but is explicitly out of initial scope.

**Format:** TOML 1.0-style configuration, using a well-maintained Go TOML decoder. Prefer a small dependency over writing a custom parser. Fail clearly on malformed TOML, unknown keys (where practical), invalid types, or an empty required source list. Never execute shell snippets or source legacy `.rcrc` files.

Example `~/.config/dots/config.toml`:

```toml
# First source directory wins; personal overrides precede shared settings.
dotfiles_dirs = [
  "~/dotfiles-local",
  "~/dotfiles",
]

# Root destination; do not force XDG paths onto the user's dotfiles.
target_dir = "~"

# Ignore repo metadata and documentation (match literal source-root-relative paths).
excludes = [".git", "README.md", "LICENSE"]

# Duplicate dotted/undotted names in a single source root are nonfatal.
warn_duplicates = true
```

Schema expectations for the first milestone:

- `dotfiles_dirs`: required, nonempty ordered array of directory paths; first directory has highest priority.
- `target_dir`: optional destination root, defaulting to the user's home directory.
- `excludes`: optional array of literal original source-root-relative paths, matched before normalization and precedence. Excluded directories prune subtrees. No glob support in Milestone 0.
- `warn_duplicates`: optional boolean, default `true`. Even when warnings are disabled, dotted-name tie-breaking remains deterministic.
- Expand leading `~` in *path-valued fields* only; optionally support `${HOME}` as a documented convenience. Do not implement general shell interpolation, command substitution, or environment-variable execution.
- Resolve relative `--config` paths against the process working directory. Keep paths absolute while preserving filesystem traversal through symlinks and `..`.
- Resolve relative source and target paths against the selected configuration file's directory. Reject unknown TOML keys and empty paths. Exclusions must be canonical relative paths; wildcard characters are literal.
- Use the same schema and validation for an explicitly supplied TOML config, regardless of filename.

**Bootstrapping:** `dots --config /path/to/config.toml up --dry-run` must work before the canonical config exists. Installing or managing `~/.config/dots/config.toml` as a dotfile is allowed, but self-referential links and source/target collisions must be detected safely. The config file must be read before planning or creating any links. Never create configuration automatically as a side effect of a dry run.

**Migration from rcm:** Provide `docs/MIGRATING_FROM_RCM.md` (outlined in Section 10). Conversion is manual and documented: transfer relevant `DOTFILES_DIRS` entries into `dotfiles_dirs = [...]`, preserve ordering, convert supported excludes, and verify results with `dots up --dry-run`. Do not silently interpret `.rcrc` shell content as TOML. Existing dotted/undotted dotfile source naming remains supported independently of config syntax.

## 3. Core dotfile destination and precedence rules

**Target root:** `$HOME` by default, without compelling users to use `$XDG_CONFIG_HOME` for arbitrary programs.

**Ordered directory precedence (highest priority first):**

```text
~/dotfiles-local/
~/dotfiles/
```

Given:

```text
~/dotfiles-local/
  gitconfig
  zshrc

~/dotfiles/
  .gitconfig
  zshrc
  tmux.conf
  config/nvim/init.lua
```

The intended destinations are:

```text
~/.gitconfig              -> ~/dotfiles-local/gitconfig
~/.zshrc                  -> ~/dotfiles-local/zshrc
~/.tmux.conf              -> ~/dotfiles/tmux.conf
~/.config/nvim/init.lua   -> ~/dotfiles/config/nvim/init.lua
```

### Source filename normalization

- At the *top level of a source directory*, `gitconfig` and `.gitconfig` both map to `~/.gitconfig`.
- `rcrc` and `.rcrc` both map to `~/.rcrc` (when managed as target dotfiles; config loading precedence is separate).
- `config/nvim/init.lua` and `.config/nvim/init.lua` both map to `~/.config/nvim/init.lua`.
- Similarly, `config/git/config` and `.config/git/config` map to `~/.config/git/config`.
- **Do not prefix every nested path component with a dot.** For example, nested `git/config` stays `git/config`.
- Plain files nested under normalized directories retain literal names.
- Files that already have a leading dot must not acquire a second dot.

### Winner rules (mandatory)

1. **Source directory order always wins first.** The first configured directory supplying a destination takes precedence over all lower-priority directories, regardless of whether the source name is dotted.
2. **Within a single source directory**, if both `.gitconfig` and `gitconfig` normalize to `~/.gitconfig`, the **dotted** version wins. Ditto `.config` vs `config` for competing paths.
3. Emit a **nonfatal, actionable warning** when ambiguous dotted/undotted variants occur **within the same priority tier**; display winning and ignored source. Do **not** warn simply because `dotfiles-local` intentionally overrides `dotfiles`.
4. Deterministic sorting / display so the same setup always produces the same plan.
5. For nested trees, support **file-level merging** across the two source directories. Avoid symlinking whole `~/.config` directories as a shortcut: that would block lower-priority files from contributing their own distinct destinations.

Examples:

| dotfiles-local (high priority) | dotfiles (low priority) | Winner / warning |
| --- | --- | --- |
| `gitconfig` | `.gitconfig` | local `gitconfig`, no cross-tier warning |
| `.gitconfig` | `gitconfig` | local `.gitconfig`, no cross-tier warning |
| `.gitconfig` and `gitconfig` | `gitconfig` | local `.gitconfig`, warn about local duplicate |
| none | `.gitconfig` and `gitconfig` | shared `.gitconfig`, warn about shared duplicate |

**Directory collision nuance to test:** If a single source root contains both `config/` and `.config/`, recursively normalize paths, prefer equivalent dotted-path variants while still retaining distinct files in each tree. When parent paths collide with a file or symlink, detect/report a conflict rather than silently dropping entries. Clarify directory traversal, symlinks in source trees, and file-vs-directory conflicts through focused test fixtures before implementing writes.

## 4. CLI commands

The long-term CLI design uses one binary and the subcommands below. Consult
the README for current availability. Historical/stale-link discovery and
ownership handling remain future work rather than inspection guarantees.

| Command | Purpose | Relationship to original rcm |
| --- | --- | --- |
| `dots up` | Install or reconcile managed symlinks safely | `rcup` |
| `dots down` | Remove links currently managed by dots, without deleting actual files | `rcdn` |
| `dots ls` | List planned/resolved managed files and destinations | `lsrc` |
| `dots add PATH` | Import an existing dotfile into a chosen source directory, then manage it safely | `mkrc` |
| `dots status` | Report correctly linked, missing, conflicting, and stale paths | New convenience command |
| `dots version` / `dots --version` | Print build version | Standard CLI |
| `dots help` / `dots --help` | Help and usage | Standard CLI |

Suggested options (design intent; specify exact compatibility in help):

| Option | Expected purpose |
| --- | --- |
| `--dry-run` / `-n` | Preview intended actions without filesystem modifications. Available at least on `up`, `down`, `add`. |
| `--config PATH` | Override selected dots configuration file. |
| `--verbose` / `-v` | Display more details, including source selections. |
| `--quiet` / `-q` | Reduce nonessential output; errors and meaningful safety warnings remain visible. |
| `--version` | Print version and exit. |
| `--help` / `-h` | Help on root and individual commands. |
| `--force` | **Deferred** pending a precise backup/overwrite policy; never introduce a dangerous implicit overwrite. |
| `--target PATH` | Optional future enhancement for an alternate destination root, particularly useful for integration tests. |

Proposed usage:

```sh
dots up --dry-run
dots --config ~/.config/dots/config.toml up --dry-run
dots up
dots ls
dots status
dots down --dry-run
dots add ~/.zshrc
dots version
```

If implementing a standard library-only CLI is straightforward, prefer that initially; a CLI package such as Cobra can be evaluated later, but isn't mandatory.

## 5. Code architecture: 3 logical components, one repo

Suggested structure:

```text
dots/
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml           # add when release packaging begins
├── internal/
│   ├── cli/
│   │   ├── cli.go
│   │   └── cli_test.go
│   ├── resolver/
│   │   ├── resolver.go
│   │   └── resolver_test.go
│   └── filesystem/
│       ├── filesystem.go
│       └── filesystem_test.go
├── docs/
│   ├── HANDOFF.md
│   └── MIGRATING_FROM_RCM.md
├── main.go
├── go.mod
├── README.md
├── CHANGELOG.md
├── LICENSE
├── .gitignore
└── CONTRIBUTING.md            # optional early; helpful for contributors
```

- **`internal/cli` — front door:** Parse subcommands/flags, invoke resolver configuration loading and resolution, orchestrate inspection and installation, and format output. No low-level symlink logic.
- **`internal/resolver` — brains:** Discover/load TOML configuration, apply exclusions, read the configured source locations, traverse files, normalize source paths, apply ordered precedence and dotted-file tie-breaking, generate a deterministic plan of `{Source, Destination}` plus warnings. This package does **not** modify the filesystem.
- **`internal/filesystem` — hands:** Inspect destinations and ancestors, distinguishing missing paths, correct links, conflicts, and broken links. Milestone 1 adds prepared create-only symlink installation and safe parent creation.

For the first proof of concept, `filesystem` may only provide nonmutating inspection helpers. Avoid unnecessary interfaces, plugin systems, or abstraction layers. Unit tests live alongside the packages (`*_test.go`). Use `t.TempDir()` for isolated fixtures and never tests that modify the real home directory.

Potential future internal config package is fine if parsing becomes sufficiently complex, but no need to start with four packages for a tiny project. The **three components** refer to responsibility boundaries, not separate repos, binaries, services, or release units.

## 6. First milestone: minimal proof of concept

**Milestone 0: `dots up --dry-run` only.** Build a working, read-only command before allowing writes to the home directory.

Deliverables:

1. Initialize Go module and a minimal `main.go` that delegates to `internal/cli`.
2. Create `README.md`, `CHANGELOG.md`, `LICENSE`, `.gitignore`, `.github/workflows/ci.yml` **on the first commit** (choose a license explicitly; MIT or BSD-3-Clause are reasonable candidates; don't claim thoughtbot endorsement).
3. Implement configuration discovery: explicit `--config` first, otherwise the canonical XDG `config.toml`; no home-directory fallback. Parse and validate the documented TOML schema, including ordered `dotfiles_dirs` and leading-`~` expansion.
4. Implement resolver scanning + destination normalization + local-over-shared precedence + dotted-over-undotted tie breaks + warnings.
5. Implement `dots up --dry-run` to **print intended links and conflicts, without creating directories, links, backups, or config files**.
6. Unit tests for config search, filename normalization, duplicate collisions, nested directory merge, order-dependent precedence, and missing paths.
7. Run `go test ./...`, `go vet ./...`, and `gofmt` checks in CI on every push and pull request.
8. Document how to run locally, including realistic test fixtures and sample output.

Sample proof-of-concept output:

```text
$ dots up --dry-run
LINK   ~/.gitconfig             -> ~/dotfiles-local/gitconfig
LINK   ~/.zshrc                 -> ~/dotfiles-local/zshrc
LINK   ~/.tmux.conf             -> ~/dotfiles/tmux.conf
LINK   ~/.config/nvim/init.lua  -> ~/dotfiles/config/nvim/init.lua
SUMMARY 4 planned, 0 correct, 0 conflicts, 0 warnings, 0 errors, 0 changes made
```

Dry-run output should distinguish `LINK` (missing), `OK` (already correct), `CONFLICT` (destination occupied by other content), and eventually `RELINK` (stale managed link) instead of claiming every destination always needs creation.

**Definition of done:** `go run . up --dry-run` produces a stable, correct action plan from a two-directory fixture, all tests pass, CI passes, and no writes occur to the user's home directory.

## 7. Subsequent milestones (not required for first PoC)

**Milestone 1 — Safe `dots up`:** Idempotent link creation, missing parent directory handling, correct existing links as no-ops, unmanaged path conflicts as errors, dry-run parity with execution. Design link ownership and stale-link policy before changing existing links.

**Milestone 2 — Visibility:** `dots ls` and `dots status` reuse the exact same resolver; show path, selected source, and current link state. Useful testable text output; JSON output can wait.

**Milestone 3 — Cleanup:** `dots down --dry-run` and `dots down` remove **only symlinks provably owned by dots**. Consider how ownership is determined when configs change: manifests, strict source-path validation, or explicitly documented behavior. Never delete the contents of the linked file.

**Milestone 4 — Import:** `dots add PATH` safely moves/copies/imports source into a chosen source directory; clearly distinguish moves from copies and avoid surprises. Define defaults for choosing a source root.

**Milestone 5 — Release & Distribution:** Version reporting, macOS/Linux arm64/amd64 archives, SHA-256 checksums, tested release automation, and installation/verification/upgrade documentation. Publication requires separate approval. Optional force/backups, tags, host-specific sources, and hooks remain deferred.

## 8. Testing strategy from day one

Use Go's built-in `testing` package. Favor table-driven unit tests for resolver rules and separate filesystem integration tests in temporary directories.

Minimum test matrix:

- XDG path with and without `XDG_CONFIG_HOME`; ignore relative `XDG_CONFIG_HOME` values per specification.
- Explicit `--config` wins; no fallback if explicit config is invalid/missing. Missing canonical config gives a useful error.
- No implicit `~/.dots.toml`, `~/.rcrc`, `~/rcrc`, or extensionless config discovery.
- TOML parsing: invalid syntax, unknown keys, invalid types, empty `dotfiles_dirs`, array ordering, and path expansion.
- Two source roots with file in both; earlier root wins.
- `.gitconfig` vs `gitconfig` in one root; dotted wins and warns.
- Local `gitconfig` vs shared `.gitconfig`; local wins without warning.
- `config/nvim/init.lua` and `.config/nvim/init.lua` normalize to same target.
- Both `config/` and `.config/` in a source root, with overlapping and nonoverlapping descendants.
- No double dot; don't dot-prefix nested `git/config` basename.
- Correct link, broken link, unmanaged file, destination directory conflict.
- Source tree with nested symlinks or unexpected filesystem types; define behavior rather than blindly recursing.
- Dry-run performs zero mutations, including no creation of target parent directories.
- Config errors and inaccessible directories produce actionable diagnostics.
- Stable ordering for plans/warnings regardless of filesystem traversal order.
- Cross-platform path and symlink tests on macOS and Linux.

CI should use GitHub Actions on push and pull requests, ideally an `ubuntu-latest` and `macos-latest` matrix. Run `gofmt` validation, `go vet ./...`, `go test ./...`, and `go build ./...`; optionally `go test -race ./...` for supported environments. Pin a supported Go version in `go.mod` and keep Actions versions reasonably current. Add dependency vulnerability scanning later if dependencies are introduced.

## 9. Versioning, releases, and binaries

Use **Semantic Versioning**, tagged Git releases (`v0.1.0`, etc.), and maintain a human-readable `CHANGELOG.md` from the start (Keep a Changelog format is fine). While the API/behavior is still rapidly changing, begin with `v0.x.y`, document intentional breaking changes, and move to `v1.0.0` when core behavior is stable.

Suggested build targets:

| GOOS | GOARCH | Example archive/binary | Intended systems |
| --- | --- | --- | --- |
| `darwin` | `arm64` | `dots_Darwin_arm64.tar.gz` | Apple Silicon Macs |
| `darwin` | `amd64` | `dots_Darwin_amd64.tar.gz` | Intel Macs |
| `linux` | `amd64` | `dots_Linux_amd64.tar.gz` | Ubuntu, Arch, other x86-64 Linux |
| `linux` | `arm64` | `dots_Linux_arm64.tar.gz` | ARM64 Linux |

Simple build examples:

```sh
go build -o dots .
GOOS=darwin GOARCH=arm64 go build -o dist/dots-darwin-arm64 .
GOOS=darwin GOARCH=amd64 go build -o dist/dots-darwin-amd64 .
GOOS=linux GOARCH=amd64 go build -o dist/dots-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dist/dots-linux-arm64 .
```

Set `CGO_ENABLED=0` for release builds if the code remains pure Go and the target builds support it. Embed version information at build time (e.g., `-ldflags -X ...`) or use Go build information, and have `dots version` report it.

Release automation (once a meaningful executable is ready):

- Trigger a release workflow on pushes of version tags matching `v*`.
- Test first, then cross-compile, archive each target, generate SHA-256 checksums, and publish artifacts with a GitHub Release.
- Use least-privilege workflow permissions; release workflow gets `contents: write` only where necessary.
- Consider GoReleaser **later** for standardized cross-platform packaging, checksums, release notes, and optional Homebrew tap. Do not add it purely for a dry-run prototype.
- No separate Arch/Ubuntu binaries required when OS and architecture match; avoid promising universal ABI compatibility if cgo/external shared libraries are later introduced.

## 10. Documentation requirements

The initial `README.md` should include:

- Clear one-paragraph mission and explicit disclaimer that dots is independent of thoughtbot/rcm.
- Feature scope and current status (`v0.x` / proof of concept); avoid presenting not-yet-built commands as available.
- Installation/development prerequisites and basic `go run . up --dry-run` instructions.
- Canonical XDG `config.toml` search behavior and `dotfiles_dirs` TOML example, with higher-priority directory first.
- Source filename mappings and dotted/undotted tie rules.
- How shared `dotfiles` + personal `dotfiles-local` work together.
- Safety behavior and limitations, especially dry-run-only status at first.
- Development/test commands and supported target platforms.
- Roadmap, license, and contribution guidance.

Create `docs/MIGRATING_FROM_RCM.md` with:

1. A clear statement that dots is independent of thoughtbot/rcm and does **not** read `.rcrc`.
2. Before/after examples converting `DOTFILES_DIRS="$HOME/dotfiles-local $HOME/dotfiles"` to TOML `dotfiles_dirs = ["~/dotfiles-local", "~/dotfiles"]` while preserving first-wins precedence.
3. Guidance for manual conversion of excludes and other settings, with unsupported rcm features clearly identified rather than silently ignored.
4. A reminder that existing source layouts (such as `gitconfig` and `config/nvim/init.lua`) still resolve to the expected dotfile destinations.
5. A safe validation procedure: back up existing configuration, run `dots up --dry-run`, inspect planned links/conflicts, and switch only when satisfied. Do not automatically run `rcdn` or unlink existing files.

Use `CHANGELOG.md` consistently for user-visible changes. Keep implementation docs proportional to project size.

## 11. Decisions and cautions

**Decided:**

- Project/CLI name: **dots**.
- Language: **Go**, idiomatic PascalCase for exported names and camelCase for unexported names; `gofmt`.
- Single repo / Go module / binary; three logical internal components.
- Ordered TOML `dotfiles_dirs` with **first directory winning**; e.g., local before shared.
- Both dotted and undotted names supported; dotted wins *inside the same directory*, with nonfatal warning on duplicates.
- Canonical configuration is **`~/.config/dots/config.toml` by default**, respecting a valid absolute `$XDG_CONFIG_HOME`.
- **No automatic home-directory fallback**, including `~/.dots.toml` or `~/.rcrc`; use `--config PATH` for nonstandard locations.
- TOML is the only supported dots configuration format; legacy rcm migration is documented, not automatically parsed.
- No forced XDG layouts for managed applications.
- Test, README, changelog, and CI from first commit.
- First vertical slice: `dots up --dry-run` only.

**Open technical decisions Codex should settle and document rather than guess:**

- TOML decoding is settled: BurntSushi TOML v1.6.0 with unknown-key rejection.
- What counts as an owned symlink for later updates/deletion; how ownership survives changes to the source order.
- Windows is **out of initial scope**; macOS and Linux only.
- `--force` semantics and backups are **out of initial scope**.
- License: MIT. The Go module path is `github.com/fernandoaleman/dots`, matching the assigned GitHub repository.

## 12. Instructions to local Codex

Read the user-designated active [GitHub milestone](https://github.com/fernandoaleman/dots/milestones)
and its linked issues before implementation. Their acceptance criteria and
discussion are authoritative for scope; contracts below preserve established
decisions and milestone history. Milestones 0 and 1 are merged and CI-validated.
Prefer small Go code and fixture-backed tests. The lead alone edits files;
Herdr specialists review architecture, tests, and safety read-only. Do not
push, open or merge PRs, close issues/milestones, tag, or release without
separate approval.

### Approved Milestone 0 contract

- `cli` owns commands, flags, orchestration, and output.
- `resolver` owns TOML discovery/loading, source traversal, exclusions,
  destination mapping, precedence, and structural collision detection.
- `filesystem` performs read-only destination and ancestor inspection.
- Dependencies are `main → cli → {resolver, filesystem}`; neither lower
  package depends on the other. Resolve desired links before inspecting
  destinations; occupancy never changes the selected source.
- Missing source roots warn when another usable root exists; otherwise
  error. Empty roots are usable. Unreadable and non-directory roots error.
- Exclusions match literal original source-root-relative paths before
  normalization and precedence. Excluded directories prune subtrees.
  Glob matching is deferred.
- Nested source symlinks are opaque entries and are never traversed.
  Dangling source links error. Configured roots may be symlinks to
  accessible directories.
- Detect file/directory structural collisions within and across roots.
  Leaf precedence cannot hide ancestor/descendant conflicts. Report all
  participants, block affected entries, and display unrelated entries.
- Broken destination symlinks are occupied conflicts with a distinct
  reason. Do not relink or remove anything.
- Plans and diagnostics are deterministic regardless of enumeration order.
- Preserve symlink-plus-`..` traversal in configured paths; do not collapse
  it lexically. Report unresolved traversal beneath missing ancestors.
- Inspect target-directory case sensitivity and Unicode normalization
  without writing probe files. Preserve distinctions supported by directory
  capabilities; block equivalent destinations, including conflicts with
  empty source directories. If capability queries fail, block potentially
  ambiguous entries while retaining unrelated entries.
- Warning-only plans succeed; errors and conflicts return nonzero.
- Dry-run performs no filesystem modifications.

### Approved Milestone 1 contract

- `dots up` creates real symlinks to selected absolute source paths. It
  never copies files or changes source precedence in response to occupancy.
- Preserve M0 dry-run output and its no-write guarantee. Installation adds
  read-only preparation with pinned directory descriptors; it must not
  require those write-preparation resources just to display a dry-run.
- Any resolver error, preflight conflict, or preparation failure prevents
  all mutations. Warning-only plans may proceed.
- Walk directory components individually. Accept existing directory
  symlinks, including those resolving outside `target_dir`, and preserve
  symlink-plus-`..` semantics. Pin resolved directory identities. Missing
  ancestors followed by unresolved `..` remain conflicts.
- Create parents and final symlinks relative to held directory descriptors.
  Never follow newly substituted parent symlinks. Never unlink, overwrite,
  copy, or rename over an existing entry. Unexpected existence, including
  a concurrently created correct link or parent, stops installation.
- Create needed parents with mode `0700`, subject to the existing umask.
  Do not change process umask or permissions of existing directories.
  Empty source directories remain collision evidence, not link actions.
- Revalidate named ancestors and selected source entries before mutations.
  Reject observed replacement or retargeting; in-place content edits are
  allowed. Real symlinks remain path-based and cannot freeze referents.
- Directory descriptors prevent symlink redirection, not concurrent rename.
  A rename after validation can move the pinned directory; operations remain
  attached to that directory identity. A newly made directory can also be
  replaced before its first identity observation. Reject observed changes
  and non-directory/symlink substitutions without claiming transactionality.
- Stop at the first runtime failure. Retain completed links and directories
  without automatic rollback or cleanup. Count successful mutation calls,
  including a successful mkdir followed by failed open, and report remaining
  entries as unattempted. Output failure does not undo completed writes.
- Prepared plans own copied inputs and deduplicated directory descriptors.
  Apply is single-use; Close is idempotent. Release descriptors after success,
  abandonment, or any failure. Fail cleanly on resource exhaustion without
  raising limits or reverting to pathname-based writes.
- Keep Go 1.23 and the three-package dependency direction. The pinned
  `golang.org/x/sys v0.35.0` dependency supplies maintained macOS/Linux
  descriptor-relative primitives; do not add a filesystem framework.
- Run all standard checks plus race testing where supported. Test actual
  symlinks, unchanged link identities, partial failures, descriptor cleanup,
  and controlled races with external sentinel trees in temporary fixtures.
- Local validation is review readiness. Formal closure requires an approved
  commit and push plus successful GitHub Actions CI. No later milestones,
  real home-directory installation, releases, or publication are authorized.

### Milestone 2 inspection contract

Authoritative scope: [M2: Inspection & Status](https://github.com/fernandoaleman/dots/milestone/3),
[resolver-based listing](https://github.com/fernandoaleman/dots/issues/2), and
[destination status](https://github.com/fernandoaleman/dots/issues/3).

- `ls`, `status`, and `up` use the same configuration loader and resolver.
  `ls` stops after resolution and reports selected mappings and source
  diagnostics without inspecting destination occupancy. `status` and `up`
  share CLI orchestration of destination inspection and physical collision
  checks, retaining empty-directory evidence and selected source identities.
- Inspection adds structured classifications to existing filesystem results;
  legacy action labels remain available to `up` and its dry-run. Compare
  named entries with error-aware metadata queries. Capability failures must
  not become confident wrong-link or confirmed-collision classifications.
- Preserve explanations when several problems affect one selected mapping.
  Inspection/source errors take precedence over confirmed conflicts, and each
  row contributes to one primary classification. Standalone diagnostics remain
  separately counted. Resolver source failures are distinguished structurally
  from collision evidence without parsing diagnostic text.
- `ls` succeeds for valid or warning-only resolution, including empty plans;
  source/configuration errors and structural conflicts fail. `status` succeeds
  only for an empty valid plan or all-correct links; missing and divergent
  states fail. Both use exit 1 for failures and exit 2 for invalid usage.
- Both commands accept `--config` before or after the command, reject
  positional arguments and installation-only flags, and are strictly read-only.
  They never require installation preparation or create configuration files.
- Report only current selected mappings. Correctness does not establish
  ownership; no historical-link scan, repair, replacement, JSON output,
  removal, import, or new dependency is included.
